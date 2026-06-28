package gameapi

import (
	"context"
	dbv1 "db-apeiron/gen/apeiron/v1"
	"fmt"
	"math"
	"net"
	"strconv"
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
	"google.golang.org/protobuf/proto"
)

const (
	defaultRegionID = "old_china_test_region"
	defaultWorldID  = "apeiron"
	defaultZoneID   = "plain_test_map"
	defaultBiomeID  = "frontier_grassland"
	tickRate        = 30

	// PlainTestMap player actor-root height. This was previously embedded at
	// spawn only; keeping it named prevents leap/grounded paths from preserving
	// contaminated airborne Z after recovery handoffs.
	defaultPlayerGroundRootZ = 98.0
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
	packs     map[string]*packRuntime
	aiSystem  *creatureai.RegionBrainSystem
	impact    *combatpipeline.ImpactResolutionPipeline
	impacts   *damagegroup.Runtime[skillImpactSchedule]

	playerSource PlayerProgressionSource
}

// PlayerProgressionSource loads and persists player progression (level/xp/attributes/coin) via the
// data service. The live runtime sets it from the db-apeiron client (read on attach, written by the
// periodic flush); tests leave it nil and keep entity defaults. Signatures match
// dbv1.PlayerDataServiceClient.
type PlayerProgressionSource interface {
	GetPlayer(ctx context.Context, in *dbv1.IdRequest, opts ...grpc.CallOption) (*dbv1.PlayerResponse, error)
	UpdatePlayer(ctx context.Context, in *dbv1.Player, opts ...grpc.CallOption) (*dbv1.OperationResult, error)
}

// playerProgression holds the DB-authoritative character progression for a player entity. nil for
// creatures.
type playerProgression struct {
	level           int32
	experience      int64
	attributePoints int32
	strength        float64
	dexterity       float64
	intelligence    float64
	endurance       float64
	coin            int64
	dirty           bool // set when runtime mutates progression; cleared once persisted
}

func defaultPlayerProgression() *playerProgression {
	return &playerProgression{level: 1, strength: 1, dexterity: 1, intelligence: 1, endurance: 1}
}

// SetPlayerProgressionSource wires the data service used to load and persist player progression.
func (r *Runtime) SetPlayerProgressionSource(source PlayerProgressionSource) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.playerSource = source
}

func (r *Runtime) hasPlayerProgressionSource() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.playerSource != nil
}

type RuntimeOptions struct {
	MovementValidation bool
	DisableCreatures   bool
	CreaturePackSize   int
}

type sessionState struct {
	id        string
	accountID string
	playerID  string
	regionID  string
}

type entityState struct {
	id                          uint64
	entityType                  string
	regionID                    string
	templateID                  string
	archetype                   string
	visualID                    string
	position                    vector
	velocity                    vector
	yaw                         float64
	health                      float64
	maxHealth                   float64
	stamina                     float64
	maxStamina                  float64
	staminaSpendLockedUntilFull bool
	posture                     float64
	maxPosture                  float64
	movementState               string
	combatState                 string
	skillState                  string
	aggroState                  string
	aggression                  float64
	impactResponseProfile       string
	progression                 *playerProgression
	groundRootZ                 float64
	groundRootKnown             bool
	lastSequence                uint64
	lastClientTick              uint64
	processedCommandIDs         map[string]struct{}
	processedCommandOrder       []string
	locomotion                  *gamev1.LocomotionState
	skillRuntime                *gamev1.SkillRuntimeState
	actionInstance              *actionruntime.Instance
	actionMotion                *actionMotionState
	creatureActionTransition    *creatureActionTransitionState
	actionOrientationLatch      *creatureActionOrientationLatch
	orientationFocusYaw         float64
	orientationFocusYawKnown    bool
	orientationAttackYaw        float64
	orientationAttackYawKnown   bool
	threat                      *threatTable
	damageCredits               map[uint64]float64
	creatureLeashed             bool
	packID                      string
	packRingSlotDeg             float64
	packSlotKnown               bool
	packRole                    string
	packFocusTargetID           uint64
	lastCommitAt                time.Time
	actionHandoffUntil          time.Time
	actionHandoffAction         string
	combatMode                  *gamev1.CombatModeState
	creatureAI                  *gamev1.CreatureAIState
	playerCooldownUntil         map[string]time.Time
	creatureCooldownUntil       map[string]time.Time
	creatureActiveSetupPolicyID string

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
	MotionSource      string
	StartedAt         time.Time
	StartedTick       uint64
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
	UseVerticalRoot   bool
	ReaimedAtTakeoff  bool
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

	if runtime.hasPlayerProgressionSource() {
		go runtime.runProgressionFlushLoop(ctx, progressionFlushInterval)
	}

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
	playerID := req.GetPlayerId()
	if playerID == "" {
		playerID = "local_player"
	}
	// Load persisted progression before taking the lock — the gRPC call must not block the runtime.
	progression := r.fetchPlayerProgression(ctx, playerID)

	r.mu.Lock()
	defer r.mu.Unlock()

	sessionID := req.GetContext().GetSessionId()
	if sessionID != "" {
		if session := r.sessions[sessionID]; session != nil {
			session.playerID = playerID
			if req.GetPreferredRegionId() != "" {
				session.regionID = req.GetPreferredRegionId()
			}
		}
	}

	_, alreadyAttached := r.players[playerID]
	player := r.ensurePlayerLocked(playerID)
	if !alreadyAttached && progression != nil {
		applyPlayerProgressionLocked(player, progression)
	}
	r.clearExpiredOwnedRootMotionForAttachLocked(player, time.Now())
	resetPlayerCommandReplayState(player)
	if r.creaturesEnabled() {
		r.ensureWolfPackLocked(player, r.options.CreaturePackSize)
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
	impactEvents := r.damageEventsFromImpactsLocked(r.runPendingSkillImpactSchedulesLocked(now))
	out := &gamev1.SnapshotResponse{
		Tick:        r.serverTickLocked(),
		Region:      regionRef(),
		Entities:    make([]*gamev1.SnapshotEntity, 0, len(r.entities)),
		Events:      impactEvents,
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
		if cooldownUntil, onCooldown := r.playerSkillCooldownLocked(player, resolvedPlayerSkillID(player, cmd.GetCastSkill().GetSkillId()), now); onCooldown {
			ack := r.ackLocked(cmd, player, false, "cooldown_active", "skill cooldown is active")
			if ack.Metadata == nil {
				ack.Metadata = map[string]string{}
			}
			skillID := resolvedPlayerSkillID(player, cmd.GetCastSkill().GetSkillId())
			cooldownMS := r.contracts.skillContract(skillID).CooldownMS
			ack.Metadata["skill_id"] = skillID
			ack.Metadata["ability_key"] = skillID
			ack.Metadata["skill_cooldown_ms"] = strconv.FormatInt(int64(cooldownMS), 10)
			ack.Metadata["cooldown_until_ms"] = strconv.FormatInt(cooldownUntil.UnixMilli(), 10)
			ack.Metadata["cooldown_remaining_ms"] = strconv.FormatInt(cooldownUntil.Sub(now).Milliseconds(), 10)
			r.queueAckLocked(cmd.GetContext().GetSessionId(), ack)
			return ack, nil
		}
	}

	switch cmd.GetType() {
	case gamev1.CommandType_COMMAND_TYPE_MOVE:
		if ok, code, message := r.canApplyMovementActionContract("move", r.contracts.contractForAbility("move")); !ok {
			ack := r.ackLocked(cmd, player, false, code, message)
			r.queueAckLocked(cmd.GetContext().GetSessionId(), ack)
			return ack, nil
		}
		r.applyMove(player, cmd)
	case gamev1.CommandType_COMMAND_TYPE_TURN:
		if ok, code, message := r.canApplyMovementActionContract("turn", r.contracts.contractForAbility("turn")); !ok {
			ack := r.ackLocked(cmd, player, false, code, message)
			r.queueAckLocked(cmd.GetContext().GetSessionId(), ack)
			return ack, nil
		}
		r.applyTurn(player, cmd)
	case gamev1.CommandType_COMMAND_TYPE_DODGE:
		r.logDodgeDebugStateLocked("submit_dodge_before_validation", player, map[string]string{
			"command_id":  cmd.GetCommandId(),
			"sequence":    strconv.FormatUint(cmd.GetSequence(), 10),
			"client_tick": strconv.FormatUint(cmd.GetClientTick(), 10),
		})
		if ok, code, message := r.canApplyMovementActionContract("dodge", r.contracts.contractForAbility("dodge")); !ok {
			r.logDodgeDebugStateLocked("submit_dodge_rejected_contract", player, map[string]string{"code": code, "message": message})
			ack := r.ackLocked(cmd, player, false, code, message)
			r.queueAckLocked(cmd.GetContext().GetSessionId(), ack)
			return ack, nil
		}
		if ok, code, message := r.spendPlayerDodgeStaminaLocked(player); !ok {
			r.logDodgeDebugStateLocked("submit_dodge_rejected_stamina", player, map[string]string{"code": code, "message": message})
			ack := r.ackLocked(cmd, player, false, code, message)
			r.queueAckLocked(cmd.GetContext().GetSessionId(), ack)
			return ack, nil
		}
		r.applyImpulse(player, cmd, r.contracts.contractForAbility("dodge"))
		r.logDodgeDebugStateLocked("submit_dodge_after_apply", player, map[string]string{
			"command_id":  cmd.GetCommandId(),
			"sequence":    strconv.FormatUint(cmd.GetSequence(), 10),
			"client_tick": strconv.FormatUint(cmd.GetClientTick(), 10),
		})
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

func resetPlayerCommandReplayState(player *entityState) {
	if player == nil {
		return
	}
	player.lastSequence = 0
	player.lastClientTick = 0
	player.processedCommandIDs = nil
	player.processedCommandOrder = nil
}

func (r *Runtime) clearExpiredOwnedRootMotionForAttachLocked(player *entityState, now time.Time) {
	if player == nil || player.actionMotion == nil || player.actionMotion.MotionSource != "owned_locomotion" || player.actionMotion.StartedAt.IsZero() {
		return
	}

	expiry := durationFromMS(player.actionMotion.Contract.DurationMS)
	if strings.EqualFold(player.actionMotion.Contract.ActionType, "dodge") && r.contracts.MovementProfile != nil {
		expiry += durationFromMS(r.contracts.MovementProfile.GetDodgeCarryHandoffMs())
	}
	if expiry <= 0 || now.Sub(player.actionMotion.StartedAt) < expiry {
		return
	}

	player.velocity = vector{}
	player.movementState = "grounded"
	player.skillState = "idle"
	player.combatState = "ready"
	player.actionLockedUntil = time.Time{}
	player.actionLockReason = ""
	if player.locomotion != nil && strings.EqualFold(player.locomotion.GetAction(), player.actionMotion.Contract.ActionType) {
		player.locomotion.Phase = "complete"
		player.locomotion.MovementMode = "grounded"
		player.locomotion.PhaseElapsedMs = 0
		player.locomotion.PhaseRemainingMs = 0
		player.locomotion.TargetSpeed = 0
		player.locomotion.EffectiveSpeed = 0
		player.locomotion.LandingHandoffActive = false
		player.locomotion.LandingExitDirection = nil
		player.locomotion.LandingExitSpeed = 0
		player.locomotion.LastUpdatedTick = r.tick
	}
	player.actionHandoffUntil = time.Time{}
	player.actionHandoffAction = ""
	player.actionMotion = nil
}

func resolvedPlayerSkillID(player *entityState, requestedSkillID string) string {
	if requestedSkillID == "" || requestedSkillID == "player_basic_attack" {
		return nextBasicAttack(player)
	}
	return requestedSkillID
}

func isPlayerBasicAttackSkillID(skillID string) bool {
	return skillID == "player_basic_attack" || strings.HasPrefix(skillID, "player_basic_attack_")
}

func (r *Runtime) playerSkillCooldownLocked(player *entityState, skillID string, now time.Time) (time.Time, bool) {
	if player == nil || skillID == "" || isPlayerBasicAttackSkillID(skillID) {
		return time.Time{}, false
	}
	if player.playerCooldownUntil == nil {
		player.playerCooldownUntil = map[string]time.Time{}
		return time.Time{}, false
	}
	until, ok := player.playerCooldownUntil[skillID]
	if !ok {
		return time.Time{}, false
	}
	if now.Before(until) {
		return until, true
	}
	delete(player.playerCooldownUntil, skillID)
	return time.Time{}, false
}

func (r *Runtime) startPlayerSkillCooldownLocked(player *entityState, skillID string, contract SkillRuntimeContract, now time.Time) {
	if player == nil || skillID == "" || isPlayerBasicAttackSkillID(skillID) || contract.CooldownMS <= 0 {
		return
	}
	if player.playerCooldownUntil == nil {
		player.playerCooldownUntil = map[string]time.Time{}
	}
	player.playerCooldownUntil[skillID] = now.Add(durationFromMS(contract.CooldownMS))
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
	if movement.ActionDistance(contract, 0) <= 0 && abilityKey != "move" && contract.ActionType != "turn" {
		return false, "invalid_movement_contract", "movement action contract has no distance: " + abilityKey
	}
	return true, "", ""
}

func (r *Runtime) canApplySkillContract(requestedSkillID string, player *entityState) (bool, string, string) {
	skillID := resolvedPlayerSkillID(player, requestedSkillID)
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
	phaseStatus := map[string]string{"runtime": "apeiron_server_runtime"}
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
	for key, value := range compatRuntimeSurfaceStatusValues() {
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
	playerCombatProfile := r.contracts.combatCoreProfileForEntity(&entityState{entityType: "player"})
	maxStamina := 100.0
	if playerCombatProfile != nil && playerCombatProfile.GetMaxStamina() > 0 {
		maxStamina = playerCombatProfile.GetMaxStamina()
	}
	entity := &entityState{
		id:                    r.nextRuntimeIDLocked(),
		entityType:            "player",
		regionID:              defaultRegionID,
		templateID:            "player_sword_shield",
		archetype:             "sword_shield",
		visualID:              "player",
		position:              vector{x: -2500, y: 1900, z: defaultPlayerGroundRootZ},
		groundRootZ:           defaultPlayerGroundRootZ,
		groundRootKnown:       true,
		yaw:                   0,
		health:                100,
		maxHealth:             100,
		stamina:               maxStamina,
		maxStamina:            maxStamina,
		posture:               100,
		maxPosture:            100,
		movementState:         "grounded",
		combatState:           "ready",
		skillState:            "idle",
		impactResponseProfile: r.contracts.playerImpactResponse(),
	}
	entity.locomotion = r.locomotion("grounded", "idle", "", "idle", entity.position, entity.position, 0)
	entity.combatMode = swordShieldCombatMode("mode_sword_shield_bulwark", r.contracts.CombatModes)
	entity.progression = defaultPlayerProgression()
	r.players[playerID] = entity
	r.entities[entity.id] = entity
	return entity
}

// fetchPlayerProgression loads persisted progression for playerID. It is nil-safe (no source / no
// player / error → nil, caller keeps defaults) and must be called WITHOUT holding r.mu.
func (r *Runtime) fetchPlayerProgression(ctx context.Context, playerID string) *dbv1.Player {
	r.mu.Lock()
	source := r.playerSource
	r.mu.Unlock()
	if source == nil || playerID == "" {
		return nil
	}
	resp, err := source.GetPlayer(ctx, &dbv1.IdRequest{Id: playerID})
	if err != nil {
		logging.WithComponent("gameapi").Warn().Err(err).Str("player_id", playerID).
			Msg("player progression load failed; using defaults")
		return nil
	}
	if !resp.GetFound() {
		return nil
	}
	return resp.GetPlayer()
}

// applyPlayerProgressionLocked copies DB-authoritative progression onto the player entity.
func applyPlayerProgressionLocked(player *entityState, p *dbv1.Player) {
	if player == nil || p == nil {
		return
	}
	if player.progression == nil {
		player.progression = defaultPlayerProgression()
	}
	prog := player.progression
	prog.level = p.GetLevel()
	prog.experience = p.GetExperience()
	prog.attributePoints = p.GetAttributePoints()
	prog.strength = p.GetStrength()
	prog.dexterity = p.GetDexterity()
	prog.intelligence = p.GetIntelligence()
	prog.endurance = p.GetEndurance()
	prog.coin = p.GetCoin()
}

func (r *Runtime) ensureWolfLocked(player *entityState) *entityState {
	for _, entity := range r.entities {
		if entity.entityType == "creature" && entity.templateID == "steppe_wolf" {
			return entity
		}
	}
	return r.spawnSteppeWolfLocked(player, vector{x: player.position.x + 520, y: player.position.y + 120, z: player.position.z})
}

func (r *Runtime) countSteppeWolvesLocked() int {
	n := 0
	for _, e := range r.entities {
		if e != nil && e.entityType == "creature" && e.templateID == "steppe_wolf" {
			n++
		}
	}
	return n
}

// ensureWolfPackLocked ensures `count` steppe wolves exist, clustered near the base spawn so they
// form one pack. count 1 preserves the single-wolf behavior; higher (CREATURE_PACK_SIZE) lets pack
// coordination be seen in PIE.
func (r *Runtime) ensureWolfPackLocked(player *entityState, count int) {
	if count < 1 {
		count = 1
	}
	r.ensureWolfLocked(player)
	for r.countSteppeWolvesLocked() < count {
		i := r.countSteppeWolvesLocked()
		r.spawnSteppeWolfLocked(player, vector{
			x: player.position.x + 520 + float64(i)*260,
			y: player.position.y + 120 + float64(i%2)*180,
			z: player.position.z,
		})
	}
}

func (r *Runtime) spawnSteppeWolfLocked(player *entityState, position vector) *entityState {
	wolf := &entityState{
		id:                    r.nextRuntimeIDLocked(),
		entityType:            "creature",
		regionID:              defaultRegionID,
		templateID:            "steppe_wolf",
		archetype:             "wolf",
		visualID:              "steppe_wolf",
		position:              position,
		groundRootZ:           player.position.z,
		groundRootKnown:       true,
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
		impactResponseProfile: strings.TrimSpace(r.contracts.WolfPolicy.ImpactResponseProfile),
		creatureCooldownUntil: map[string]time.Time{},
	}
	if r.contracts.WolfPolicy.MaxStamina > 0 {
		wolf.maxStamina = r.contracts.WolfPolicy.MaxStamina
		wolf.stamina = wolf.maxStamina
	}
	wolf.locomotion = r.locomotion("grounded", "orbit", "run", "active", wolf.position, wolf.position, 0)
	wolf.creatureAI = &gamev1.CreatureAIState{
		MovementTactic: "flank",
		CombatTactic:   "harass",
		Commitment:     "probing",
		CapabilityId:   r.contracts.WolfPolicy.CapabilityID,
		ContractId:     r.contracts.WolfPolicy.ContractID,
		ContractHash:   r.contracts.WolfPolicy.ContractHash,
		OrbitSide:      "left",
		LastReason:     "spawned_from_db_contracts",
		BehaviorFamily: "beast_harasser",
		CombatRole:     "duelist",
		DesiredRangeCm: r.contracts.WolfPolicy.DesiredRangeCM,
		ActualRangeCm:  distance(wolf.position, player.position),
		ProfileSource:  r.contracts.Source,
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
	now := time.Now()
	// Group nearby creatures into packs, then slot members around their target so they surround.
	r.formCreaturePacksLocked()
	r.assignPackFocusLocked()
	r.assignPackRingSlotsLocked(now)
	r.assignPackRolesLocked()
	for _, creature := range r.entities {
		if creature.entityType != "creature" || creature.templateID != "steppe_wolf" {
			continue
		}
		// Leash/reset: if pulled too far from home, disengage and walk back before any combat.
		if r.updateCreatureLeashLocked(creature, now) {
			continue
		}
		// Combat target: pack focus distribution when assigned, else the member's own threat
		// selection. Resolves to the single player unchanged when there is only one (no-regression).
		target := r.resolveCreatureCombatTargetLocked(creature, now)
		if target == nil {
			target = player
		}
		r.updateWolfPolicyLocked(creature, target)
	}
}

func (r *Runtime) updateWolfPolicyLocked(wolf *entityState, player *entityState) {
	rangeCM := distance(wolf.position, player.position)
	start := wolf.position

	policy := r.contracts.WolfPolicy
	r.regenerateCreatureStaminaLocked(wolf, policy)
	r.accrueProximityThreatLocked(wolf, 1.0/tickRate)
	r.decayCreatureThreatLocked(wolf, 1.0/tickRate)
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
		ActiveSetupPolicyID:     wolf.creatureActiveSetupPolicyID,
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

	// Pack commit budget: a member without a commit token cannot enter a committed action this
	// turn; it falls back to tactical harass/reposition so only the budgeted number attack at once.
	if creatureai.PublishesSkill(decision.Action) && r.isCommittingSkill(selectedSkill) && !r.packMayCommitLocked(wolf) {
		decision = suppressCommitDecision(decision)
		action = decision.Action
		selectedSkill = decision.SelectedSkill
	}

	selectedRuntime := r.contracts.skillContract(selectedSkill)
	actionUpdate := r.applyCreatureActionRuntimeLocked(wolf, player, decision, selectedRuntime, start, nowTime)
	resolvedMotion := creatureDecisionMotion{Start: start}
	if !actionUpdate.RootMotionApplied {
		// Bias tactical orbit toward this member's pack ring slot so the pack surrounds the target.
		decision = r.applyPackSlotSteeringLocked(wolf, player, decision)
		resolvedMotion = resolveGroundedCreatureDecisionMotion(wolf, player, decision)
		applyCreatureDecisionMotion(wolf, player, decision, resolvedMotion)
	} else {
		resolvedMotion.Motion.Start = toDomainVector(start)
		resolvedMotion.Motion.Projected = toDomainVector(wolf.position)
		resolvedMotion.Motion.Velocity = toDomainVector(wolf.velocity)
		resolvedMotion.Motion.DistanceCM = distance(start, wolf.position)
		resolvedMotion.Motion.SpeedCMPerSecond = length(wolf.velocity)
	}
	r.publishWolfLocomotionLocked(wolf, decision, selectedRuntime, actionUpdate, resolvedMotion, nowTime)
	r.publishWolfAIStateLocked(wolf, player, decision, policy, selectedRuntime, actionUpdate, rangeCM, lungeMinRangeCM, lungeMaxRangeCM, nowTime)
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
	targetIFrame := runtimeEntityHasIFrameStateAt(target, now)
	return creatureai.Perception{
		TargetVelocityCMPerSec: toDomainVector(target.velocity),
		TargetMovementState:    target.movementState,
		TargetCombatState:      target.combatState,
		TargetSkillState:       target.skillState,
		TargetActionActive:     target.actionInstance != nil && target.actionInstance.PhaseAt(now) != actionruntime.PhaseComplete,
		TargetBlocking:         combatState == "blocking" || combatState == "block" || combatState == "guard",
		TargetParrying:         combatState == "parry" || combatState == "parry_active" || combatState == "perfect_block",
		TargetIFrame:           targetIFrame || combatState == "iframe" || combatState == "evade" || combatState == "dodge" || skillState == "dodge",
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

	instanceID := ""
	startedAt := time.UnixMilli(startedAtMS)
	if creature.actionInstance != nil && creature.actionInstance.SkillID.String() == skill.SkillID {
		instanceID = creature.actionInstance.InstanceID
		startedAt = creature.actionInstance.StartedAt
	}

	dir := normalize(vector{x: player.position.x - creature.position.x, y: player.position.y - creature.position.y})
	if dir == (vector{}) {
		dir = yawVector(creature.yaw)
	}
	// A committed action sweeps along its latched attack line, so the strike follows where
	// the creature actually lunged instead of re-aiming at the moving target every tick.
	if latch := creature.actionOrientationLatch; latch != nil && latch.Latched && instanceID != "" && latch.InstanceID == instanceID {
		if latchedDir := yawVector(latch.AttackYawDeg); latchedDir != (vector{}) {
			dir = latchedDir
		}
	}
	reach := skillRangeToCM(skill.Range)
	if reach <= 0 {
		reach = movement.ActionDistance(skill.MovementAction, 0)
	}
	if reach <= 0 {
		reach = maxSkillHitboxReachCM(skill)
	}
	end := vector{x: creature.position.x + dir.x*reach, y: creature.position.y + dir.y*reach, z: creature.position.z}
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
		TurnRateDegPerSec:              policy.TurnRateDegPerSec,
		DodgeSkillID:                   policy.DodgeSkillID,
		EvasionLateralBias:             policy.EvasionLateralBias,
		EvasionBackstepBias:            policy.EvasionBackstepBias,
		EvasionPressureThreshold:       policy.EvasionPressureThreshold,
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
		CommitThreatWeight:             policy.CommitThreatWeight,
		ClosingThreatWeight:            policy.ClosingThreatWeight,
		DefensiveBiteWeight:            policy.DefensiveBiteWeight,
		FleeingLungeWeight:             policy.FleeingLungeWeight,
		LowResourceRiskFloor:           policy.LowResourceRiskFloor,
		DodgeCommittedThreatMultiplier: policy.DodgeCommittedThreatMultiplier,
		VulnerableBiteMultiplier:       policy.VulnerableBiteMultiplier,
		VulnerableMaulMultiplier:       policy.VulnerableMaulMultiplier,
		TacticalDestinationDistanceCM:  policy.TacticalDestinationDistanceCM,
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
	move := cmd.GetMove()
	handoffApplied := r.applyMoveHandoffLocked(player, cmd, move)
	if !handoffApplied && r.advanceActionMotionLocked(player, time.Now()) {
		if player.actionMotion != nil && blocksNormalInputDuringOwnedRoot(player.actionMotion.NormalInputPolicy) {
			player.lastSequence = cmd.GetSequence()
			player.lastClientTick = cmd.GetClientTick()
			return
		}
	}

	dir := normalize(fromProto(move.GetDirection()))
	if dir == (vector{}) {
		if !handoffApplied {
			r.normalizePlayerGroundedRootLocked(player, r.contracts.contractForAbility("move"), "move_stop")
		}
		player.velocity = vector{}
		player.movementState = "idle"
		r.logGroundedMoveDebugStateLocked("move_stop", player, map[string]string{
			"sequence":    strconv.FormatUint(cmd.GetSequence(), 10),
			"client_tick": strconv.FormatUint(cmd.GetClientTick(), 10),
			"handoff":     strconv.FormatBool(handoffApplied),
		})
		if handoffApplied {
			contract := r.contracts.contractForAbility("jump")
			player.locomotion = locomotionFromContractWithOverrides(contract, "complete", player.position, player.position, r.tick, cmd.GetSequence(), 0, 0)
			player.locomotion.MovementMode = "grounded"
			player.locomotion.Action = "leap"
			player.locomotion.AbilityKey = "jump"
			player.locomotion.LandingHandoffActive = false
			player.locomotion.LastUpdatedTick = r.tick
		} else {
			player.locomotion = r.locomotion("grounded", "move_stop", "move", "recovery", player.position, player.position, cmd.GetSequence())
		}
		return
	}
	r.normalizePlayerGroundedRootLocked(player, r.contracts.contractForAbility("move"), "move")
	facingYaw := player.yaw
	if move.TargetYaw != nil {
		facingYaw = move.GetTargetYaw()
	}
	effectiveSprint := move.GetSprint()
	if effectiveSprint {
		effectiveSprint = r.spendPlayerSprintStaminaLocked(player)
	}
	motion := movement.ResolveGroundedMove(movement.GroundedMoveInput{
		Position:        toDomainVector(player.position),
		Direction:       toDomainVector(dir),
		FacingYawDeg:    facingYaw,
		AnalogMagnitude: move.GetAnalogMagnitude(),
		Sprint:          effectiveSprint,
		TickRate:        tickRate,
		Profile:         r.movementSpeedProfile(),
	})
	if player.combatState == "blocking" {
		motion = capBlockMotion(motion, r.movementSpeedProfile())
	}
	profile := r.movementSpeedProfile()
	start := player.position
	player.position = fromDomainVector(motion.Projected)
	player.velocity = fromDomainVector(motion.Velocity)
	player.movementState = "moving"
	if move.TargetYaw != nil {
		player.yaw = move.GetTargetYaw()
	}
	r.logGroundedMoveDebugStateLocked("move", player, map[string]string{
		"sequence":         strconv.FormatUint(cmd.GetSequence(), 10),
		"client_tick":      strconv.FormatUint(cmd.GetClientTick(), 10),
		"dir":              fmt.Sprintf("(%.3f,%.3f,%.3f)", dir.x, dir.y, dir.z),
		"analog":           strconv.FormatFloat(move.GetAnalogMagnitude(), 'f', 2, 64),
		"sprint":           strconv.FormatBool(move.GetSprint()),
		"effective_sprint": strconv.FormatBool(effectiveSprint),
		"stamina":          strconv.FormatFloat(player.stamina, 'f', 2, 64),
		"stamina_locked":   strconv.FormatBool(player.staminaSpendLockedUntilFull),
		"facing_yaw":       strconv.FormatFloat(facingYaw, 'f', 1, 64),
		"speed":            strconv.FormatFloat(motion.SpeedCMPerSecond, 'f', 1, 64),
		"distance":         strconv.FormatFloat(motion.DistanceCM, 'f', 1, 64),
		"profile_walk":     strconv.FormatFloat(profile.MaxSpeed, 'f', 1, 64),
		"profile_sprint":   strconv.FormatFloat(profile.SprintSpeedMultiplier, 'f', 2, 64),
		"profile_strafe":   strconv.FormatFloat(profile.StrafeSprintSpeedMultiplier, 'f', 2, 64),
		"profile_backped":  strconv.FormatFloat(profile.BackpedalSprintSpeedMultiplier, 'f', 2, 64),
	})
	player.locomotion = locomotionFromContractWithOverrides(r.contracts.contractForAbility("move"), "active", start, player.position, r.tick, cmd.GetSequence(), motion.SpeedCMPerSecond, motion.DistanceCM)
	player.locomotion.MovementMode = "grounded"
	player.locomotion.Action = "move"
	player.locomotion.AbilityKey = "move"
}

func (r *Runtime) applyMoveHandoffLocked(player *entityState, cmd *gamev1.PlayerCommand, move *gamev1.MoveCommand) bool {
	if player == nil || move == nil {
		return false
	}
	handoffAction := strings.ToLower(strings.TrimSpace(move.GetHandoffAction()))
	if handoffAction == "" {
		return false
	}
	if handoffAction != "leap" {
		return false
	}
	motion := player.actionMotion
	if motion == nil {
		r.logLeapDebugStateLocked("landing_handoff_ignored", player, map[string]string{
			"reason":              "no_active_motion",
			"handoff_sequence":    strconv.FormatUint(move.GetHandoffSequence(), 10),
			"handoff_client_tick": strconv.FormatUint(move.GetHandoffClientTick(), 10),
		})
		return false
	}
	if !strings.EqualFold(motion.MotionSource, "owned_locomotion") || !strings.EqualFold(motion.Contract.ActionType, "leap") {
		r.logLeapDebugStateLocked("landing_handoff_ignored", player, map[string]string{
			"reason":              "motion_not_active_leap",
			"motion_source":       motion.MotionSource,
			"motion_action":       motion.Contract.ActionType,
			"motion_sequence":     strconv.FormatUint(motion.Sequence, 10),
			"handoff_sequence":    strconv.FormatUint(move.GetHandoffSequence(), 10),
			"handoff_client_tick": strconv.FormatUint(move.GetHandoffClientTick(), 10),
		})
		return false
	}
	if move.GetHandoffSequence() > 0 && motion.Sequence > 0 && move.GetHandoffSequence() != motion.Sequence {
		r.logLeapDebugStateLocked("landing_handoff_ignored", player, map[string]string{
			"reason":              "sequence_mismatch",
			"motion_sequence":     strconv.FormatUint(motion.Sequence, 10),
			"handoff_sequence":    strconv.FormatUint(move.GetHandoffSequence(), 10),
			"handoff_client_tick": strconv.FormatUint(move.GetHandoffClientTick(), 10),
		})
		return false
	}

	contract := r.contracts.contractForAbility("jump")
	if pos := move.GetHandoffPosition(); pos != nil {
		player.position = r.playerGroundRootPosition(fromProto(pos), contract)
	} else {
		player.position = r.playerGroundRootPosition(player.position, contract)
	}
	handoffVelocity := fromProto(move.GetHandoffVelocity())
	handoffVelocity.z = 0
	player.velocity = handoffVelocity
	player.movementState = "grounded"
	player.skillState = "idle"
	player.combatState = "ready"
	player.actionMotion = nil
	player.actionLockedUntil = time.Time{}
	player.actionLockReason = ""
	if player.actionInstance != nil && string(player.actionInstance.SkillID) == "jump" {
		player.actionInstance = nil
	}

	start := player.position
	if motion != nil {
		start = motion.StartPosition
	}
	start = r.playerGroundRootPosition(start, contract)
	player.locomotion = locomotionFromContractWithOverrides(contract, "complete", start, player.position, r.tick, cmd.GetSequence(), 0, distance(start, player.position))
	player.locomotion.MovementMode = "grounded"
	player.locomotion.Action = "leap"
	player.locomotion.AbilityKey = "jump"
	player.locomotion.ClientActionSequence = move.GetHandoffSequence()
	player.locomotion.LastUpdatedTick = r.tick
	player.locomotion.LandingHandoffActive = false
	r.logLeapDebugStateLocked("landing_handoff_applied", player, map[string]string{
		"handoff_sequence":    strconv.FormatUint(move.GetHandoffSequence(), 10),
		"handoff_client_tick": strconv.FormatUint(move.GetHandoffClientTick(), 10),
	})
	return true
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

func (r *Runtime) playerGroundRootZ(contract MovementActionRuntimeContract) float64 {
	z := defaultPlayerGroundRootZ
	if strings.EqualFold(contract.GroundZPolicy, "server_position_is_actor_root") {
		z += contract.CapsuleBaseOffset
	}
	return z
}

func (r *Runtime) playerGroundRootPosition(pos vector, contract MovementActionRuntimeContract) vector {
	pos.z = r.playerGroundRootZ(contract)
	return pos
}

func (r *Runtime) entityGroundRootPosition(entity *entityState, pos vector) vector {
	if entity != nil && entity.groundRootKnown {
		pos.z = entity.groundRootZ
		return pos
	}
	if entity != nil {
		pos.z = entity.position.z
		return pos
	}
	return pos
}

func (r *Runtime) normalizePlayerGroundedRootLocked(player *entityState, contract MovementActionRuntimeContract, reason string) {
	if player == nil || player.entityType != "player" {
		return
	}
	if player.actionMotion != nil && player.actionMotion.MotionSource == "owned_locomotion" && player.actionMotion.UseVerticalRoot {
		return
	}
	grounded := r.playerGroundRootPosition(player.position, contract)
	if math.Abs(player.position.z-grounded.z) <= 0.01 {
		return
	}
	beforeZ := player.position.z
	player.position = grounded
	if player.velocity.z != 0 {
		player.velocity.z = 0
	}
	if player.locomotion != nil {
		player.locomotion.ActionProjectedPosition = toProto(player.position)
		player.locomotion.LastUpdatedTick = r.tick
	}
	r.logLeapDebugStateLocked("grounded_root_normalized", player, map[string]string{
		"reason":   reason,
		"before_z": strconv.FormatFloat(beforeZ, 'f', 1, 64),
		"after_z":  strconv.FormatFloat(player.position.z, 'f', 1, 64),
	})
}

func (r *Runtime) applyTurn(player *entityState, cmd *gamev1.PlayerCommand) {
	turn := cmd.GetTurn()
	r.normalizePlayerGroundedRootLocked(player, r.contracts.contractForAbility("move"), "turn")
	player.yaw = turn.GetTargetYaw()
	r.logGroundedMoveDebugStateLocked("turn", player, map[string]string{
		"sequence":    strconv.FormatUint(cmd.GetSequence(), 10),
		"client_tick": strconv.FormatUint(cmd.GetClientTick(), 10),
		"target_yaw":  strconv.FormatFloat(turn.GetTargetYaw(), 'f', 1, 64),
		"current_yaw": strconv.FormatFloat(turn.GetCurrentYaw(), 'f', 1, 64),
	})
	if player.locomotion == nil {
		player.locomotion = r.locomotion("grounded", "turn", "turn", "active", player.position, player.position, cmd.GetSequence())
	}
	player.locomotion.AuthoritativeYaw = player.yaw
	player.locomotion.LastUpdatedTick = r.tick
}

func (r *Runtime) applyImpulse(player *entityState, cmd *gamev1.PlayerCommand, contract MovementActionRuntimeContract) {
	nowTime := time.Now()
	if strings.EqualFold(contract.ActionType, "leap") {
		r.normalizePlayerGroundedRootLocked(player, contract, "leap_start")
	}
	r.logDodgeDebugStateLocked("owned_locomotion_begin_before_clear", player, map[string]string{
		"requested_action":  contract.ActionType,
		"requested_ability": contract.AbilityKey,
	})
	r.logLeapDebugStateLocked("owned_locomotion_begin_before_clear", player, map[string]string{
		"requested_action":  contract.ActionType,
		"requested_ability": contract.AbilityKey,
	})
	r.beginOwnedLocomotionActionLocked(player, nowTime)
	r.logDodgeDebugStateLocked("owned_locomotion_begin_after_clear", player, map[string]string{
		"requested_action":  contract.ActionType,
		"requested_ability": contract.AbilityKey,
	})
	r.logLeapDebugStateLocked("owned_locomotion_begin_after_clear", player, map[string]string{
		"requested_action":  contract.ActionType,
		"requested_ability": contract.AbilityKey,
	})

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
	if strings.EqualFold(contract.ActionType, "leap") {
		start = r.playerGroundRootPosition(start, contract)
		player.position = start
	}
	fullMotion := movement.ResolveActionMotion(movement.ActionMotionInput{
		Position:  toDomainVector(start),
		Direction: toDomainVector(dir),
		Contract:  contract,
	})
	progress := movement.ResolveActionMotionProgress(movement.ActionMotionProgressInput{
		Position:        toDomainVector(start),
		Direction:       toDomainVector(dir),
		Contract:        contract,
		Elapsed:         0,
		UseVerticalRoot: shouldUseOwnedLocomotionVerticalRoot(contract),
	})
	player.actionMotion = &actionMotionState{
		CommandID:         cmd.GetCommandId(),
		Sequence:          cmd.GetSequence(),
		ClientTick:        cmd.GetClientTick(),
		MotionSource:      "owned_locomotion",
		StartedAt:         nowTime,
		StartedTick:       r.tick,
		StartPosition:     start,
		ProjectedPosition: fromDomainVector(fullMotion.Projected),
		Direction:         dir,
		Contract:          contract,
		NormalInputPolicy: "blocked_during_owned_root",
		TotalDistanceCM:   fullMotion.DistanceCM,
		UseVerticalRoot:   shouldUseOwnedLocomotionVerticalRoot(contract),
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
	player.actionLockedUntil = nowTime.Add(time.Duration(lockMS) * time.Millisecond)
	player.actionLockReason = "active_locomotion:" + contract.ActionType
	r.logDodgeDebugStateLocked("owned_locomotion_apply_complete", player, map[string]string{
		"requested_action":  contract.ActionType,
		"requested_ability": contract.AbilityKey,
		"lock_ms":           strconv.FormatInt(int64(lockMS), 10),
	})
	r.logLeapDebugStateLocked("owned_locomotion_apply_complete", player, map[string]string{
		"requested_action":  contract.ActionType,
		"requested_ability": contract.AbilityKey,
		"lock_ms":           strconv.FormatInt(int64(lockMS), 10),
	})
}

func shouldUseOwnedLocomotionVerticalRoot(contract MovementActionRuntimeContract) bool {
	if !strings.EqualFold(contract.ActionType, "leap") {
		return false
	}
	return contract.VerticalMotionModel != "" || len(contract.VerticalCurveSamples) > 0 || contract.JumpZVelocity > 0
}

func (r *Runtime) beginOwnedLocomotionActionLocked(player *entityState, now time.Time) {
	if player == nil {
		return
	}
	r.cancelEntityActionImpactScheduleLocked(player)
	player.actionInstance = nil
	player.skillRuntime = nil
	player.creatureActiveSetupPolicyID = ""
	player.actionHandoffUntil = time.Time{}
	player.actionHandoffAction = ""
	if player.actionMotion != nil && player.actionMotion.MotionSource != "owned_locomotion" {
		player.actionMotion = nil
	}
	player.skillState = "idle"
	player.combatState = "ready"
	_ = now
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
	skillID := resolvedPlayerSkillID(player, cast.GetSkillId())
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
			MotionSource:      "skill_root",
			StartedAt:         nowTime,
			StartedTick:       r.tick,
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
	r.startPlayerSkillCooldownLocked(player, skillID, skillContract, nowTime)
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
	if player.actionMotion != nil && player.actionMotion.MotionSource == "owned_locomotion" {
		r.logLeapDebugStateLocked("defense_preserved_owned_locomotion", player, map[string]string{
			"command_sequence": strconv.FormatUint(cmd.GetSequence(), 10),
			"defense_state":    player.combatState,
		})
		return
	}
	r.normalizePlayerGroundedRootLocked(player, r.contracts.contractForAbility("move"), "defense")
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
		if entity.entityType == "player" {
			r.regeneratePlayerStaminaLocked(entity)
		}
		r.refreshCompletedActionHandoffLocked(entity, now)
		r.advanceActionMotionLocked(entity, now)
		r.refreshCreatureActionTransitionLocked(entity, now)
		if entity.actionInstance == nil || entity.skillRuntime == nil {
			continue
		}
		phase := entity.actionInstance.PhaseAt(now)
		entity.skillRuntime.State = string(phase)
		entity.skillRuntime.LastResolvedAtMs = now.UnixMilli()
		if phase == actionruntime.PhaseComplete {
			if entity.entityType == "creature" {
				r.completeCreatureActionRuntimeLocked(entity, now)
				continue
			}
			entity.skillState = "idle"
			entity.combatState = "ready"
			entity.actionMotion = nil
			continue
		}
		entity.skillState = string(phase)
	}
}

func (r *Runtime) spendPlayerDodgeStaminaLocked(player *entityState) (bool, string, string) {
	if player == nil {
		return false, "player_not_found", "player not found"
	}
	profile := r.contracts.combatCoreProfileForEntity(player)
	if profile == nil {
		return false, "missing_combat_core_profile", "player combat core profile is missing"
	}
	r.syncPlayerStaminaProfileLocked(player, profile)
	cost := profile.GetDodgeStaminaCost()
	if cost <= 0 {
		return true, "", ""
	}
	if !r.canSpendPlayerStaminaLocked(player) {
		return false, "stamina_recovery_locked", "stamina is exhausted and must fully recover before spending"
	}
	if player.stamina+1e-6 < cost {
		return false, "insufficient_stamina", "not enough stamina for dodge"
	}
	r.spendPlayerStaminaLocked(player, cost)
	return true, "", ""
}

func (r *Runtime) spendPlayerSprintStaminaLocked(player *entityState) bool {
	if player == nil {
		return false
	}
	profile := r.contracts.combatCoreProfileForEntity(player)
	if profile == nil {
		return false
	}
	r.syncPlayerStaminaProfileLocked(player, profile)
	costPerSecond := profile.GetSprintStaminaCostPerSec()
	if costPerSecond <= 0 {
		return true
	}
	if !r.canSpendPlayerStaminaLocked(player) {
		return false
	}
	cost := costPerSecond / tickRate
	if player.stamina+1e-6 < cost {
		r.markPlayerStaminaExhaustedLocked(player)
		return false
	}
	r.spendPlayerStaminaLocked(player, cost)
	return true
}

func (r *Runtime) canSpendPlayerStaminaLocked(player *entityState) bool {
	if player == nil {
		return false
	}
	if !player.staminaSpendLockedUntilFull {
		return true
	}
	return player.maxStamina > 0 && player.stamina+1e-6 >= player.maxStamina
}

func (r *Runtime) spendPlayerStaminaLocked(player *entityState, cost float64) {
	if player == nil || cost <= 0 {
		return
	}
	player.stamina = math.Max(0, player.stamina-cost)
	if player.stamina <= 1e-6 {
		r.markPlayerStaminaExhaustedLocked(player)
	}
}

func (r *Runtime) markPlayerStaminaExhaustedLocked(player *entityState) {
	if player == nil {
		return
	}
	player.stamina = 0
	player.staminaSpendLockedUntilFull = true
}

func (r *Runtime) regeneratePlayerStaminaLocked(player *entityState) {
	if player == nil {
		return
	}
	profile := r.contracts.combatCoreProfileForEntity(player)
	if profile == nil {
		return
	}
	r.syncPlayerStaminaProfileLocked(player, profile)
	regen := profile.GetStaminaRegenPerSec()
	if regen <= 0 || player.maxStamina <= 0 || player.stamina >= player.maxStamina {
		return
	}
	if player.staminaSpendLockedUntilFull {
		multiplier := profile.GetStaminaZeroRegenMultiplier()
		if multiplier <= 0 {
			return
		}
		regen *= multiplier
	}
	player.stamina = math.Min(player.maxStamina, player.stamina+(regen/tickRate))
	if player.stamina+1e-6 >= player.maxStamina {
		player.stamina = player.maxStamina
		player.staminaSpendLockedUntilFull = false
	}
}

func (r *Runtime) syncPlayerStaminaProfileLocked(player *entityState, profile *dbv1.CombatCoreProfile) {
	if player == nil || profile == nil {
		return
	}
	if profile.GetMaxStamina() > 0 {
		player.maxStamina = profile.GetMaxStamina()
		if player.stamina > player.maxStamina {
			player.stamina = player.maxStamina
		}
		if player.stamina+1e-6 >= player.maxStamina {
			player.staminaSpendLockedUntilFull = false
		}
	}
}

func (r *Runtime) refreshCompletedActionHandoffLocked(entity *entityState, now time.Time) {
	if entity == nil || entity.actionHandoffUntil.IsZero() || now.Before(entity.actionHandoffUntil) {
		return
	}
	action := entity.actionHandoffAction
	entity.actionHandoffUntil = time.Time{}
	entity.actionHandoffAction = ""
	if entity.locomotion == nil || action == "" || !strings.EqualFold(entity.locomotion.GetAction(), action) {
		return
	}
	entity.locomotion.Phase = "complete"
	entity.locomotion.MovementMode = "grounded"
	entity.locomotion.PhaseElapsedMs = 0
	entity.locomotion.PhaseRemainingMs = 0
	entity.locomotion.TargetSpeed = 0
	entity.locomotion.EffectiveSpeed = 0
	entity.locomotion.LandingHandoffActive = false
	entity.locomotion.LandingExitDirection = nil
	entity.locomotion.LandingExitSpeed = 0
	entity.locomotion.LastUpdatedTick = r.tick
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
		UseVerticalRoot:    motion.UseVerticalRoot,
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
	applyActionMotionLocomotionTiming(entity.locomotion, motion, now)
	entity.locomotion.ActionDistanceTraveled = distanceCM
	entity.locomotion.ActionProjectedPosition = toProto(entity.position)
	entity.locomotion.ClientActionSequence = motion.Sequence
	entity.locomotion.ServerReceivedTick = r.tick
	entity.locomotion.LastUpdatedTick = r.tick
	applyActionInstanceLocomotionTiming(entity.locomotion, entity.actionInstance, now)
	r.logLeapDebugStateLocked("owned_locomotion_progress", entity, map[string]string{
		"phase":       entity.locomotion.GetPhase(),
		"distance_cm": strconv.FormatFloat(distanceCM, 'f', 1, 64),
		"complete":    strconv.FormatBool(progress.Complete),
		"projected_z": strconv.FormatFloat(entity.position.z, 'f', 1, 64),
		"velocity_z":  strconv.FormatFloat(entity.velocity.z, 'f', 1, 64),
		"elapsed_ms":  strconv.FormatInt(progress.Elapsed.Milliseconds(), 10),
		"duration_ms": strconv.FormatInt(progress.Duration.Milliseconds(), 10),
	})

	if progress.Complete || contact.Stopped {
		finalVelocity := velocity
		finalDistanceCM := distanceCM
		if progress.Complete && !contact.Stopped {
			entity.position = motion.ProjectedPosition
		}
		if entity.entityType == "creature" && motion.UseVerticalRoot {
			entity.position = r.entityGroundRootPosition(entity, entity.position)
			velocity.z = 0
			finalVelocity.z = 0
		}
		entity.velocity = vector{}
		entity.locomotion.ActionProjectedPosition = toProto(entity.position)
		if progress.Complete && !contact.Stopped {
			entity.locomotion.ActionDistanceTraveled = progress.TotalDistanceCM
		} else {
			entity.locomotion.ActionDistanceTraveled = distanceCM
		}
		creatureTransitionStarted := r.beginCreatureActionTransitionLocked(entity, motion, now, entity.position, finalVelocity, finalDistanceCM)
		if !creatureTransitionStarted {
			r.applyOwnedLocomotionExitHandoffLocked(entity, motion, now)
		}
		r.completeActionMotionLocked(entity, motion)
		entity.actionMotion = nil
		return false
	}
	return true
}

func applyActionMotionLocomotionTiming(state *gamev1.LocomotionState, motion *actionMotionState, now time.Time) {
	if state == nil || motion == nil || motion.StartedAt.IsZero() {
		return
	}
	if motion.StartedTick > 0 {
		state.ServerActionStartedTick = motion.StartedTick
		state.ActionStartedTick = motion.StartedTick
	}
	elapsed := now.Sub(motion.StartedAt)
	if elapsed < 0 {
		elapsed = 0
	}
	phase, elapsedMS, remainingMS := locomotionResolver.ResolvePhase(elapsed, 0, motion.Contract.ActiveMS, motion.Contract.RecoveryMS)
	state.Phase = phase
	state.PhaseElapsedMs = elapsedMS
	state.PhaseRemainingMs = remainingMS
}

func (r *Runtime) applyOwnedLocomotionExitHandoffLocked(entity *entityState, motion *actionMotionState, now time.Time) {
	if entity == nil || motion == nil || entity.locomotion == nil || motion.MotionSource != "owned_locomotion" {
		return
	}
	if motion.Contract.ActionType != "dodge" {
		entity.locomotion.Phase = "complete"
		entity.locomotion.PhaseElapsedMs = 0
		entity.locomotion.PhaseRemainingMs = 0
		entity.locomotion.LandingHandoffActive = false
		entity.locomotion.LandingExitDirection = nil
		entity.locomotion.LandingExitSpeed = 0
		return
	}
	handoffMS := int32(0)
	if r.contracts.MovementProfile != nil {
		handoffMS = r.contracts.MovementProfile.GetDodgeCarryHandoffMs()
	}
	if handoffMS <= 0 {
		entity.locomotion.Phase = "complete"
		entity.locomotion.PhaseElapsedMs = 0
		entity.locomotion.PhaseRemainingMs = 0
		entity.locomotion.LandingHandoffActive = false
		entity.locomotion.LandingExitDirection = nil
		entity.locomotion.LandingExitSpeed = 0
		return
	}
	entity.locomotion.MovementMode = "grounded_handoff"
	entity.locomotion.Phase = "exit_handoff"
	entity.locomotion.PhaseElapsedMs = 0
	entity.locomotion.PhaseRemainingMs = handoffMS
	entity.locomotion.LandingHandoffActive = true
	entity.locomotion.LandingExitDirection = toProto(motion.Direction)
	entity.locomotion.LandingExitSpeed = 0
	entity.actionHandoffUntil = now.Add(time.Duration(handoffMS) * time.Millisecond)
	entity.actionHandoffAction = "dodge"
}

func (r *Runtime) completeActionMotionLocked(entity *entityState, motion *actionMotionState) {
	if entity == nil || motion == nil {
		return
	}
	switch motion.MotionSource {
	case "impact_control":
		entity.movementState = "grounded"
		if entity.entityType == "creature" {
			entity.skillState = "idle"
			entity.combatState = "ready"
		} else {
			entity.skillState = "idle"
			entity.combatState = "ready"
		}
		entity.skillRuntime = nil
		if entity.locomotion != nil {
			entity.locomotion.MovementMode = "grounded"
			entity.locomotion.Action = "post_impact_control"
			entity.locomotion.Phase = "complete"
			entity.locomotion.TargetSpeed = 0
			entity.locomotion.EffectiveSpeed = 0
			entity.locomotion.ActionProjectedPosition = toProto(entity.position)
			entity.locomotion.ActionDistanceTraveled = distance(entity.position, motion.StartPosition)
			entity.locomotion.LastUpdatedTick = r.tick
		}
	case "owned_locomotion":
		r.logDodgeDebugStateLocked("owned_locomotion_complete_before_clear", entity, map[string]string{
			"completed_action":  motion.Contract.ActionType,
			"completed_ability": motion.Contract.AbilityKey,
		})
		r.logLeapDebugStateLocked("owned_locomotion_complete_before_clear", entity, map[string]string{
			"completed_action":  motion.Contract.ActionType,
			"completed_ability": motion.Contract.AbilityKey,
		})
		if strings.EqualFold(motion.Contract.ActionType, "leap") {
			entity.position = r.playerGroundRootPosition(entity.position, motion.Contract)
			entity.velocity.z = 0
			if entity.locomotion != nil {
				entity.locomotion.ActionProjectedPosition = toProto(entity.position)
				entity.locomotion.LandingHandoffActive = false
			}
		}
		entity.movementState = "grounded"
		entity.skillState = "idle"
		entity.combatState = "ready"
		entity.skillRuntime = nil
		entity.actionInstance = nil
		entity.actionLockedUntil = time.Time{}
		entity.actionLockReason = ""
		r.logDodgeDebugStateLocked("owned_locomotion_complete_after_clear", entity, map[string]string{
			"completed_action":  motion.Contract.ActionType,
			"completed_ability": motion.Contract.AbilityKey,
		})
		r.logLeapDebugStateLocked("owned_locomotion_complete_after_clear", entity, map[string]string{
			"completed_action":  motion.Contract.ActionType,
			"completed_ability": motion.Contract.AbilityKey,
		})
	case "skill_root":
		if entity.entityType == "creature" && creatureActionTransitionActive(entity) {
			entity.actionInstance = nil
			entity.creatureActiveSetupPolicyID = ""
			entity.skillState = "recovery"
			entity.combatState = "committed"
			return
		}
		if entity.entityType == "creature" {
			entity.movementState = "grounded"
			entity.skillState = "idle"
			entity.combatState = "ready"
			entity.skillRuntime = nil
			entity.actionInstance = nil
		}
	}
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
		"movement_protocol": "apeiron_game_v1",
	}
	if player != nil {
		if player.maxStamina > 0 {
			metadata["current_stamina"] = strconv.FormatFloat(player.stamina, 'f', 3, 64)
			metadata["max_stamina"] = strconv.FormatFloat(player.maxStamina, 'f', 3, 64)
		}
		if accepted && cmd.GetType() == gamev1.CommandType_COMMAND_TYPE_DODGE {
			if profile := r.contracts.combatCoreProfileForEntity(player); profile != nil && profile.GetDodgeStaminaCost() > 0 {
				metadata["stamina_delta"] = strconv.FormatFloat(-profile.GetDodgeStaminaCost(), 'f', 3, 64)
				metadata["dodge_stamina_cost"] = strconv.FormatFloat(profile.GetDodgeStaminaCost(), 'f', 3, 64)
			}
		}
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
		if skillContract := r.contracts.skillContract(skillID); skillContract.CooldownMS > 0 {
			metadata["skill_cooldown_ms"] = strconv.FormatInt(int64(skillContract.CooldownMS), 10)
			if player != nil {
				if cooldownUntil, onCooldown := r.playerSkillCooldownLocked(player, skillID, time.Now()); onCooldown {
					metadata["cooldown_until_ms"] = strconv.FormatInt(cooldownUntil.UnixMilli(), 10)
					metadata["cooldown_remaining_ms"] = strconv.FormatInt(cooldownUntil.Sub(time.Now()).Milliseconds(), 10)
				}
			}
		}
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
		SkillRuntimeState:            cloneSkillRuntimeState(e.skillRuntime),
		AggroState:                   e.aggroState,
		Aggression:                   e.aggression,
		LastProcessedCommandSequence: e.lastSequence,
		LastProcessedClientTick:      e.lastClientTick,
		Locomotion:                   cloneLocomotionState(e.locomotion),
		MovementReconciliation:       cloneMovementReconciliationProfile(contracts.MovementProfile),
		CreatureAiState:              cloneCreatureAIState(e.creatureAI),
		CombatModeState:              cloneCombatModeState(e.combatMode),
		PlayerProgression:            playerProgressionSnapshot(e),
	}
}

func cloneSkillRuntimeState(state *gamev1.SkillRuntimeState) *gamev1.SkillRuntimeState {
	if state == nil {
		return nil
	}
	return proto.Clone(state).(*gamev1.SkillRuntimeState)
}

func cloneLocomotionState(state *gamev1.LocomotionState) *gamev1.LocomotionState {
	if state == nil {
		return nil
	}
	return proto.Clone(state).(*gamev1.LocomotionState)
}

func cloneMovementReconciliationProfile(profile *gamev1.MovementReconciliationProfile) *gamev1.MovementReconciliationProfile {
	if profile == nil {
		return nil
	}
	return proto.Clone(profile).(*gamev1.MovementReconciliationProfile)
}

func cloneCreatureAIState(state *gamev1.CreatureAIState) *gamev1.CreatureAIState {
	if state == nil {
		return nil
	}
	return proto.Clone(state).(*gamev1.CreatureAIState)
}

func cloneCombatModeState(state *gamev1.CombatModeState) *gamev1.CombatModeState {
	if state == nil {
		return nil
	}
	return proto.Clone(state).(*gamev1.CombatModeState)
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
	if player == nil || player.skillRuntime == nil {
		return "player_basic_attack_1"
	}
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
