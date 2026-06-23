package gameapi

import (
	"context"
	"fmt"
	"math"
	"net"
	"strings"
	"sync"
	"time"

	gamev1 "server-apeiron/gen/apeiron/game/v1"
	creatureai "server-apeiron/internal/ai"
	combatpipeline "server-apeiron/internal/combat"
	"server-apeiron/internal/combat/actionruntime"
	"server-apeiron/internal/combat/damagegroup"
	"server-apeiron/internal/config"
	"server-apeiron/internal/domain/ids"
	domainmath "server-apeiron/internal/domain/math"
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
	options   RuntimeOptions
	aiSystem  *creatureai.RegionBrainSystem
	impact    *combatpipeline.ImpactResolutionPipeline
	impacts   *damagegroup.Runtime[skillImpactSchedule]
}

type RuntimeOptions struct {
	MovementValidation bool
	DisableCreatures   bool
}

type sessionState struct {
	id        string
	accountID string
	playerID  string
	regionID  string
}

type entityState struct {
	id                    uint64
	entityType            string
	regionID              string
	templateID            string
	archetype             string
	visualID              string
	position              vector
	velocity              vector
	yaw                   float64
	health                float64
	maxHealth             float64
	stamina               float64
	maxStamina            float64
	posture               float64
	maxPosture            float64
	movementState         string
	combatState           string
	skillState            string
	aggroState            string
	aggression            float64
	lastSequence          uint64
	lastClientTick        uint64
	processedCommandIDs   map[string]struct{}
	processedCommandOrder []string
	locomotion            *gamev1.LocomotionState
	skillRuntime          *gamev1.SkillRuntimeState
	actionInstance        *actionruntime.Instance
	actionMotion          *actionMotionState
	combatMode            *gamev1.CombatModeState
	creatureAI            *gamev1.CreatureAIState
	creatureCooldownUntil map[string]time.Time

	// actionLockedUntil marks an owned movement action (leap/dodge) the player cannot
	// interrupt with a skill/basic. Restores the chat 6 #3 rule: no skill while
	// jumping/dodging. Time-based so it auto-expires with the action.
	actionLockedUntil time.Time
	actionLockReason  string
}

type actionMotionState struct {
	SkillID           string
	CommandID         string
	Sequence          uint64
	ClientTick        uint64
	StartedAt         time.Time
	StartPosition     vector
	ProjectedPosition vector
	Direction         vector
	Contract          MovementActionRuntimeContract
	NormalInputPolicy string
	TotalDistanceCM   float64
	ContactPolicy     string
	ContactTargetID   uint64
	AllowsPassthrough bool
	StopsAtContact    bool
	ContactStopCM     float64
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
	return NewRuntimeWithContracts(RuntimeContracts{
		Source:          runtimeContractSourceUnconfigured,
		ActionContracts: map[string]MovementActionRuntimeContract{},
		SkillContracts:  map[string]SkillRuntimeContract{},
	})
}

func NewRuntimeWithContracts(contracts RuntimeContracts) *Runtime {
	return NewRuntimeWithOptions(contracts, RuntimeOptions{})
}

func NewRuntimeWithOptions(contracts RuntimeContracts, options RuntimeOptions) *Runtime {
	if contracts.ActionContracts == nil {
		contracts.ActionContracts = map[string]MovementActionRuntimeContract{}
	}
	if contracts.SkillContracts == nil {
		contracts.SkillContracts = map[string]SkillRuntimeContract{}
	}
	return &Runtime{
		started:   time.Now(),
		sessions:  make(map[string]*sessionState),
		players:   make(map[string]*entityState),
		entities:  make(map[uint64]*entityState),
		acks:      make(map[string][]*gamev1.CommandAck),
		nextID:    1000000,
		contracts: contracts,
		options:   options,
		aiSystem:  creatureai.NewRegionBrainSystem(),
		impact:    combatpipeline.NewImpactResolutionPipeline(nil, nil, nil, nil),
		impacts:   damagegroup.NewRuntime[skillImpactSchedule](),
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
	if r.creaturesEnabled() {
		r.ensureWolfLocked(player)
	}

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
	if r.creaturesEnabled() {
		r.updateCreaturePoliciesLocked()
	}
	now := time.Now()
	r.refreshActionRuntimeStatesLocked(now)
	r.runPendingSkillImpactSchedulesLocked(now)
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

	player := r.playerForCommandLocked(cmd)
	if player == nil {
		r.tick++
		ack := r.ackLocked(cmd, nil, false, "player_not_attached", "player is not attached")
		r.queueAckLocked(cmd.GetContext().GetSessionId(), ack)
		return ack, nil
	}
	if duplicate, stale, reason := playerCommandReplayState(player, cmd); duplicate || stale {
		code := ""
		accepted := true
		message := "duplicate command ignored"
		if stale {
			code = "stale_command"
			accepted = false
			message = reason
		}
		ack := r.ackLocked(cmd, player, accepted, code, message)
		if ack.Metadata == nil {
			ack.Metadata = map[string]string{}
		}
		ack.Metadata["command_replay_state"] = reason
		r.queueAckLocked(cmd.GetContext().GetSessionId(), ack)
		return ack, nil
	}

	r.tick++
	now := time.Now()
	r.refreshActionRuntimeStatesLocked(now)
	player.lastSequence = cmd.GetSequence()
	player.lastClientTick = cmd.GetClientTick()

	if cmd.GetType() == gamev1.CommandType_COMMAND_TYPE_CAST_SKILL {
		if reason, locked := r.skillActionLockedLocked(player); locked {
			ack := r.ackLocked(cmd, player, false, "action_locked", reason)
			r.queueAckLocked(cmd.GetContext().GetSessionId(), ack)
			return ack, nil
		}
		if r.isBasicAttackCommandLockedByCombatMode(player, cmd) {
			ack := r.ackLocked(cmd, player, false, "empty_skill_slot", "basic attack is not assigned to the active combat mode")
			r.queueAckLocked(cmd.GetContext().GetSessionId(), ack)
			return ack, nil
		}
	}

	switch cmd.GetType() {
	case gamev1.CommandType_COMMAND_TYPE_MOVE:
		r.applyMove(player, cmd)
	case gamev1.CommandType_COMMAND_TYPE_TURN:
		r.applyTurn(player, cmd)
	case gamev1.CommandType_COMMAND_TYPE_DODGE:
		if ok, code, message := r.canApplyMovementActionContract("dodge", r.contracts.contractForAbility("dodge")); !ok {
			ack := r.ackLocked(cmd, player, false, code, message)
			r.queueAckLocked(cmd.GetContext().GetSessionId(), ack)
			return ack, nil
		}
		r.applyImpulse(player, cmd, r.contracts.contractForAbility("dodge"))
	case gamev1.CommandType_COMMAND_TYPE_LEAP:
		if ok, code, message := r.canApplyMovementActionContract("jump", r.contracts.contractForAbility("jump")); !ok {
			ack := r.ackLocked(cmd, player, false, code, message)
			r.queueAckLocked(cmd.GetContext().GetSessionId(), ack)
			return ack, nil
		}
		r.applyImpulse(player, cmd, r.contracts.contractForAbility("jump"))
	case gamev1.CommandType_COMMAND_TYPE_CAST_SKILL:
		if ok, code, message := r.canApplySkillContract(cmd.GetCastSkill().GetSkillId(), player); !ok {
			ack := r.ackLocked(cmd, player, false, code, message)
			r.queueAckLocked(cmd.GetContext().GetSessionId(), ack)
			return ack, nil
		}
		r.applySkill(player, cmd)
	case gamev1.CommandType_COMMAND_TYPE_BLOCK_START, gamev1.CommandType_COMMAND_TYPE_BLOCK_STOP, gamev1.CommandType_COMMAND_TYPE_PARRY:
		r.applyDefense(player, cmd)
	case gamev1.CommandType_COMMAND_TYPE_SWITCH_COMBAT_MODE:
		r.applyCombatMode(player, cmd)
	default:
		player.locomotion = r.locomotion("grounded", "idle", "", "idle", player.position, player.position, 0)
	}

	ack := r.ackLocked(cmd, player, true, "", "accepted")
	rememberPlayerCommand(player, cmd)
	r.queueAckLocked(cmd.GetContext().GetSessionId(), ack)
	return ack, nil
}

func playerCommandReplayState(player *entityState, cmd *gamev1.PlayerCommand) (duplicate bool, stale bool, reason string) {
	if player == nil || cmd == nil {
		return false, false, ""
	}
	key := playerCommandReplayKey(cmd)
	if key != "" && player.processedCommandIDs != nil {
		if _, seen := player.processedCommandIDs[key]; seen {
			return true, false, "duplicate_command_id"
		}
	}
	sequence := cmd.GetSequence()
	if sequence > 0 && player.lastSequence > 0 && sequence <= player.lastSequence {
		return false, true, fmt.Sprintf("sequence %d is not newer than last accepted sequence %d", sequence, player.lastSequence)
	}
	return false, false, ""
}

func rememberPlayerCommand(player *entityState, cmd *gamev1.PlayerCommand) {
	if player == nil || cmd == nil {
		return
	}
	key := playerCommandReplayKey(cmd)
	if key == "" {
		return
	}
	if player.processedCommandIDs == nil {
		player.processedCommandIDs = map[string]struct{}{}
	}
	if _, exists := player.processedCommandIDs[key]; exists {
		return
	}
	player.processedCommandIDs[key] = struct{}{}
	player.processedCommandOrder = append(player.processedCommandOrder, key)
	const maxRememberedPlayerCommands = 128
	if len(player.processedCommandOrder) <= maxRememberedPlayerCommands {
		return
	}
	removeCount := len(player.processedCommandOrder) - maxRememberedPlayerCommands
	for _, oldKey := range player.processedCommandOrder[:removeCount] {
		delete(player.processedCommandIDs, oldKey)
	}
	player.processedCommandOrder = append([]string(nil), player.processedCommandOrder[removeCount:]...)
}

func playerCommandReplayKey(cmd *gamev1.PlayerCommand) string {
	if cmd == nil {
		return ""
	}
	if commandID := strings.TrimSpace(cmd.GetCommandId()); commandID != "" {
		return commandID
	}
	if cmd.GetSequence() == 0 {
		return ""
	}
	return fmt.Sprintf("%s:%d", cmd.GetType().String(), cmd.GetSequence())
}

func (r *Runtime) canApplyMovementActionContract(abilityKey string, contract MovementActionRuntimeContract) (bool, string, string) {
	if contract.ID == "" || contract.AbilityKey == "" {
		return false, "missing_movement_contract", "movement action contract is not loaded: " + abilityKey
	}
	if movement.ActionDuration(contract) <= 0 {
		return false, "invalid_movement_contract", "movement action contract has no duration: " + abilityKey
	}
	if movement.ActionDistance(contract, 0) <= 0 && contract.ActionType != "turn" {
		return false, "invalid_movement_contract", "movement action contract has no distance: " + abilityKey
	}
	return true, "", ""
}

func (r *Runtime) canApplySkillContract(requestedSkillID string, player *entityState) (bool, string, string) {
	skillID := requestedSkillID
	if skillID == "" || skillID == "player_basic_attack" {
		skillID = nextBasicAttack(player)
	}
	contract := r.contracts.skillContract(skillID)
	if !contract.Enabled || contract.SkillID == "" {
		return false, "missing_skill_contract", "skill runtime contract is not loaded: " + skillID
	}
	if contract.MovementAction.ID == "" {
		return false, "missing_movement_contract", "skill movement action contract is not loaded: " + skillID
	}
	if movement.ActionDuration(contract.MovementAction) <= 0 {
		return false, "invalid_movement_contract", "skill movement action contract has no duration: " + skillID
	}
	return true, "", ""
}

func (r *Runtime) Health(ctx context.Context, _ *gamev1.Empty) (*gamev1.HealthResponse, error) {
	return &gamev1.HealthResponse{Healthy: true, Status: "healthy"}, nil
}

func (r *Runtime) Readiness(ctx context.Context, _ *gamev1.Empty) (*gamev1.ReadinessResponse, error) {
	if r == nil {
		return &gamev1.ReadinessResponse{Ready: false, Blockers: []string{"runtime is nil"}}, nil
	}
	strict := r.contracts.Source == runtimeContractSourceDB
	report := r.contracts.CoverageReport(strict)
	if !report.Ready {
		return &gamev1.ReadinessResponse{Ready: false, Blockers: report.Blockers()}, nil
	}
	return &gamev1.ReadinessResponse{Ready: true}, nil
}

func splitCoverageBlockers(err error) []string {
	if err == nil {
		return nil
	}
	message := strings.TrimSpace(err.Error())
	const prefix = "runtime contract coverage incomplete: "
	message = strings.TrimPrefix(message, prefix)
	if message == "" {
		return []string{err.Error()}
	}
	parts := strings.Split(message, ";")
	blockers := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			blockers = append(blockers, trimmed)
		}
	}
	if len(blockers) == 0 {
		return []string{err.Error()}
	}
	return blockers
}

func (r *Runtime) RuntimeStats(ctx context.Context, _ *gamev1.Empty) (*gamev1.RuntimeStatsResponse, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	phaseStatus := map[string]string{"runtime": "recovered_in_memory"}
	if r.contracts.Source != "" {
		phaseStatus["contract_source"] = r.contracts.Source
	}
	coverage := r.contracts.CoverageReport(r.contracts.Source == runtimeContractSourceDB)
	for _, category := range coverage.Categories {
		if category.Ready {
			phaseStatus["contracts."+category.Name] = "ready"
			continue
		}
		phaseStatus["contracts."+category.Name] = "blocked"
	}
	for key, value := range legacyRuntimeSurfaceStatusValues() {
		phaseStatus[key] = value
	}
	for key, value := range requirementStatusValues(r.contracts) {
		phaseStatus[key] = value
	}
	return &gamev1.RuntimeStatsResponse{
		Tick:                 r.serverTickLocked(),
		ActiveRegions:        1,
		ActiveSessions:       uint32(len(r.sessions)),
		ActiveEntities:       uint32(len(r.entities)),
		AverageFrameMs:       0.2,
		P95FrameMs:           0.5,
		PhaseStatus:          phaseStatus,
		SpawnedCreatureCount: uint64(r.spawnedCreatureCountLocked()),
	}, nil
}

func (r *Runtime) spawnedCreatureCountLocked() int {
	count := 0
	for _, entity := range r.entities {
		if entity.entityType == "creature" {
			count++
		}
	}
	return count
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
		id:                    r.nextRuntimeIDLocked(),
		entityType:            "creature",
		regionID:              defaultRegionID,
		templateID:            "steppe_wolf",
		archetype:             "wolf",
		visualID:              "steppe_wolf",
		position:              vector{x: player.position.x + 520, y: player.position.y + 120, z: player.position.z},
		yaw:                   180,
		health:                160,
		maxHealth:             160,
		stamina:               100,
		maxStamina:            100,
		posture:               100,
		maxPosture:            100,
		movementState:         "orbit",
		combatState:           "engaged",
		skillState:            "idle",
		aggroState:            "engaged",
		aggression:            0.75,
		creatureCooldownUntil: map[string]time.Time{},
	}
	if r.contracts.WolfPolicy.MaxStamina > 0 {
		wolf.maxStamina = r.contracts.WolfPolicy.MaxStamina
		wolf.stamina = wolf.maxStamina
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
	rangeCM := distance(wolf.position, player.position)
	start := wolf.position

	policy := r.contracts.WolfPolicy
	r.regenerateCreatureStaminaLocked(wolf, policy)
	lungeMinRangeCM := positiveOr(policy.LungeMinRangeCM, policy.LungeRangeCM)
	lungeMaxRangeCM := positiveOr(policy.LungeMaxRangeCM, policy.ChaseRangeCM)
	nowTime := time.Now()
	activeSkill := ""
	activeSkillElapsedTicks := uint64(0)
	if skillID, active := r.activeCreatureSkillLocked(wolf, nowTime); active {
		activeSkill = skillID
		if wolf.skillRuntime != nil && wolf.skillRuntime.GetStartedAtMs() > 0 {
			elapsedMS := nowTime.UnixMilli() - wolf.skillRuntime.GetStartedAtMs()
			if elapsedMS > 0 {
				activeSkillElapsedTicks = uint64(math.Ceil(float64(elapsedMS) * tickRate / 1000))
			}
		}
	}
	decision := r.creatureBrainSystemLocked().Decide(fmt.Sprintf("creature:%d", wolf.id), wolfBrainPolicy(policy), creatureai.Input{
		Tick:                    r.tick,
		CreaturePosition:        toDomainVector(wolf.position),
		TargetPosition:          toDomainVector(player.position),
		TargetFacingYaw:         player.yaw,
		ActiveSkillID:           activeSkill,
		ActiveSkillElapsedTicks: activeSkillElapsedTicks,
		LineOfSight:             true,
		Pressure:                wolf.aggression,
		Perception:              creaturePerceptionFromTarget(player, nowTime),
		ResourceCurrent:         wolf.stamina,
		ResourceMax:             wolf.maxStamina,
		SkillCosts:              r.creatureSkillCostsLocked(policy),
		UnavailableSkill:        r.creatureUnavailableSkillsLocked(wolf, nowTime),
	})
	action := decision.Action
	selectedSkill := decision.SelectedSkill

	selectedRuntime := r.contracts.skillContract(selectedSkill)
	actionUpdate := r.applyCreatureActionRuntimeLocked(wolf, player, decision, selectedRuntime, start, nowTime)
	resolvedMotion := creatureDecisionMotion{Start: start}
	if !actionUpdate.RootMotionApplied {
		resolvedMotion = resolveGroundedCreatureDecisionMotion(wolf, decision)
		applyCreatureDecisionMotion(wolf, player, decision, resolvedMotion)
	} else {
		resolvedMotion.Motion.Start = toDomainVector(start)
		resolvedMotion.Motion.Projected = toDomainVector(wolf.position)
		resolvedMotion.Motion.Velocity = toDomainVector(wolf.velocity)
		resolvedMotion.Motion.DistanceCM = distance(start, wolf.position)
		resolvedMotion.Motion.SpeedCMPerSecond = length(wolf.velocity)
	}
	r.publishWolfLocomotionLocked(wolf, decision, selectedRuntime, resolvedMotion, nowTime)
	r.publishWolfAIStateLocked(wolf, decision, policy, selectedRuntime, rangeCM, lungeMinRangeCM, lungeMaxRangeCM)
	if actionUpdate.Active && wolf.locomotion != nil {
		wolf.locomotion.Phase = string(actionUpdate.Phase)
		applyActionInstanceLocomotionTiming(wolf.locomotion, wolf.actionInstance, nowTime)
	} else if creatureActionMotionComplete(wolf, nowTime) && !creatureai.PublishesSkill(action) {
		wolf.skillState = "idle"
	}
}

func creaturePerceptionFromTarget(target *entityState, now time.Time) creatureai.Perception {
	if target == nil {
		return creatureai.Perception{}
	}
	combatState := strings.ToLower(strings.TrimSpace(target.combatState))
	skillState := strings.ToLower(strings.TrimSpace(target.skillState))
	return creatureai.Perception{
		TargetVelocityCMPerSec: toDomainVector(target.velocity),
		TargetMovementState:    target.movementState,
		TargetCombatState:      target.combatState,
		TargetSkillState:       target.skillState,
		TargetActionActive:     target.actionInstance != nil && target.actionInstance.PhaseAt(now) != actionruntime.PhaseComplete,
		TargetBlocking:         combatState == "blocking" || combatState == "block" || combatState == "guard",
		TargetParrying:         combatState == "parry" || combatState == "parry_active" || combatState == "perfect_block",
		TargetIFrame:           combatState == "iframe" || combatState == "evade" || combatState == "dodge" || skillState == "dodge",
		TargetResourceCurrent:  target.stamina,
		TargetResourceMax:      target.maxStamina,
		TargetPostureCurrent:   target.posture,
		TargetPostureMax:       target.maxPosture,
	}
}

func (r *Runtime) resolveCreatureSkillImpactLocked(creature *entityState, player *entityState, skill SkillRuntimeContract, now time.Time) []runtimeSkillImpact {
	schedule, ok := r.creatureSkillImpactScheduleLocked(creature, player, skill, now)
	if !ok {
		return nil
	}
	return r.resolveSkillImpactScheduleLocked(schedule)
}

func (r *Runtime) enqueueCreatureSkillImpactLocked(creature *entityState, player *entityState, skill SkillRuntimeContract, now time.Time) bool {
	schedule, ok := r.creatureSkillImpactScheduleLocked(creature, player, skill, now)
	if !ok {
		return false
	}
	return r.enqueueSkillImpactScheduleLocked(schedule)
}

func (r *Runtime) creatureSkillImpactScheduleLocked(creature *entityState, player *entityState, skill SkillRuntimeContract, now time.Time) (skillImpactSchedule, bool) {
	if r == nil || creature == nil || player == nil || creature.skillRuntime == nil || skill.SkillID == "" {
		return skillImpactSchedule{}, false
	}
	startedAtMS := creature.skillRuntime.GetStartedAtMs()
	if startedAtMS <= 0 || creature.skillRuntime.GetCurrentSkillId() != skill.SkillID {
		return skillImpactSchedule{}, false
	}
	elapsedMS := float64(now.UnixMilli() - startedAtMS)

	dir := normalize(vector{x: player.position.x - creature.position.x, y: player.position.y - creature.position.y})
	if dir == (vector{}) {
		dir = yawVector(creature.yaw)
	}
	reach := skillRangeToCM(skill.Range)
	if reach <= 0 {
		reach = movement.ActionDistance(skill.MovementAction, 0)
	}
	if reach <= 0 {
		reach = maxSkillHitboxReachCM(skill)
	}
	end := vector{x: creature.position.x + dir.x*reach, y: creature.position.y + dir.y*reach, z: creature.position.z}
	instanceID := ""
	startedAt := time.UnixMilli(startedAtMS)
	if creature.actionInstance != nil && creature.actionInstance.SkillID.String() == skill.SkillID {
		instanceID = creature.actionInstance.InstanceID
		startedAt = creature.actionInstance.StartedAt
	}
	return skillImpactScheduleFromActionInstance(
		creature,
		skill,
		instanceID,
		startedAt,
		creature.position,
		end,
		dir,
		elapsedMS,
	), true
}

func skillHasTemporalImpactWindowAt(skill SkillRuntimeContract, elapsedMS float64) bool {
	for _, profile := range skill.Hitboxes {
		if profile == nil {
			continue
		}
		startMS := float64(profile.GetHitboxStartMs())
		endMS := float64(profile.GetHitboxEndMs())
		if endMS <= startMS {
			return true
		}
		if elapsedMS+impactTemporalEpsilon >= startMS && elapsedMS-impactTemporalEpsilon <= endMS {
			return true
		}
	}
	return false
}

func maxSkillHitboxReachCM(skill SkillRuntimeContract) float64 {
	reach := 0.0
	for _, profile := range skill.Hitboxes {
		if profile == nil {
			continue
		}
		reach = maxFloat64(reach, profile.GetOffsetX()+profile.GetLength(), profile.GetLength(), profile.GetRadius())
		if motion := profile.GetMotionProfile(); motion != nil {
			for _, sample := range motion.GetSamples() {
				if sample == nil {
					continue
				}
				reach = maxFloat64(reach, sample.GetOffsetX()+sample.GetLength(), sample.GetLength(), sample.GetRadius())
			}
		}
	}
	return reach
}

func (r *Runtime) creatureSkillCostsLocked(policy WolfRuntimePolicy) map[string]float64 {
	costs := map[string]float64{}
	for _, binding := range policy.SkillBehaviorBindings {
		if binding.SkillID == "" {
			continue
		}
		contract := r.contracts.skillContract(binding.SkillID)
		cost := contract.StaminaCost
		if binding.SkillID == policy.DodgeSkillID && policy.DodgeStaminaCostMultiplier > 0 {
			cost *= policy.DodgeStaminaCostMultiplier
		}
		if cost > 0 {
			costs[binding.SkillID] = cost
		}
	}
	if len(costs) == 0 {
		return nil
	}
	return costs
}

func (r *Runtime) spendCreatureSkillStaminaLocked(creature *entityState, skillID string, contract SkillRuntimeContract) {
	if creature == nil || skillID == "" {
		return
	}
	cost := contract.StaminaCost
	if skillID == r.contracts.WolfPolicy.DodgeSkillID && r.contracts.WolfPolicy.DodgeStaminaCostMultiplier > 0 {
		cost *= r.contracts.WolfPolicy.DodgeStaminaCostMultiplier
	}
	if cost <= 0 {
		return
	}
	creature.stamina = math.Max(0, creature.stamina-cost)
}

func (r *Runtime) regenerateCreatureStaminaLocked(creature *entityState, policy WolfRuntimePolicy) {
	if creature == nil || policy.StaminaRegenPerSecond <= 0 {
		return
	}
	maxStamina := creature.maxStamina
	if policy.MaxStamina > 0 {
		maxStamina = policy.MaxStamina
		creature.maxStamina = maxStamina
	}
	if maxStamina <= 0 || creature.stamina >= maxStamina {
		return
	}
	creature.stamina = math.Min(maxStamina, creature.stamina+(policy.StaminaRegenPerSecond/tickRate))
}

func (r *Runtime) creatureUnavailableSkillsLocked(creature *entityState, now time.Time) map[string]string {
	if creature == nil {
		return nil
	}
	if creature.creatureCooldownUntil == nil {
		creature.creatureCooldownUntil = map[string]time.Time{}
	}
	unavailable := map[string]string{}
	for skillID, until := range creature.creatureCooldownUntil {
		if now.Before(until) {
			unavailable[skillID] = "cooldown"
			continue
		}
		delete(creature.creatureCooldownUntil, skillID)
	}
	if len(unavailable) == 0 {
		return nil
	}
	return unavailable
}

func (r *Runtime) startCreatureSkillCooldownLocked(creature *entityState, skillID string, contract SkillRuntimeContract, now time.Time) {
	if creature == nil || skillID == "" || contract.CooldownMS <= 0 {
		return
	}
	if creature.creatureCooldownUntil == nil {
		creature.creatureCooldownUntil = map[string]time.Time{}
	}
	creature.creatureCooldownUntil[skillID] = now.Add(durationFromMS(contract.CooldownMS))
}

func (r *Runtime) creatureBrainSystemLocked() *creatureai.RegionBrainSystem {
	if r.aiSystem == nil {
		r.aiSystem = creatureai.NewRegionBrainSystem()
	}
	return r.aiSystem
}

func wolfBrainPolicy(policy WolfRuntimePolicy) creatureai.Policy {
	return creatureai.Policy{
		ContractID:                     policy.ContractID,
		ContractHash:                   policy.ContractHash,
		CapabilityID:                   policy.CapabilityID,
		DesiredRangeCM:                 policy.DesiredRangeCM,
		ChaseRangeCM:                   policy.ChaseRangeCM,
		RetreatRangeCM:                 policy.RetreatRangeCM,
		OrbitSpeedCMS:                  policy.OrbitSpeedCMS,
		ChaseSpeedCMS:                  policy.ChaseSpeedCMS,
		LungeSpeedCMS:                  policy.LungeSpeedCMS,
		MaulSpeedCMS:                   policy.MaulSpeedCMS,
		RetreatSpeedCMS:                policy.RetreatSpeedCMS,
		DodgeSkillID:                   policy.DodgeSkillID,
		ApproachMinDistanceCM:          policy.ApproachMinDistanceCM,
		ApproachMaxDistanceCM:          policy.ApproachMaxDistanceCM,
		BiteRangeCM:                    policy.BiteRangeCM,
		LungeMinRangeCM:                policy.LungeMinRangeCM,
		LungeMaxRangeCM:                policy.LungeMaxRangeCM,
		MaulPressureThreshold:          policy.MaulPressureThreshold,
		DodgeUnderPressure:             policy.DodgeUnderPressure,
		MaulCounterUnderPressure:       policy.MaulCounterUnderPressure,
		MaulCounterChance:              policy.MaulCounterChance,
		DodgeRetreatMultiplier:         policy.DodgeRetreatMultiplier,
		GlobalDodgeMultiplier:          policy.GlobalDodgeMultiplier,
		OrbitLocomotionMode:            policy.OrbitLocomotionMode,
		OrbitSpeedScale:                policy.OrbitSpeedScale,
		MinOrbitDurationTicks:          msToRuntimeTicks(policy.MinOrbitDurationMS),
		SideSwitchCooldownTicks:        msToRuntimeTicks(policy.SideSwitchCooldownMS),
		AllowSideSwitchWhenTargetFaces: policy.AllowSideSwitchWhenTargetFaces,
		PreferLongSideCommit:           policy.PreferLongSideCommit,
		SideFlipChanceMultiplier:       policy.SideFlipChanceMultiplier,
		LockSideDuringSetup:            policy.LockSideDuringSetup,
		RepeatSkillPenaltyWindowTicks:  msToRuntimeTicks(policy.RepeatSkillPenaltyWindowMS),
		RepeatSkillPenaltyMultiplier:   policy.RepeatSkillPenaltyMultiplier,
		Bindings:                       wolfBrainBindings(policy.SkillBehaviorBindings),
		SetupPolicies:                  wolfBrainSetupPolicies(policy.SkillSetupPolicies),
	}
}

func wolfBrainBindings(bindings []CreatureSkillBehaviorRuntimeBinding) []creatureai.SkillBinding {
	out := make([]creatureai.SkillBinding, 0, len(bindings))
	for _, binding := range bindings {
		out = append(out, creatureai.SkillBinding{
			ID:                  binding.ID,
			SkillID:             binding.SkillID,
			TacticalState:       binding.TacticalState,
			DecisionPhase:       binding.DecisionPhase,
			SetupPolicyID:       binding.SetupPolicyID,
			MinRangeCM:          binding.MinRangeCM,
			MaxRangeCM:          binding.MaxRangeCM,
			Priority:            binding.Priority,
			UsageWeight:         binding.UsageWeight,
			CooldownGroup:       binding.CooldownGroup,
			RequiresLineOfSight: binding.RequiresLineOfSight,
			Enabled:             binding.Enabled,
		})
	}
	return out
}

func wolfBrainSetupPolicies(policies []CreatureSkillSetupRuntimePolicy) map[string]creatureai.SkillSetupPolicy {
	if len(policies) == 0 {
		return nil
	}
	out := make(map[string]creatureai.SkillSetupPolicy, len(policies))
	for _, policy := range policies {
		if policy.ID == "" || !policy.Enabled {
			continue
		}
		out[policy.ID] = creatureai.SkillSetupPolicy{
			ID:                  policy.ID,
			SkillID:             policy.SkillID,
			SetupType:           policy.SetupType,
			MinSetupTicks:       msToRuntimeTicks(policy.MinSetupMS),
			MaxSetupTicks:       msToRuntimeTicks(policy.MaxSetupMS),
			CommitDistanceCM:    policy.CommitDistanceCM,
			PreferredMinRangeCM: policy.PreferredMinRangeCM,
			PreferredMaxRangeCM: policy.PreferredMaxRangeCM,
			MovementTactic:      policy.MovementTactic,
			LockSideDuringSetup: policy.LockSideDuringSetup,
			Enabled:             policy.Enabled,
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func msToRuntimeTicks(ms int32) uint64 {
	if ms <= 0 {
		return 0
	}
	return uint64(math.Ceil(float64(ms) * tickRate / 1000))
}

func (r *Runtime) activeCreatureSkillLocked(creature *entityState, now time.Time) (string, bool) {
	if creature == nil || creature.skillRuntime == nil {
		return "", false
	}
	skillID := creature.skillRuntime.GetCurrentSkillId()
	startedAtMS := creature.skillRuntime.GetStartedAtMs()
	if skillID == "" || startedAtMS <= 0 {
		return "", false
	}
	if creature.actionInstance != nil && creature.actionInstance.SkillID.String() == skillID {
		return skillID, creature.actionInstance.PhaseAt(now) != actionruntime.PhaseComplete
	}
	contract := r.contracts.skillContract(skillID)
	duration := durationFromMS(contract.WindupMS + contract.ActiveMS + contract.RecoveryMS)
	if duration <= 0 {
		duration = durationFromMS(contract.MovementAction.DurationMS)
	}
	if duration <= 0 {
		return "", false
	}
	return skillID, now.Before(time.UnixMilli(startedAtMS).Add(duration))
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

func (r *Runtime) creaturesEnabled() bool {
	return !r.options.MovementValidation && !r.options.DisableCreatures
}

func (r *Runtime) isBasicAttackCommandLockedByCombatMode(player *entityState, cmd *gamev1.PlayerCommand) bool {
	if player == nil || cmd.GetCastSkill() == nil {
		return false
	}
	skillID := cmd.GetCastSkill().GetSkillId()
	if skillID != "" && skillID != "player_basic_attack" && !strings.HasPrefix(skillID, "player_basic_attack_") {
		return false
	}
	activeMode := swordShieldBulwarkModeID
	if player.combatMode != nil {
		activeMode = player.combatMode.GetActiveCombatMode()
	}
	return !isBulwarkCombatMode(activeMode)
}

func (r *Runtime) applyMove(player *entityState, cmd *gamev1.PlayerCommand) {
	if r.advanceActionMotionLocked(player, time.Now()) {
		if player.actionMotion != nil && blocksNormalInputDuringOwnedRoot(player.actionMotion.NormalInputPolicy) {
			player.lastSequence = cmd.GetSequence()
			player.lastClientTick = cmd.GetClientTick()
			return
		}
	}

	move := cmd.GetMove()
	dir := normalize(fromProto(move.GetDirection()))
	if dir == (vector{}) {
		player.velocity = vector{}
		player.movementState = "idle"
		player.locomotion = r.locomotion("grounded", "move_stop", "move", "recovery", player.position, player.position, cmd.GetSequence())
		return
	}
	facingYaw := player.yaw
	if move.TargetYaw != nil {
		facingYaw = move.GetTargetYaw()
	}
	motion := movement.ResolveGroundedMove(movement.GroundedMoveInput{
		Position:        toDomainVector(player.position),
		Direction:       toDomainVector(dir),
		FacingYawDeg:    facingYaw,
		AnalogMagnitude: move.GetAnalogMagnitude(),
		Sprint:          move.GetSprint(),
		TickRate:        tickRate,
		Profile:         r.movementSpeedProfile(),
	})
	if player.combatState == "blocking" {
		motion = capBlockMotion(motion, r.movementSpeedProfile())
	}
	start := player.position
	player.position = fromDomainVector(motion.Projected)
	player.velocity = fromDomainVector(motion.Velocity)
	player.movementState = "moving"
	if move.TargetYaw != nil {
		player.yaw = move.GetTargetYaw()
	}
	player.locomotion = locomotionFromContractWithOverrides(r.contracts.contractForAbility("move"), "active", start, player.position, r.tick, cmd.GetSequence(), motion.SpeedCMPerSecond, motion.DistanceCM)
	player.locomotion.MovementMode = "grounded"
	player.locomotion.Action = "move"
	player.locomotion.AbilityKey = "move"
}

// blockSpeedWalkFraction caps movement while blocking: the player moves at most at half
// the walk speed (chat-13 rule "block reduz pra metade da velocidade de walk", applied
// even when sprinting).
const blockSpeedWalkFraction = 0.5

func capBlockMotion(motion movement.MotionResult, profile movement.SpeedProfile) movement.MotionResult {
	if motion.Stopped || motion.SpeedCMPerSecond <= 0 || profile.MaxSpeed <= 0 {
		return motion
	}
	capSpeed := profile.MaxSpeed * blockSpeedWalkFraction
	if motion.SpeedCMPerSecond <= capSpeed {
		return motion
	}
	ratio := capSpeed / motion.SpeedCMPerSecond
	capped := motion
	capped.SpeedCMPerSecond = capSpeed
	capped.DistanceCM = motion.DistanceCM * ratio
	capped.Velocity = motion.Direction.Scale(capSpeed)
	capped.Projected = motion.Start.Add(motion.Direction.Scale(capped.DistanceCM))
	return capped
}

func (r *Runtime) groundedMoveSpeed(sprint bool, analogMagnitude float64, dir vector, facingYaw float64) float64 {
	return movement.GroundedMoveSpeed(r.movementSpeedProfile(), sprint, analogMagnitude, toDomainVector(dir), facingYaw)
}

func (r *Runtime) movementSpeedProfile() movement.SpeedProfile {
	profile := r.contracts.MovementProfile
	if profile == nil {
		profile = &gamev1.MovementReconciliationProfile{}
	}
	return movement.SpeedProfile{
		MaxSpeed:                       profile.GetMaxSpeed(),
		SprintSpeedMultiplier:          profile.GetSprintSpeedMultiplier(),
		StrafeSpeedMultiplier:          profile.GetStrafeSpeedMultiplier(),
		BackpedalSpeedMultiplier:       profile.GetBackpedalSpeedMultiplier(),
		StrafeSprintSpeedMultiplier:    profile.GetStrafeSprintSpeedMultiplier(),
		BackpedalSprintSpeedMultiplier: profile.GetBackpedalSprintSpeedMultiplier(),
	}
}

func (r *Runtime) applyTurn(player *entityState, cmd *gamev1.PlayerCommand) {
	turn := cmd.GetTurn()
	player.yaw = turn.GetTargetYaw()
	if player.locomotion == nil {
		player.locomotion = r.locomotion("grounded", "turn", "turn", "active", player.position, player.position, cmd.GetSequence())
	}
	player.locomotion.AuthoritativeYaw = player.yaw
	player.locomotion.LastUpdatedTick = r.tick
}

func (r *Runtime) applyImpulse(player *entityState, cmd *gamev1.PlayerCommand, contract MovementActionRuntimeContract) {
	dir := vector{x: 1}
	if cmd.GetDodge() != nil {
		dir = normalize(fromProto(cmd.GetDodge().GetDirection()))
	}
	if cmd.GetLeap() != nil {
		dir = normalize(fromProto(cmd.GetLeap().GetDirection()))
	}
	if dir == (vector{}) {
		if cmd.GetLeap() == nil {
			dir = yawVector(player.yaw)
		}
	}
	start := player.position
	fullMotion := movement.ResolveActionMotion(movement.ActionMotionInput{
		Position:  toDomainVector(player.position),
		Direction: toDomainVector(dir),
		Contract:  contract,
	})
	progress := movement.ResolveActionMotionProgress(movement.ActionMotionProgressInput{
		Position:  toDomainVector(start),
		Direction: toDomainVector(dir),
		Contract:  contract,
		Elapsed:   0,
	})
	player.actionMotion = &actionMotionState{
		CommandID:         cmd.GetCommandId(),
		Sequence:          cmd.GetSequence(),
		ClientTick:        cmd.GetClientTick(),
		StartedAt:         time.Now(),
		StartPosition:     start,
		ProjectedPosition: fromDomainVector(fullMotion.Projected),
		Direction:         dir,
		Contract:          contract,
		NormalInputPolicy: "blocked_during_owned_root",
		TotalDistanceCM:   fullMotion.DistanceCM,
	}
	player.position = start
	player.velocity = fromDomainVector(progress.Velocity)
	player.movementState = contract.ActionType
	player.skillState = contract.AbilityKey
	player.locomotion = locomotionFromContractWithOverrides(contract, "active", start, player.position, r.tick, cmd.GetSequence(), fullMotion.SpeedCMPerSecond, progress.DistanceCM)
	player.locomotion.ActionDistanceTraveled = progress.DistanceCM
	player.locomotion.ActionProjectedPosition = toProto(fromDomainVector(fullMotion.Projected))

	lockMS := contract.DurationMS
	if lockMS <= 0 {
		lockMS = contract.ActiveMS + contract.RecoveryMS
	}
	if lockMS <= 0 {
		lockMS = 300
	}
	player.actionLockedUntil = time.Now().Add(time.Duration(lockMS) * time.Millisecond)
	player.actionLockReason = "active_locomotion:" + contract.ActionType
}

// skillActionLockedLocked reports whether the player is mid-leap/dodge (an owned
// movement action) and therefore must not start a skill or basic attack. Caller holds
// r.mu. Restores the chat 6 #3 rule "no skill while jumping/dodging" in the live runtime.
func (r *Runtime) skillActionLockedLocked(player *entityState) (string, bool) {
	if player != nil && player.actionMotion != nil && blocksNormalInputDuringOwnedRoot(player.actionMotion.NormalInputPolicy) {
		if player.actionMotion.SkillID != "" {
			return "active_skill_root_motion:" + player.actionMotion.SkillID, true
		}
		return "active_skill_root_motion", true
	}
	if player == nil || player.actionLockedUntil.IsZero() {
		return "", false
	}
	if time.Now().Before(player.actionLockedUntil) {
		reason := player.actionLockReason
		if reason == "" {
			reason = "active_action"
		}
		return reason, true
	}
	return "", false
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
	start := player.position
	fullMotion := movement.ResolveActionMotion(movement.ActionMotionInput{
		Position:  toDomainVector(player.position),
		Direction: toDomainVector(dir),
		Contract:  skillContract.MovementAction,
	})
	nowTime := time.Now()
	instance := r.newActionInstance(player, cmd, skillID, skillContract, start, nowTime)
	player.actionInstance = &instance
	player.actionMotion = nil
	if !fullMotion.Stopped && fullMotion.DistanceCM > 0 {
		player.actionMotion = &actionMotionState{
			SkillID:           skillID,
			CommandID:         cmd.GetCommandId(),
			Sequence:          cmd.GetSequence(),
			ClientTick:        cmd.GetClientTick(),
			StartedAt:         nowTime,
			StartPosition:     start,
			ProjectedPosition: fromDomainVector(fullMotion.Projected),
			Direction:         dir,
			Contract:          skillContract.MovementAction,
			NormalInputPolicy: skillContract.NormalInputPolicy,
			TotalDistanceCM:   fullMotion.DistanceCM,
		}
	}
	progress := movement.ResolveActionMotionProgress(movement.ActionMotionProgressInput{
		Position:  toDomainVector(start),
		Direction: toDomainVector(dir),
		Contract:  skillContract.MovementAction,
		Elapsed:   0,
	})
	player.position = start
	player.velocity = fromDomainVector(progress.Velocity)
	player.skillState = "active"
	player.combatState = "committed"
	player.locomotion = locomotionFromContractWithOverrides(skillContract.MovementAction, string(instance.PhaseAt(nowTime)), start, start, r.tick, cmd.GetSequence(), fullMotion.SpeedCMPerSecond, progress.DistanceCM)
	player.locomotion.ActionDistanceTraveled = progress.DistanceCM
	applyActionInstanceLocomotionTiming(player.locomotion, &instance, nowTime)
	now := nowTime.UnixMilli()
	player.skillRuntime = &gamev1.SkillRuntimeState{
		CurrentSkillId:   skillID,
		State:            string(instance.PhaseAt(nowTime)),
		StartedAtMs:      now,
		CooldownEndMs:    time.UnixMilli(now).Add(durationFromMS(skillContract.CooldownMS)).UnixMilli(),
		LastResolvedAtMs: now,
	}
	r.enqueueSkillImpactScheduleLocked(skillImpactScheduleFromActionInstance(
		player,
		skillContract,
		instance.InstanceID,
		instance.StartedAt,
		start,
		fromDomainVector(fullMotion.Projected),
		dir,
		skillImpactEvaluationElapsedMS(skillContract),
	))
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
		mode = swordShieldBulwarkModeID
	}
	mode = normalizeCombatModeID(mode)
	player.combatMode = swordShieldCombatMode(mode, r.contracts.CombatModes)
}

func (r *Runtime) newActionInstance(player *entityState, cmd *gamev1.PlayerCommand, skillID string, contract SkillRuntimeContract, start vector, now time.Time) actionruntime.Instance {
	timing := actionruntime.Timing{
		Windup:     durationFromMS(contract.WindupMS),
		Active:     durationFromMS(contract.ActiveMS),
		Recovery:   durationFromMS(contract.RecoveryMS),
		Cooldown:   durationFromMS(contract.CooldownMS),
		ActionLock: durationFromMS(contract.WindupMS + contract.ActiveMS + contract.RecoveryMS),
	}
	actionKind := actionruntime.ActionKindActiveSkill
	if strings.HasPrefix(skillID, "player_basic_attack_") {
		actionKind = actionruntime.ActionKindWeaponBasic
	}
	return actionruntime.NewInstance(actionruntime.NewInstanceSpec{
		InstanceID:           actionruntime.NewInstanceID(ids.RuntimeEntityID(player.id), skillID, cmd.GetCommandId(), cmd.GetSequence(), r.tick),
		EntityID:             ids.RuntimeEntityID(player.id),
		ActorKind:            actionruntime.ActorKindPlayer,
		ActionKind:           actionKind,
		SkillID:              ids.SkillID(skillID),
		CommandID:            cmd.GetCommandId(),
		CommandSequence:      cmd.GetSequence(),
		ServerActionSequence: r.tick,
		ClientTick:           cmd.GetClientTick(),
		StartedAt:            now,
		Timing:               timing,
		Cooldown:             timing.Cooldown,
		MovementContract:     movementActionContractForRuntime(contract.MovementAction),
		HasMovementContract:  contract.MovementAction.ID != "",
		ActionStartPosition:  toDomainVector(start),
		MovementLockedUntil:  now.Add(timing.ActionLock),
		GlobalLockedUntil:    now.Add(timing.Cooldown),
		RecoveryEndsAt:       now.Add(timing.Windup + timing.Active + timing.Recovery),
	})
}

func (r *Runtime) refreshActionRuntimeStatesLocked(now time.Time) {
	for _, entity := range r.entities {
		r.advanceActionMotionLocked(entity, now)
		if entity.actionInstance == nil || entity.skillRuntime == nil {
			continue
		}
		phase := entity.actionInstance.PhaseAt(now)
		entity.skillRuntime.State = string(phase)
		entity.skillRuntime.LastResolvedAtMs = now.UnixMilli()
		if phase == actionruntime.PhaseComplete {
			entity.skillState = "idle"
			entity.combatState = "ready"
			entity.actionMotion = nil
			continue
		}
		entity.skillState = string(phase)
	}
}

func (r *Runtime) advanceActionMotionLocked(entity *entityState, now time.Time) bool {
	if entity == nil || entity.actionMotion == nil {
		return false
	}
	motion := entity.actionMotion
	progress := movement.ResolveActionMotionProgress(movement.ActionMotionProgressInput{
		Position:           toDomainVector(motion.StartPosition),
		Direction:          toDomainVector(motion.Direction),
		Contract:           motion.Contract,
		FallbackDistanceCM: motion.TotalDistanceCM,
		Elapsed:            now.Sub(motion.StartedAt),
	})

	projected := fromDomainVector(progress.Projected)
	velocity := fromDomainVector(progress.Velocity)
	distanceCM := progress.DistanceCM
	contact := r.resolveActionMotionContactResponseLocked(entity, motion, projected, velocity, distanceCM)
	if contact.Applied {
		projected = contact.Position
		velocity = contact.Velocity
		distanceCM = contact.DistanceCM
	}

	entity.position = projected
	entity.velocity = velocity
	entity.movementState = motion.Contract.ActionType
	if motion.SkillID != "" {
		entity.skillState = "active"
		entity.combatState = "committed"
	}

	phase := "active"
	if entity.actionInstance != nil {
		instancePhase := entity.actionInstance.PhaseAt(now)
		if instancePhase == actionruntime.PhaseComplete {
			phase = "recovery"
		} else {
			phase = string(instancePhase)
		}
	}
	entity.locomotion = locomotionFromContractWithOverrides(motion.Contract, phase, motion.StartPosition, entity.position, r.tick, motion.Sequence, progress.SpeedCMPerSecond, progress.DistanceCM)
	entity.locomotion.ActionDistanceTraveled = distanceCM
	entity.locomotion.ActionProjectedPosition = toProto(entity.position)
	entity.locomotion.ClientActionSequence = motion.Sequence
	entity.locomotion.ServerReceivedTick = r.tick
	entity.locomotion.LastUpdatedTick = r.tick
	applyActionInstanceLocomotionTiming(entity.locomotion, entity.actionInstance, now)

	if progress.Complete || contact.Stopped {
		if progress.Complete && !contact.Stopped {
			entity.position = motion.ProjectedPosition
		}
		entity.velocity = vector{}
		entity.locomotion.ActionProjectedPosition = toProto(entity.position)
		if progress.Complete && !contact.Stopped {
			entity.locomotion.ActionDistanceTraveled = progress.TotalDistanceCM
		} else {
			entity.locomotion.ActionDistanceTraveled = distanceCM
		}
		entity.actionMotion = nil
		return false
	}
	return true
}

func blocksNormalInputDuringOwnedRoot(policy string) bool {
	normalized := strings.ToLower(strings.TrimSpace(policy))
	switch normalized {
	case "allow", "allowed", "none", "normal", "free":
		return false
	case "", "blocked_during_owned_root", "buffer_until_recovery_handoff":
		return true
	default:
		return strings.Contains(normalized, "block") || strings.Contains(normalized, "buffer")
	}
}

func movementActionContractForRuntime(contract MovementActionRuntimeContract) movement.MovementActionContract {
	return movement.MovementActionContract{
		ID:                    contract.ID,
		MovementAction:        contract.ActionType,
		MovementType:          contract.ActionType,
		ReconciliationMode:    movement.ReconciliationMode(contract),
		PredictionErrorPolicy: contract.PredictionErrorPolicy,
		Enabled:               true,
		DurationMS:            contract.DurationMS,
		ActiveMS:              contract.ActiveMS,
		RecoveryMS:            contract.RecoveryMS,
		HorizontalDistanceCM:  contract.DistanceCM,
		BaseSpeedCMPerSec:     contract.BaseSpeedCMS,
		MovementMode:          "grounded",
	}
}

func durationFromMS(ms int32) time.Duration {
	if ms <= 0 {
		return 0
	}
	return time.Duration(ms) * time.Millisecond
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
		contract := r.contracts.contractForAbility(skillID)
		metadata["skill_id"] = skillID
		metadata["movement_action_type"] = contract.ActionType
		metadata["ability_key"] = skillID
		metadata["movement_action_contract_id"] = contract.ID
		metadata["contract_hash"] = contractHash(contract)
		metadata["movement_action_contract_hash"] = contractHash(contract)
		metadata["movement_action_contract_sync_state"] = "confirmed"
		if player != nil && player.actionInstance != nil {
			metadata["action_instance_id"] = player.actionInstance.InstanceID
			metadata["action_phase"] = string(player.actionInstance.PhaseAt(time.Now()))
			metadata["action_kind"] = string(player.actionInstance.ActionKind)
		}
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
	state := locomotionFromContractWithOverrides(contract, phase, start, projected, r.tick, sequence, 0, 0)
	state.MovementMode = mode
	state.Action = action
	state.AbilityKey = ability
	return state
}

// locomotionResolver is the single locomotion-policy authority (internal/movement).
// The gameapi runtime publishes what it returns and adds only the verbose wire-only
// fields below; it must not compute reconciliation/timing/distance/speed on its own.
var locomotionResolver = movement.NewResolver()

func locomotionFromContractWithOverrides(contract MovementActionRuntimeContract, phase string, start, projected vector, tick uint64, sequence uint64, overrideTargetSpeedCMPerSec, overrideDistanceCM float64) *gamev1.LocomotionState {
	state := locomotionFromContract(contract, phase, start, projected, tick, sequence)
	if overrideDistanceCM > 0 {
		state.ActionDistanceTraveled = overrideDistanceCM
	}
	if overrideTargetSpeedCMPerSec > 0 {
		state.TargetSpeed = overrideTargetSpeedCMPerSec
		state.EffectiveSpeed = overrideTargetSpeedCMPerSec
	}
	return state
}

func locomotionFromContract(contract MovementActionRuntimeContract, phase string, start, projected vector, tick uint64, sequence uint64) *gamev1.LocomotionState {
	loco := locomotionResolver.ResolveRuntime(contract, phase)
	return &gamev1.LocomotionState{
		MovementMode:            loco.MovementMode,
		Action:                  loco.Action,
		AbilityKey:              loco.AbilityKey,
		Phase:                   loco.Phase,
		ReconciliationMode:      loco.ReconciliationMode,
		PhaseElapsedMs:          loco.PhaseElapsedMS,
		PhaseRemainingMs:        loco.PhaseRemainingMS,
		DurationMs:              loco.DurationMS,
		AirborneDurationMs:      contract.AirborneDurationMS,
		StartupMs:               loco.StartupMS,
		ActiveMs:                loco.ActiveMS,
		RecoveryMs:              loco.RecoveryMS,
		SpeedCurveSamples:       movementCurveSamplesToProto(contract.SpeedCurveSamples),
		VerticalCurveSamples:    movementCurveSamplesToProto(contract.VerticalCurveSamples),
		JumpZVelocity:           contract.JumpZVelocity,
		GravityScale:            contract.GravityScale,
		ExpectedApexMs:          contract.ExpectedApexMS,
		LandingDetectionPolicy:  contract.LandingDetectionPolicy,
		GroundZPolicy:           contract.GroundZPolicy,
		CapsuleBaseOffset:       contract.CapsuleBaseOffset,
		AllowsAirControl:        contract.AllowsAirControl,
		AirControlModifier:      contract.AirControlModifier,
		ContractVersion:         "movement_action_v1",
		ContractHash:            contractHash(contract),
		PhaseWindowPolicy:       loco.PhaseWindowPolicy,
		PredictionErrorPolicy:   loco.PredictionErrorPolicy,
		ActionContractId:        loco.ActionContractID,
		ActionFamily:            actionFamily(contract),
		MovementType:            loco.MovementType,
		ContractSyncState:       "confirmed",
		ClientActionSequence:    sequence,
		ServerReceivedTick:      tick,
		ServerActionStartedTick: tick,
		ActionStartedTick:       tick,
		ActionStartPosition:     toProto(start),
		ActionProjectedPosition: toProto(projected),
		ActionDistanceTraveled:  loco.ActionDistanceTraveled,
		TargetSpeed:             loco.TargetSpeed,
		EffectiveSpeed:          loco.EffectiveSpeed,
		YawRate:                 contract.YawRateDegPerSec,
		LastUpdatedTick:         tick,
	}
}

func applyActionInstanceLocomotionTiming(state *gamev1.LocomotionState, instance *actionruntime.Instance, now time.Time) {
	if state == nil || instance == nil || instance.StartedAt.IsZero() {
		return
	}
	windup := nonNegativeDuration(instance.Timing.Windup)
	active := nonNegativeDuration(instance.Timing.Active)
	recovery := nonNegativeDuration(instance.Timing.Recovery)
	total := windup + active + recovery
	if total <= 0 {
		return
	}

	elapsed := now.Sub(instance.StartedAt)
	if elapsed < 0 {
		elapsed = 0
	}

	state.StartupMs = durationMillis(windup)
	state.ActiveMs = durationMillis(active)
	state.RecoveryMs = durationMillis(recovery)
	state.DurationMs = durationMillis(total)

	phase := strings.ToLower(strings.TrimSpace(state.GetPhase()))
	switch phase {
	case "windup", "accepted", "startup":
		state.PhaseElapsedMs = durationMillis(minDuration(elapsed, windup))
		state.PhaseRemainingMs = durationMillis(clampDuration(windup-elapsed, 0, windup))
	case "active":
		activeElapsed := clampDuration(elapsed-windup, 0, active)
		state.PhaseElapsedMs = durationMillis(activeElapsed)
		state.PhaseRemainingMs = durationMillis(active - activeElapsed)
	case "recovery", "complete":
		recoveryElapsed := clampDuration(elapsed-windup-active, 0, recovery)
		state.PhaseElapsedMs = durationMillis(recoveryElapsed)
		state.PhaseRemainingMs = durationMillis(recovery - recoveryElapsed)
	default:
		phase, phaseElapsed, phaseRemaining := locomotionResolver.ResolvePhase(elapsed, durationMillis(windup), durationMillis(active), durationMillis(recovery))
		state.Phase = phase
		state.PhaseElapsedMs = phaseElapsed
		state.PhaseRemainingMs = phaseRemaining
	}
}

func nonNegativeDuration(d time.Duration) time.Duration {
	if d < 0 {
		return 0
	}
	return d
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func clampDuration(value, minValue, maxValue time.Duration) time.Duration {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func durationMillis(d time.Duration) int32 {
	if d <= 0 {
		return 0
	}
	return int32(math.Round(float64(d) / float64(time.Millisecond)))
}

func movementCurveSamplesToProto(samples []movement.MovementActionCurvePoint) []*gamev1.MovementCurveSample {
	if len(samples) == 0 {
		return nil
	}
	out := make([]*gamev1.MovementCurveSample, 0, len(samples))
	for _, sample := range samples {
		out = append(out, &gamev1.MovementCurveSample{
			T:     sample.T,
			Scale: sample.Value,
		})
	}
	return out
}

func swordShieldCombatMode(active string, slots []*gamev1.CombatModeSlot) *gamev1.CombatModeState {
	active = normalizeCombatModeID(active)
	if active == "" {
		active = swordShieldBulwarkModeID
	}
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

func toDomainVector(v vector) domainmath.Vec3 {
	return domainmath.V3(v.x, v.y, v.z)
}

func fromDomainVector(v domainmath.Vec3) vector {
	return vector{x: v.X, y: v.Y, z: v.Z}
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
