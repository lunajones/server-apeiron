package gameapi

import (
	"context"
	"fmt"
	"math"
	"net"
	"sync"
	"time"

	gamev1 "server-apeiron/gen/apeiron/game/v1"
	"server-apeiron/internal/config"
	"server-apeiron/internal/logging"
	"server-apeiron/internal/movement"

	"google.golang.org/grpc"
)

const (
	defaultRegionID = "old_china_test_region"
	defaultWorldID  = "apeiron"
	defaultZoneID   = "plain_test_map"
	defaultBiomeID  = "frontier_grassland"
	tickRate        = 30
)

type Runtime struct {
	gamev1.UnimplementedSessionServiceServer
	gamev1.UnimplementedSnapshotServiceServer
	gamev1.UnimplementedCommandServiceServer
	gamev1.UnimplementedObservabilityServiceServer

	mu        sync.Mutex
	started   time.Time
	tick      uint64
	sessions  map[string]*sessionState
	players   map[string]*entityState
	entities  map[uint64]*entityState
	acks      map[string][]*gamev1.CommandAck
	nextID    uint64
	contracts RuntimeContracts
}

type sessionState struct {
	id        string
	accountID string
	playerID  string
	regionID  string
}

type entityState struct {
	id             uint64
	entityType     string
	regionID       string
	templateID     string
	archetype      string
	visualID       string
	position       vector
	velocity       vector
	yaw            float64
	health         float64
	maxHealth      float64
	stamina        float64
	maxStamina     float64
	posture        float64
	maxPosture     float64
	movementState  string
	combatState    string
	skillState     string
	aggroState     string
	aggression     float64
	lastSequence   uint64
	lastClientTick uint64
	locomotion     *gamev1.LocomotionState
	skillRuntime   *gamev1.SkillRuntimeState
	combatMode     *gamev1.CombatModeState
	creatureAI     *gamev1.CreatureAIState
}

type vector struct {
	x float64
	y float64
	z float64
}

func Serve(ctx context.Context, cfg config.NetworkConfig) error {
	return ServeRuntime(ctx, cfg, NewRuntime())
}

func ServeRuntime(ctx context.Context, cfg config.NetworkConfig, runtime *Runtime) error {
	addr := fmt.Sprintf("%s:%d", cfg.GRPCHost, cfg.GRPCPort)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen game grpc %s: %w", addr, err)
	}
	defer lis.Close()

	server := grpc.NewServer()
	gamev1.RegisterSessionServiceServer(server, runtime)
	gamev1.RegisterSnapshotServiceServer(server, runtime)
	gamev1.RegisterCommandServiceServer(server, runtime)
	gamev1.RegisterObservabilityServiceServer(server, runtime)

	log := logging.WithComponent("gameapi")
	go func() {
		<-ctx.Done()
		server.GracefulStop()
	}()

	log.Info().Str("addr", addr).Msg("game grpc server listening")
	if err := server.Serve(lis); err != nil {
		return fmt.Errorf("serve game grpc: %w", err)
	}
	return nil
}

func NewRuntime() *Runtime {
	return NewRuntimeWithContracts(RecoveredRuntimeContracts())
}

func NewRuntimeWithContracts(contracts RuntimeContracts) *Runtime {
	if contracts.MovementProfile == nil {
		contracts.MovementProfile = recoveredMovementProfile()
	}
	if contracts.ActionContracts == nil {
		recovered := RecoveredRuntimeContracts()
		contracts.ActionContracts = recovered.ActionContracts
	}
	if contracts.SkillContracts == nil {
		recovered := RecoveredRuntimeContracts()
		contracts.SkillContracts = recovered.SkillContracts
	}
	if contracts.WolfPolicy.ContractID == "" {
		contracts.WolfPolicy = RecoveredRuntimeContracts().WolfPolicy
	}
	if len(contracts.CombatModes) == 0 {
		contracts.CombatModes = recoveredCombatModeSlots()
	}
	return &Runtime{
		started:   time.Now(),
		sessions:  make(map[string]*sessionState),
		players:   make(map[string]*entityState),
		entities:  make(map[uint64]*entityState),
		acks:      make(map[string][]*gamev1.CommandAck),
		nextID:    1000000,
		contracts: contracts,
	}
}

func (r *Runtime) OpenSession(ctx context.Context, req *gamev1.OpenSessionRequest) (*gamev1.OpenSessionResponse, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	accountID := req.GetContext().GetAccountId()
	if accountID == "" {
		accountID = "local_account"
	}
	sessionID := req.GetContext().GetSessionId()
	if sessionID == "" {
		sessionID = fmt.Sprintf("session:%s:%d", accountID, time.Now().UnixMilli())
	}
	r.sessions[sessionID] = &sessionState{id: sessionID, accountID: accountID, regionID: defaultRegionID}

	return &gamev1.OpenSessionResponse{
		Result:                         &gamev1.Result{Success: true, Code: "ok", Message: "session opened"},
		SessionId:                      sessionID,
		AccountId:                      accountID,
		Tick:                           r.serverTickLocked(),
		MovementActionContracts:        r.contracts.movementContractManifest(),
		MovementActionContractPayloads: r.contracts.movementContractPayloads(),
	}, nil
}

func (r *Runtime) AttachPlayer(ctx context.Context, req *gamev1.AttachPlayerRequest) (*gamev1.AttachPlayerResponse, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	playerID := req.GetPlayerId()
	if playerID == "" {
		playerID = "local_player"
	}
	sessionID := req.GetContext().GetSessionId()
	if sessionID != "" {
		if session := r.sessions[sessionID]; session != nil {
			session.playerID = playerID
			if req.GetPreferredRegionId() != "" {
				session.regionID = req.GetPreferredRegionId()
			}
		}
	}

	player := r.ensurePlayerLocked(playerID)
	r.ensureWolfLocked(player)

	return &gamev1.AttachPlayerResponse{
		Result:                         &gamev1.Result{Success: true, Code: "ok", Message: "player attached"},
		Player:                         &gamev1.PlayerRef{PlayerId: playerID},
		Region:                         regionRef(),
		Tick:                           r.serverTickLocked(),
		SpawnTransform:                 transform(player.position, player.yaw),
		MovementActionContracts:        r.contracts.movementContractManifest(),
		MovementActionContractPayloads: r.contracts.movementContractPayloads(),
	}, nil
}

func (r *Runtime) GetSnapshot(ctx context.Context, req *gamev1.SnapshotRequest) (*gamev1.SnapshotResponse, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.tick++
	r.updateCreaturePoliciesLocked()
	out := &gamev1.SnapshotResponse{
		Tick:        r.serverTickLocked(),
		Region:      regionRef(),
		Entities:    make([]*gamev1.SnapshotEntity, 0, len(r.entities)),
		CommandAcks: r.drainAcksLocked(req.GetContext().GetSessionId()),
	}
	for _, entity := range r.entities {
		out.Entities = append(out.Entities, entity.snapshot(r.contracts))
	}
	return out, nil
}

func (r *Runtime) SubmitCommand(ctx context.Context, cmd *gamev1.PlayerCommand) (*gamev1.CommandAck, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.tick++
	player := r.playerForCommandLocked(cmd)
	if player == nil {
		ack := r.ackLocked(cmd, nil, false, "player_not_attached", "player is not attached")
		r.queueAckLocked(cmd.GetContext().GetSessionId(), ack)
		return ack, nil
	}

	player.lastSequence = cmd.GetSequence()
	player.lastClientTick = cmd.GetClientTick()

	switch cmd.GetType() {
	case gamev1.CommandType_COMMAND_TYPE_MOVE:
		r.applyMove(player, cmd)
	case gamev1.CommandType_COMMAND_TYPE_TURN:
		r.applyTurn(player, cmd)
	case gamev1.CommandType_COMMAND_TYPE_DODGE:
		r.applyImpulse(player, cmd, r.contracts.contractForAbility("dodge"), 260)
	case gamev1.CommandType_COMMAND_TYPE_LEAP:
		r.applyImpulse(player, cmd, r.contracts.contractForAbility("jump"), 280)
	case gamev1.CommandType_COMMAND_TYPE_CAST_SKILL:
		r.applySkill(player, cmd)
	case gamev1.CommandType_COMMAND_TYPE_BLOCK_START, gamev1.CommandType_COMMAND_TYPE_BLOCK_STOP, gamev1.CommandType_COMMAND_TYPE_PARRY:
		r.applyDefense(player, cmd)
	case gamev1.CommandType_COMMAND_TYPE_SWITCH_COMBAT_MODE:
		r.applyCombatMode(player, cmd)
	default:
		player.locomotion = r.locomotion("grounded", "idle", "", "idle", player.position, player.position, 0)
	}

	ack := r.ackLocked(cmd, player, true, "", "accepted")
	r.queueAckLocked(cmd.GetContext().GetSessionId(), ack)
	return ack, nil
}

func (r *Runtime) Health(ctx context.Context, _ *gamev1.Empty) (*gamev1.HealthResponse, error) {
	return &gamev1.HealthResponse{Healthy: true, Status: "healthy"}, nil
}

func (r *Runtime) Readiness(ctx context.Context, _ *gamev1.Empty) (*gamev1.ReadinessResponse, error) {
	return &gamev1.ReadinessResponse{Ready: true}, nil
}

func (r *Runtime) RuntimeStats(ctx context.Context, _ *gamev1.Empty) (*gamev1.RuntimeStatsResponse, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return &gamev1.RuntimeStatsResponse{
		Tick:                 r.serverTickLocked(),
		ActiveRegions:        1,
		ActiveSessions:       uint32(len(r.sessions)),
		ActiveEntities:       uint32(len(r.entities)),
		AverageFrameMs:       0.2,
		P95FrameMs:           0.5,
		PhaseStatus:          map[string]string{"runtime": "recovered_in_memory"},
		SpawnedCreatureCount: 1,
	}, nil
}

func (r *Runtime) ensurePlayerLocked(playerID string) *entityState {
	if player := r.players[playerID]; player != nil {
		return player
	}
	entity := &entityState{
		id:            r.nextRuntimeIDLocked(),
		entityType:    "player",
		regionID:      defaultRegionID,
		templateID:    "player_sword_shield",
		archetype:     "sword_shield",
		visualID:      "player",
		position:      vector{x: -2500, y: 1900, z: 98},
		yaw:           0,
		health:        100,
		maxHealth:     100,
		stamina:       100,
		maxStamina:    100,
		posture:       100,
		maxPosture:    100,
		movementState: "grounded",
		combatState:   "ready",
		skillState:    "idle",
	}
	entity.locomotion = r.locomotion("grounded", "idle", "", "idle", entity.position, entity.position, 0)
	entity.combatMode = swordShieldCombatMode("mode_sword_shield_bulwark", r.contracts.CombatModes)
	r.players[playerID] = entity
	r.entities[entity.id] = entity
	return entity
}

func (r *Runtime) ensureWolfLocked(player *entityState) *entityState {
	for _, entity := range r.entities {
		if entity.entityType == "creature" && entity.templateID == "steppe_wolf" {
			return entity
		}
	}
	wolf := &entityState{
		id:            r.nextRuntimeIDLocked(),
		entityType:    "creature",
		regionID:      defaultRegionID,
		templateID:    "steppe_wolf",
		archetype:     "wolf",
		visualID:      "steppe_wolf",
		position:      vector{x: player.position.x + 520, y: player.position.y + 120, z: player.position.z},
		yaw:           180,
		health:        160,
		maxHealth:     160,
		stamina:       100,
		maxStamina:    100,
		posture:       100,
		maxPosture:    100,
		movementState: "orbit",
		combatState:   "engaged",
		skillState:    "idle",
		aggroState:    "engaged",
		aggression:    0.75,
	}
	wolf.locomotion = r.locomotion("grounded", "orbit", "run", "active", wolf.position, wolf.position, 0)
	wolf.creatureAI = &gamev1.CreatureAIState{
		MovementTactic:          "flank",
		CombatTactic:            "harass",
		Commitment:              "probing",
		CapabilityId:            r.contracts.WolfPolicy.CapabilityID,
		ContractId:              r.contracts.WolfPolicy.ContractID,
		ContractHash:            r.contracts.WolfPolicy.ContractHash,
		OrbitSide:               "left",
		LastReason:              "recovered_runtime_seed",
		BehaviorFamily:          "beast_harasser",
		CombatRole:              "duelist",
		DesiredRangeCm:          r.contracts.WolfPolicy.DesiredRangeCM,
		ActualRangeCm:           distance(wolf.position, player.position),
		SelectedSkillId:         "bite",
		ProfileSource:           "db_contract_recovery_pending",
		SkillMovementType:       "leap",
		SkillMovementDistanceCm: r.contracts.WolfPolicy.LungeDistanceCM,
	}
	r.entities[wolf.id] = wolf
	return wolf
}

func (r *Runtime) updateCreaturePoliciesLocked() {
	var player *entityState
	for _, candidate := range r.players {
		player = candidate
		break
	}
	if player == nil {
		return
	}
	for _, creature := range r.entities {
		if creature.entityType != "creature" || creature.templateID != "steppe_wolf" {
			continue
		}
		r.updateWolfPolicyLocked(creature, player)
	}
}

func (r *Runtime) updateWolfPolicyLocked(wolf *entityState, player *entityState) {
	toPlayer := normalize(vector{x: player.position.x - wolf.position.x, y: player.position.y - wolf.position.y})
	if toPlayer == (vector{}) {
		toPlayer = vector{x: -1}
	}
	right := vector{x: -toPlayer.y, y: toPlayer.x}
	rangeCM := distance(wolf.position, player.position)
	start := wolf.position

	policy := r.contracts.WolfPolicy
	phaseTick := r.tick % 240
	action := "orbit"
	selectedSkill := "bite"
	speed := policy.OrbitSpeedCMS
	moveDir := right

	switch {
	case rangeCM > policy.ChaseRangeCM:
		action = "chase"
		selectedSkill = "lunge"
		speed = policy.ChaseSpeedCMS
		moveDir = toPlayer
	case phaseTick >= 72 && phaseTick < 92 && rangeCM > policy.LungeRangeCM:
		action = "lunge"
		selectedSkill = "lunge"
		speed = policy.LungeSpeedCMS
		moveDir = toPlayer
	case phaseTick >= 150 && phaseTick < 166 && rangeCM < 260:
		action = "maul"
		selectedSkill = "maul"
		speed = policy.MaulSpeedCMS
		moveDir = right
	case rangeCM < policy.RetreatRangeCM:
		action = "retreat"
		selectedSkill = policy.DodgeSkillID
		speed = policy.RetreatSpeedCMS
		moveDir = scale(toPlayer, -1)
	}

	selectedRuntime := r.contracts.skillContract(selectedSkill)
	step := scale(normalize(moveDir), speed/float64(tickRate))
	wolf.position = add(wolf.position, step)
	wolf.velocity = scale(normalize(moveDir), speed)
	wolf.yaw = vectorYaw(toPlayer)
	wolf.movementState = action
	wolf.skillState = selectedSkill
	wolf.locomotion = r.locomotion("grounded", action, selectedSkill, "active", start, wolf.position, 0)
	wolf.locomotion.ReconciliationMode = creatureReconciliation(action)
	wolf.locomotion.ContractHash = wolf.locomotion.ReconciliationMode
	wolf.locomotion.TargetSpeed = speed
	wolf.locomotion.EffectiveSpeed = speed
	wolf.locomotion.ActionDistanceTraveled = length(step)
	wolf.creatureAI = &gamev1.CreatureAIState{
		MovementTactic:                        "flank",
		CombatTactic:                          "harass",
		Commitment:                            "probing",
		CapabilityId:                          policy.CapabilityID,
		ContractId:                            policy.ContractID,
		ContractHash:                          policy.ContractHash,
		OrbitSide:                             "left",
		LastReason:                            "recovered_runtime_policy",
		TacticalDestination:                   toProto(add(wolf.position, scale(moveDir, 180))),
		BehaviorFamily:                        "beast_harasser",
		CombatRole:                            "duelist",
		DecisionScore:                         0.72,
		DesiredRangeCm:                        policy.DesiredRangeCM,
		ActualRangeCm:                         rangeCM,
		PathState:                             "direct",
		LosState:                              "clear",
		SelectedSkillId:                       selectedSkill,
		ProfileSource:                         "db_contract_recovery_pending",
		SkillMovementArcHeightCm:              policy.LungeArcHeightCM,
		SkillMovementArcCurve:                 "low_fast",
		SkillMovementTakeoffMs:                140,
		SkillMovementLandingLockMs:            120,
		SkillWindupMs:                         selectedRuntime.WindupMS,
		SkillActiveStartMs:                    selectedRuntime.WindupMS,
		SkillActiveEndMs:                      selectedRuntime.WindupMS + selectedRuntime.ActiveMS,
		SkillRecoveryMs:                       selectedRuntime.RecoveryMS,
		SkillActionLockMs:                     selectedRuntime.WindupMS + selectedRuntime.ActiveMS + selectedRuntime.RecoveryMS,
		SkillMovementType:                     selectedRuntime.MovementAction.ActionType,
		SkillMovementStartMs:                  selectedRuntime.WindupMS,
		SkillMovementDurationMs:               selectedRuntime.MovementAction.DurationMS,
		SkillMovementDistanceCm:               selectedRuntime.MovementAction.DistanceCM,
		SkillMovementDesiredLandingDistanceCm: 760,
		SkillMovementMinLandingDistanceCm:     180,
		SkillMovementStopAtContactRatio:       1,
	}
	now := time.Now().UnixMilli()
	wolf.skillRuntime = &gamev1.SkillRuntimeState{CurrentSkillId: selectedSkill, State: action, StartedAtMs: now, LastResolvedAtMs: now}
}

func (r *Runtime) playerForCommandLocked(cmd *gamev1.PlayerCommand) *entityState {
	if session := r.sessions[cmd.GetContext().GetSessionId()]; session != nil && session.playerID != "" {
		return r.players[session.playerID]
	}
	for _, player := range r.players {
		return player
	}
	return nil
}

func (r *Runtime) applyMove(player *entityState, cmd *gamev1.PlayerCommand) {
	move := cmd.GetMove()
	dir := normalize(fromProto(move.GetDirection()))
	if dir == (vector{}) {
		player.velocity = vector{}
		player.movementState = "idle"
		player.locomotion = r.locomotion("grounded", "move_stop", "move", "recovery", player.position, player.position, cmd.GetSequence())
		return
	}
	speed := 470.0
	if move.GetSprint() {
		speed = 690.0
		if math.Abs(dir.x) > 0.20 && dir.y < 0.35 {
			speed *= 0.75
		}
	}
	step := scale(dir, speed/float64(tickRate))
	start := player.position
	player.position = add(player.position, step)
	player.velocity = scale(dir, speed)
	player.movementState = "moving"
	if move.TargetYaw != nil {
		player.yaw = move.GetTargetYaw()
	}
	player.locomotion = r.locomotion("grounded", "move", "move", "active", start, player.position, cmd.GetSequence())
	player.locomotion.TargetSpeed = speed
	player.locomotion.EffectiveSpeed = speed
	player.locomotion.ActionDistanceTraveled = length(step)
}

func (r *Runtime) applyTurn(player *entityState, cmd *gamev1.PlayerCommand) {
	turn := cmd.GetTurn()
	player.yaw = turn.GetTargetYaw()
	player.locomotion = r.locomotion("grounded", "turn", "turn", "active", player.position, player.position, cmd.GetSequence())
	player.locomotion.AuthoritativeYaw = player.yaw
}

func (r *Runtime) applyImpulse(player *entityState, cmd *gamev1.PlayerCommand, contract MovementActionRuntimeContract, fallbackDistanceCM float64) {
	dir := vector{x: 1}
	if cmd.GetDodge() != nil {
		dir = normalize(fromProto(cmd.GetDodge().GetDirection()))
	}
	if cmd.GetLeap() != nil {
		dir = normalize(fromProto(cmd.GetLeap().GetDirection()))
	}
	if dir == (vector{}) {
		dir = yawVector(player.yaw)
	}
	start := player.position
	distanceCM := distanceFromContract(contract, fallbackDistanceCM)
	player.position = add(player.position, scale(dir, distanceCM))
	player.velocity = scale(dir, distanceCM*float64(tickRate)/10)
	player.movementState = contract.ActionType
	player.skillState = contract.AbilityKey
	player.locomotion = locomotionFromContract(contract, "active", start, player.position, r.tick, cmd.GetSequence())
	player.locomotion.ActionDistanceTraveled = distanceCM
	player.locomotion.TargetSpeed = length(player.velocity)
	player.locomotion.EffectiveSpeed = player.locomotion.TargetSpeed
}

func (r *Runtime) applySkill(player *entityState, cmd *gamev1.PlayerCommand) {
	cast := cmd.GetCastSkill()
	skillID := cast.GetSkillId()
	if skillID == "" || skillID == "player_basic_attack" {
		skillID = nextBasicAttack(player)
	}
	dir := normalize(fromProto(cast.GetAimDirection()))
	if dir == (vector{}) {
		dir = yawVector(player.yaw)
	}
	skillContract := r.contracts.skillContract(skillID)
	distanceCM := distanceFromContract(skillContract.MovementAction, 0)
	start := player.position
	player.position = add(player.position, scale(dir, distanceCM))
	player.velocity = scale(dir, distanceCM*float64(tickRate)/12)
	player.skillState = "active"
	player.combatState = "committed"
	player.locomotion = locomotionFromContract(skillContract.MovementAction, "active", start, player.position, r.tick, cmd.GetSequence())
	player.locomotion.ActionDistanceTraveled = distanceCM
	player.locomotion.TargetSpeed = length(player.velocity)
	player.locomotion.EffectiveSpeed = player.locomotion.TargetSpeed
	now := time.Now().UnixMilli()
	player.skillRuntime = &gamev1.SkillRuntimeState{CurrentSkillId: skillID, State: "active", StartedAtMs: now, LastResolvedAtMs: now}
}

func (r *Runtime) applyDefense(player *entityState, cmd *gamev1.PlayerCommand) {
	defense := cmd.GetDefense()
	if cmd.GetType() == gamev1.CommandType_COMMAND_TYPE_BLOCK_STOP {
		player.combatState = "ready"
	} else if defense.GetParry() {
		player.combatState = "parry"
	} else {
		player.combatState = "blocking"
	}
	player.locomotion = r.locomotion("grounded", "defense", "block", "active", player.position, player.position, cmd.GetSequence())
}

func (r *Runtime) applyCombatMode(player *entityState, cmd *gamev1.PlayerCommand) {
	mode := cmd.GetSwitchCombatMode().GetTargetCombatModeId()
	if mode == "" {
		mode = "mode_sword_shield_bulwark"
	}
	player.combatMode = swordShieldCombatMode(mode, r.contracts.CombatModes)
}

func (r *Runtime) ackLocked(cmd *gamev1.PlayerCommand, player *entityState, accepted bool, code string, message string) *gamev1.CommandAck {
	metadata := map[string]string{
		"command_type":      commandTypeName(cmd.GetType()),
		"movement_protocol": "recovered_game_v1",
	}
	if cmd.GetCastSkill() != nil {
		skillID := cmd.GetCastSkill().GetSkillId()
		if player != nil && player.skillRuntime != nil && (skillID == "" || skillID == "player_basic_attack") {
			skillID = player.skillRuntime.GetCurrentSkillId()
		}
		metadata["skill_id"] = skillID
		metadata["movement_action_type"] = "grounded_skill"
		metadata["ability_key"] = skillID
		metadata["contract_hash"] = "grounded_skill_action_reconciliation"
		metadata["movement_action_contract_hash"] = "grounded_skill_action_reconciliation"
		metadata["movement_action_contract_sync_state"] = "confirmed"
	}
	return &gamev1.CommandAck{
		Accepted:      accepted,
		CommandId:     cmd.GetCommandId(),
		Sequence:      cmd.GetSequence(),
		RejectionCode: code,
		Message:       message,
		ServerTick:    r.tick,
		Metadata:      metadata,
	}
}

func (r *Runtime) queueAckLocked(sessionID string, ack *gamev1.CommandAck) {
	if sessionID == "" || ack == nil {
		return
	}
	r.acks[sessionID] = append(r.acks[sessionID], ack)
	if len(r.acks[sessionID]) > 32 {
		r.acks[sessionID] = r.acks[sessionID][len(r.acks[sessionID])-32:]
	}
}

func (r *Runtime) drainAcksLocked(sessionID string) []*gamev1.CommandAck {
	if sessionID == "" {
		return nil
	}
	acks := r.acks[sessionID]
	delete(r.acks, sessionID)
	return acks
}

func (r *Runtime) nextRuntimeIDLocked() uint64 {
	id := r.nextID
	r.nextID++
	return id
}

func (r *Runtime) serverTickLocked() *gamev1.ServerTick {
	elapsed := uint64(time.Since(r.started).Seconds() * tickRate)
	if elapsed > r.tick {
		r.tick = elapsed
	}
	return &gamev1.ServerTick{Tick: r.tick, ServerTimeMs: time.Now().UnixMilli(), TickRate: tickRate}
}

func (e *entityState) snapshot(contracts RuntimeContracts) *gamev1.SnapshotEntity {
	return &gamev1.SnapshotEntity{
		Ref:                          &gamev1.EntityRef{RuntimeEntityId: e.id, EntityType: e.entityType, RegionId: e.regionID},
		TemplateId:                   e.templateID,
		Archetype:                    e.archetype,
		VisualId:                     e.visualID,
		Transform:                    transform(e.position, e.yaw),
		Velocity:                     toProto(e.velocity),
		Health:                       e.health,
		MaxHealth:                    e.maxHealth,
		Stamina:                      e.stamina,
		MaxStamina:                   e.maxStamina,
		Posture:                      e.posture,
		MaxPosture:                   e.maxPosture,
		MovementState:                e.movementState,
		CombatState:                  e.combatState,
		SkillState:                   e.skillState,
		SkillRuntimeState:            e.skillRuntime,
		AggroState:                   e.aggroState,
		Aggression:                   e.aggression,
		LastProcessedCommandSequence: e.lastSequence,
		LastProcessedClientTick:      e.lastClientTick,
		Locomotion:                   e.locomotion,
		MovementReconciliation:       contracts.MovementProfile,
		CreatureAiState:              e.creatureAI,
		CombatModeState:              e.combatMode,
	}
}

func (r *Runtime) locomotion(mode, action, ability, phase string, start, projected vector, sequence uint64) *gamev1.LocomotionState {
	contract := r.contracts.contractForAbility(ability)
	if ability == "" {
		contract = r.contracts.contractForAbility(action)
	}
	state := locomotionFromContract(contract, phase, start, projected, r.tick, sequence)
	state.MovementMode = mode
	state.Action = action
	state.AbilityKey = ability
	return state
}

func locomotionFromContract(contract MovementActionRuntimeContract, phase string, start, projected vector, tick uint64, sequence uint64) *gamev1.LocomotionState {
	reconciliation := movement.ReconciliationMode(contract)
	if reconciliation == "" {
		reconciliation = "grounded_move_reconciliation"
	}
	duration := contract.DurationMS
	if duration == 0 {
		duration = 180
	}
	active := contract.ActiveMS
	if active == 0 {
		active = 120
	}
	recovery := contract.RecoveryMS
	if recovery == 0 {
		recovery = 60
	}
	phasePolicy := contract.PhaseWindowPolicy
	if phasePolicy == "" {
		phasePolicy = "server_authoritative"
	}
	predictionPolicy := contract.PredictionErrorPolicy
	if predictionPolicy == "" {
		predictionPolicy = "bounded_smooth_correction"
	}
	return &gamev1.LocomotionState{
		MovementMode:            "grounded",
		Action:                  contract.ActionType,
		AbilityKey:              contract.AbilityKey,
		Phase:                   phase,
		ReconciliationMode:      reconciliation,
		DurationMs:              duration,
		ActiveMs:                active,
		RecoveryMs:              recovery,
		ContractVersion:         "movement_action_v1",
		ContractHash:            contractHash(contract),
		PhaseWindowPolicy:       phasePolicy,
		PredictionErrorPolicy:   predictionPolicy,
		ActionContractId:        contract.ID,
		ActionFamily:            actionFamily(contract),
		MovementType:            contract.ActionType,
		ContractSyncState:       "confirmed",
		ClientActionSequence:    sequence,
		ServerReceivedTick:      tick,
		ServerActionStartedTick: tick,
		ActionStartedTick:       tick,
		ActionStartPosition:     toProto(start),
		ActionProjectedPosition: toProto(projected),
		LastUpdatedTick:         tick,
	}
}

func swordShieldCombatMode(active string, slots []*gamev1.CombatModeSlot) *gamev1.CombatModeState {
	return &gamev1.CombatModeState{
		WeaponCombinationId: "sword_shield",
		ActiveCombatMode:    active,
		TargetCombatMode:    active,
		Phase:               "ready",
		SwitchDurationMs:    500,
		CombatModeEnforced:  true,
		ModeSlots:           slots,
	}
}

func nextBasicAttack(player *entityState) string {
	switch player.skillRuntime.GetCurrentSkillId() {
	case "player_basic_attack_1":
		return "player_basic_attack_2"
	case "player_basic_attack_2":
		return "player_basic_attack_3"
	default:
		return "player_basic_attack_1"
	}
}

func commandTypeName(t gamev1.CommandType) string {
	switch t {
	case gamev1.CommandType_COMMAND_TYPE_MOVE:
		return "move"
	case gamev1.CommandType_COMMAND_TYPE_DODGE:
		return "dodge"
	case gamev1.CommandType_COMMAND_TYPE_LEAP:
		return "leap"
	case gamev1.CommandType_COMMAND_TYPE_TURN:
		return "turn"
	case gamev1.CommandType_COMMAND_TYPE_CAST_SKILL:
		return "cast_skill"
	case gamev1.CommandType_COMMAND_TYPE_BLOCK_START:
		return "block_start"
	case gamev1.CommandType_COMMAND_TYPE_BLOCK_STOP:
		return "block_stop"
	case gamev1.CommandType_COMMAND_TYPE_PARRY:
		return "parry"
	case gamev1.CommandType_COMMAND_TYPE_SWITCH_COMBAT_MODE:
		return "switch_combat_mode"
	case gamev1.CommandType_COMMAND_TYPE_USE_CONSUMABLE:
		return "use_consumable"
	case gamev1.CommandType_COMMAND_TYPE_INTERACT:
		return "interact"
	default:
		return "unknown"
	}
}

func regionRef() *gamev1.RegionRef {
	return &gamev1.RegionRef{RegionId: defaultRegionID, WorldId: defaultWorldID, ZoneId: defaultZoneID, BiomeId: defaultBiomeID}
}

func transform(pos vector, yaw float64) *gamev1.Transform {
	return &gamev1.Transform{Position: toProto(pos), Rotation: &gamev1.Rotation{Yaw: yaw}}
}

func fromProto(v *gamev1.Vector3) vector {
	if v == nil {
		return vector{}
	}
	return vector{x: v.GetX(), y: v.GetY(), z: v.GetZ()}
}

func toProto(v vector) *gamev1.Vector3 {
	return &gamev1.Vector3{X: v.x, Y: v.y, Z: v.z}
}

func normalize(v vector) vector {
	l := length(v)
	if l <= 0.0001 {
		return vector{}
	}
	return vector{x: v.x / l, y: v.y / l, z: v.z / l}
}

func length(v vector) float64 {
	return math.Sqrt(v.x*v.x + v.y*v.y + v.z*v.z)
}

func distance(a, b vector) float64 {
	return length(vector{x: a.x - b.x, y: a.y - b.y, z: a.z - b.z})
}

func add(a, b vector) vector {
	return vector{x: a.x + b.x, y: a.y + b.y, z: a.z + b.z}
}

func scale(v vector, amount float64) vector {
	return vector{x: v.x * amount, y: v.y * amount, z: v.z * amount}
}

func yawVector(yaw float64) vector {
	radians := yaw * math.Pi / 180
	return vector{x: math.Cos(radians), y: math.Sin(radians)}
}

func vectorYaw(v vector) float64 {
	return math.Atan2(v.y, v.x) * 180 / math.Pi
}

func creatureReconciliation(action string) string {
	switch action {
	case "lunge":
		return "leap_reconciliation"
	case "maul":
		return "grounded_skill_action_reconciliation"
	case "retreat":
		return "dodge_reconciliation"
	default:
		return "grounded_move_reconciliation"
	}
}
