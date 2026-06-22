package gameapi

import (
	"context"
	"math"

	dbv1 "db-apeiron/gen/apeiron/v1"
	gamev1 "server-apeiron/gen/apeiron/game/v1"
	"server-apeiron/internal/movement"

	"google.golang.org/grpc"
)

type ContractSource interface {
	GetSkillActionTiming(context.Context, *dbv1.IdRequest, ...grpc.CallOption) (*dbv1.SkillActionTimingResponse, error)
	GetSkillMovementActionBinding(context.Context, *dbv1.IdRequest, ...grpc.CallOption) (*dbv1.SkillMovementActionBindingResponse, error)
}

type ProfileContractSource interface {
	GetMovementActionContract(context.Context, *dbv1.IdRequest, ...grpc.CallOption) (*dbv1.MovementActionContractResponse, error)
	GetMovementReconciliationContract(context.Context, *dbv1.IdRequest, ...grpc.CallOption) (*dbv1.MovementReconciliationContractResponse, error)
	GetCreatureBehaviorRuntimeContract(context.Context, *dbv1.IdRequest, ...grpc.CallOption) (*dbv1.CreatureBehaviorRuntimeContractResponse, error)
	GetCreatureEvasionPolicies(context.Context, *dbv1.IdRequest, ...grpc.CallOption) (*dbv1.CreatureEvasionPoliciesResponse, error)
	GetCreatureSkillSetupPolicies(context.Context, *dbv1.IdRequest, ...grpc.CallOption) (*dbv1.CreatureSkillSetupPoliciesResponse, error)
}

type RuntimeContracts struct {
	Source string

	MovementProfile *gamev1.MovementReconciliationProfile
	ActionContracts map[string]MovementActionRuntimeContract
	SkillContracts  map[string]SkillRuntimeContract
	WolfPolicy      WolfRuntimePolicy
	CombatModes     []*gamev1.CombatModeSlot
}

type MovementActionRuntimeContract = movement.RuntimeActionContract

type SkillRuntimeContract struct {
	SkillID                  string
	MovementActionContractID string
	MovementAction           MovementActionRuntimeContract
	WindupMS                 int32
	ActiveMS                 int32
	RecoveryMS               int32
	CooldownMS               int32
	ComboWindowMS            int32
	MovementLockPolicy       string
	QueuePolicy              string
	CancelPolicy             string
	StartsAtPhase            string
	HandoffPolicy            string
	NormalInputPolicy        string
	TargetPolicy             string
	ContactPolicy            string
	Enabled                  bool
}

type WolfRuntimePolicy struct {
	ContractID        string
	ContractHash      string
	CapabilityID      string
	DesiredRangeCM    float64
	ChaseRangeCM      float64
	LungeRangeCM      float64
	RetreatRangeCM    float64
	OrbitSpeedCMS     float64
	ChaseSpeedCMS     float64
	LungeSpeedCMS     float64
	MaulSpeedCMS      float64
	RetreatSpeedCMS   float64
	LungeWindupMS     int32
	LungeActiveEndMS  int32
	LungeRecoveryMS   int32
	LungeDistanceCM   float64
	LungeDurationMS   int32
	LungeArcHeightCM  float64
	DodgeSkillID      string
	EvasionChainCount int32
}

func LoadRuntimeContractsFromDB(ctx context.Context, skills ContractSource, profiles ProfileContractSource) RuntimeContracts {
	contracts := RecoveredRuntimeContracts()
	contracts.Source = "db_contracts_with_recovered_fallback"

	for _, ability := range []struct {
		abilityKey string
		contractID string
	}{
		{"move", "grounded_move_v1"},
		{"turn", "turn_v1_rate_limited_contextual"},
		{"dodge", "dodge_v1_full_iframe"},
		{"jump", "jump_v1_authoritative_grounded_handoff"},
	} {
		if resp, err := profiles.GetMovementActionContract(ctx, &dbv1.IdRequest{Id: ability.contractID}); err == nil && resp.GetFound() {
			contract := runtimeContractFromDB(resp.GetContract(), ability.abilityKey)
			if contract.ID != "" {
				contracts.ActionContracts[ability.abilityKey] = contract
			}
		}
	}

	for _, skillID := range []string{
		"player_basic_attack_1",
		"player_basic_attack_2",
		"player_basic_attack_3",
		"player_shield_bash",
		"player_shield_rush",
		"bite",
		"lunge",
		"wolf_dodge",
		"maul",
	} {
		loaded, ok := loadSkillRuntimeContract(ctx, skills, skillID)
		if !ok {
			continue
		}
		contracts.SkillContracts[skillID] = loaded
		contracts.ActionContracts[skillID] = loaded.MovementAction
	}

	if setupResp, err := profiles.GetCreatureSkillSetupPolicies(ctx, &dbv1.IdRequest{Id: contracts.WolfPolicy.ContractID}); err == nil && setupResp.GetFound() {
		for _, setup := range setupResp.GetPolicies() {
			if setup.GetSkillId() != "lunge" || !setup.GetIsEnabled() {
				continue
			}
			contracts.WolfPolicy.LungeRangeCM = setup.GetPreferredMinRangeCm()
			contracts.WolfPolicy.ChaseRangeCM = setup.GetPreferredMaxRangeCm()
			if setup.GetMaxSetupMs() > 0 {
				contracts.WolfPolicy.LungeWindupMS = setup.GetMaxSetupMs()
			}
		}
	}
	if evasionResp, err := profiles.GetCreatureEvasionPolicies(ctx, &dbv1.IdRequest{Id: contracts.WolfPolicy.ContractID}); err == nil && evasionResp.GetFound() {
		for _, evasion := range evasionResp.GetPolicies() {
			if evasion.GetDodgeSkillId() == "" {
				continue
			}
			contracts.WolfPolicy.DodgeSkillID = evasion.GetDodgeSkillId()
			contracts.WolfPolicy.EvasionChainCount = evasion.GetMaxChainCount()
			break
		}
	}
	return contracts
}

func loadSkillRuntimeContract(ctx context.Context, skills ContractSource, skillID string) (SkillRuntimeContract, bool) {
	timingResp, timingErr := skills.GetSkillActionTiming(ctx, &dbv1.IdRequest{Id: skillID})
	bindingResp, bindingErr := skills.GetSkillMovementActionBinding(ctx, &dbv1.IdRequest{Id: skillID})
	if timingErr != nil || bindingErr != nil || !timingResp.GetFound() || !bindingResp.GetFound() {
		return SkillRuntimeContract{}, false
	}

	timing := timingResp.GetContract()
	binding := bindingResp.GetBinding()
	action := runtimeContractFromDB(binding.GetMovementActionContract(), skillID)
	if action.ID == "" {
		return SkillRuntimeContract{}, false
	}
	return SkillRuntimeContract{
		SkillID:                  skillID,
		MovementActionContractID: binding.GetMovementActionContractId(),
		MovementAction:           action,
		WindupMS:                 timing.GetWindupMs(),
		ActiveMS:                 timing.GetActiveMs(),
		RecoveryMS:               timing.GetRecoveryMs(),
		CooldownMS:               timing.GetCooldownMs(),
		ComboWindowMS:            timing.GetComboWindowMs(),
		MovementLockPolicy:       timing.GetMovementLockPolicy(),
		QueuePolicy:              timing.GetQueuePolicy(),
		CancelPolicy:             timing.GetCancelPolicy(),
		StartsAtPhase:            binding.GetStartsAtPhase(),
		HandoffPolicy:            binding.GetHandoffPolicy(),
		NormalInputPolicy:        binding.GetNormalInputPolicy(),
		TargetPolicy:             binding.GetTargetPolicy(),
		ContactPolicy:            binding.GetContactPolicy(),
		Enabled:                  binding.GetIsEnabled(),
	}, true
}

func RecoveredRuntimeContracts() RuntimeContracts {
	contracts := RuntimeContracts{
		Source:          "recovered_runtime_fallback",
		MovementProfile: recoveredMovementProfile(),
		ActionContracts: map[string]MovementActionRuntimeContract{},
		SkillContracts:  map[string]SkillRuntimeContract{},
		WolfPolicy: WolfRuntimePolicy{
			ContractID:        "contract_wolf_pack_harasser_v1",
			ContractHash:      "contract_wolf_pack_harasser_v1",
			CapabilityID:      "wolf_pack_harasser",
			DesiredRangeCM:    420,
			ChaseRangeCM:      760,
			LungeRangeCM:      220,
			RetreatRangeCM:    130,
			OrbitSpeedCMS:     360,
			ChaseSpeedCMS:     620,
			LungeSpeedCMS:     760,
			MaulSpeedCMS:      420,
			RetreatSpeedCMS:   520,
			LungeWindupMS:     3600,
			LungeActiveEndMS:  4030,
			LungeRecoveryMS:   500,
			LungeDistanceCM:   620,
			LungeDurationMS:   980,
			LungeArcHeightCM:  120,
			DodgeSkillID:      "wolf_dodge",
			EvasionChainCount: 4,
		},
		CombatModes: recoveredCombatModeSlots(),
	}
	for _, contract := range []MovementActionRuntimeContract{
		{ID: "move_contract", AbilityKey: "move", ActionType: "move", DurationMS: 180, ActiveMS: 120, RecoveryMS: 60, ReconciliationContractID: "grounded_move_reconciliation", ReconciliationCategory: "grounded_move_reconciliation", PhaseWindowPolicy: "server_authoritative", PredictionErrorPolicy: "bounded_smooth_correction"},
		{ID: "turn_contract", AbilityKey: "turn", ActionType: "turn", DurationMS: 180, ActiveMS: 120, RecoveryMS: 60, ReconciliationContractID: "turn_reconciliation", ReconciliationCategory: "turn_reconciliation", PhaseWindowPolicy: "server_authoritative", PredictionErrorPolicy: "bounded_smooth_correction"},
		{ID: "dodge_contract", AbilityKey: "dodge", ActionType: "dodge", DurationMS: 320, ActiveMS: 260, RecoveryMS: 60, DistanceCM: 260, ReconciliationContractID: "dodge_reconciliation", ReconciliationCategory: "dodge_reconciliation", PhaseWindowPolicy: "server_authoritative", PredictionErrorPolicy: "bounded_smooth_correction"},
		{ID: "jump_contract", AbilityKey: "jump", ActionType: "leap", DurationMS: 620, ActiveMS: 560, RecoveryMS: 60, DistanceCM: 280, ReconciliationContractID: "leap_reconciliation", ReconciliationCategory: "leap_reconciliation", PhaseWindowPolicy: "server_authoritative", PredictionErrorPolicy: "bounded_smooth_correction"},
	} {
		contracts.ActionContracts[contract.AbilityKey] = contract
	}
	for _, skill := range []SkillRuntimeContract{
		recoveredSkillContract("player_basic_attack_1", 55, 260, 180, 80),
		recoveredSkillContract("player_basic_attack_2", 35, 260, 180, 80),
		recoveredSkillContract("player_basic_attack_3", 200, 420, 300, 120),
		recoveredSkillContract("player_shield_bash", 130, 360, 260, 100),
		recoveredSkillContract("player_shield_rush", 340, 640, 520, 120),
		recoveredCreatureSkillContract("bite", "wolf_bite_melee_commit_v1", "grounded_skill", "grounded_skill_action_reconciliation", "melee_contact", 0, 520, 220, 180, 120, 180, 900),
		recoveredCreatureSkillContract("lunge", "wolf_lunge_airborne_v1", "leap", "leap_reconciliation", "airborne_passthrough", 620, 980, 430, 260, 3600, 500, 4200),
		recoveredCreatureSkillContract("wolf_dodge", "wolf_dodge_lateral_leap_v1", "dodge", "dodge_reconciliation", "iframe", 210, 520, 420, 100, 0, 100, 0),
		recoveredCreatureSkillContract("maul", "wolf_maul_lateral_counter_v1", "grounded_skill", "grounded_skill_action_reconciliation", "lateral_counter_contact", 140, 800, 260, 360, 180, 360, 5200),
	} {
		contracts.SkillContracts[skill.SkillID] = skill
		contracts.ActionContracts[skill.SkillID] = skill.MovementAction
	}
	return contracts
}

func recoveredSkillContract(skillID string, distance float64, durationMS, activeMS, recoveryMS int32) SkillRuntimeContract {
	action := MovementActionRuntimeContract{
		ID:                       skillID + "_contract",
		AbilityKey:               skillID,
		ActionType:               "grounded_skill",
		DurationMS:               durationMS,
		ActiveMS:                 activeMS,
		RecoveryMS:               recoveryMS,
		DistanceCM:               distance,
		ReconciliationContractID: "grounded_skill_action_reconciliation",
		ReconciliationCategory:   "grounded_skill_action_reconciliation",
		PhaseWindowPolicy:        "server_authoritative",
		PredictionErrorPolicy:    "bounded_smooth_correction",
		RootMotionOwner:          "skill",
		ContactPolicy:            "authoritative_contact",
	}
	return SkillRuntimeContract{
		SkillID:                  skillID,
		MovementActionContractID: action.ID,
		MovementAction:           action,
		ActiveMS:                 activeMS,
		RecoveryMS:               recoveryMS,
		MovementLockPolicy:       "skill_root_motion_owner",
		QueuePolicy:              "queue_after_recovery",
		CancelPolicy:             "contract_cancel_windows",
		StartsAtPhase:            "active",
		HandoffPolicy:            "explicit_post_action_handoff",
		NormalInputPolicy:        "buffer_until_recovery_handoff",
		TargetPolicy:             "aim_direction",
		ContactPolicy:            action.ContactPolicy,
		Enabled:                  true,
	}
}

func recoveredCreatureSkillContract(skillID string, contractID string, actionType string, reconciliation string, contactPolicy string, distance float64, durationMS, activeMS, recoveryMS, windupMS, skillRecoveryMS, cooldownMS int32) SkillRuntimeContract {
	action := MovementActionRuntimeContract{
		ID:                       contractID,
		AbilityKey:               skillID,
		ActionType:               actionType,
		DurationMS:               durationMS,
		ActiveMS:                 activeMS,
		RecoveryMS:               recoveryMS,
		DistanceCM:               distance,
		ReconciliationContractID: reconciliation,
		ReconciliationCategory:   reconciliation,
		PhaseWindowPolicy:        actionType,
		PredictionErrorPolicy:    "bounded_smooth_correction",
		RootMotionOwner:          "movement",
		ContactPolicy:            contactPolicy,
	}
	return SkillRuntimeContract{
		SkillID:                  skillID,
		MovementActionContractID: action.ID,
		MovementAction:           action,
		WindupMS:                 windupMS,
		ActiveMS:                 activeMS,
		RecoveryMS:               skillRecoveryMS,
		CooldownMS:               cooldownMS,
		MovementLockPolicy:       "contract",
		QueuePolicy:              "none",
		CancelPolicy:             "none",
		StartsAtPhase:            "active",
		HandoffPolicy:            "explicit_recovery_handoff",
		NormalInputPolicy:        "blocked_during_owned_root",
		TargetPolicy:             "target_direction",
		ContactPolicy:            contactPolicy,
		Enabled:                  true,
	}
}

func recoveredCombatModeSlots() []*gamev1.CombatModeSlot {
	return []*gamev1.CombatModeSlot{
		{CombatModeId: "mode_sword_shield_vanguard", SlotIndex: 1, SkillId: "player_basic_attack_1", Enabled: true},
		{CombatModeId: "mode_sword_shield_bulwark", SlotIndex: 1, SkillId: "player_basic_attack_1", Enabled: true},
		{CombatModeId: "mode_sword_shield_bulwark", SlotIndex: 3, SkillId: "player_shield_bash", Enabled: true},
		{CombatModeId: "mode_sword_shield_bulwark", SlotIndex: 4, SkillId: "player_shield_rush", Enabled: true},
	}
}

func (c RuntimeContracts) contractForAbility(ability string) MovementActionRuntimeContract {
	registry := movement.NewActionContractRegistry(c.ActionContracts)
	if contract, ok := registry.Resolve(ability); ok {
		return contract
	}
	return MovementActionRuntimeContract{
		ID:                       ability + "_contract",
		AbilityKey:               ability,
		ActionType:               ability,
		DurationMS:               180,
		ActiveMS:                 120,
		RecoveryMS:               60,
		ReconciliationContractID: "grounded_move_reconciliation",
		ReconciliationCategory:   "grounded_move_reconciliation",
		PhaseWindowPolicy:        "server_authoritative",
		PredictionErrorPolicy:    "bounded_smooth_correction",
	}
}

func (c RuntimeContracts) skillContract(skillID string) SkillRuntimeContract {
	if contract, ok := c.SkillContracts[skillID]; ok {
		return contract
	}
	return recoveredSkillContract(skillID, 0, 180, 120, 60)
}

func (c RuntimeContracts) movementContractManifest() []*gamev1.MovementActionContractManifest {
	keys := c.orderedActionKeys()
	out := make([]*gamev1.MovementActionContractManifest, 0, len(keys))
	priority := int32(1)
	for _, key := range keys {
		contract := c.ActionContracts[key]
		out = append(out, &gamev1.MovementActionContractManifest{
			ContractId:      contract.ID,
			AbilityKey:      contract.AbilityKey,
			MovementType:    contract.ActionType,
			ActionFamily:    actionFamily(contract),
			ContractVersion: "movement_action_v1",
			ContractHash:    contractHash(contract),
			ActionPriority:  priority,
			Enabled:         true,
		})
		priority++
	}
	return out
}

func (c RuntimeContracts) movementContractPayloads() []*gamev1.LocomotionState {
	keys := c.orderedActionKeys()
	out := make([]*gamev1.LocomotionState, 0, len(keys))
	for _, key := range keys {
		contract := c.contractForAbility(key)
		state := locomotionFromContract(contract, "contract", vector{}, vector{}, 0, 0)
		out = append(out, state)
	}
	return out
}

func (c RuntimeContracts) orderedActionKeys() []string {
	preferred := []string{
		"move",
		"turn",
		"dodge",
		"jump",
		"player_basic_attack_1",
		"player_basic_attack_2",
		"player_basic_attack_3",
		"player_shield_bash",
		"player_shield_rush",
		"bite",
		"lunge",
		"wolf_dodge",
		"maul",
	}
	return movement.NewActionContractRegistry(c.ActionContracts).OrderedKeys(preferred)
}

func actionFamily(contract MovementActionRuntimeContract) string {
	return movement.ActionFamily(contract)
}

func contractHash(contract MovementActionRuntimeContract) string {
	return movement.ContractHash(contract)
}

func runtimeContractFromDB(contract *dbv1.MovementActionContract, abilityKey string) MovementActionRuntimeContract {
	if contract == nil {
		return MovementActionRuntimeContract{}
	}
	category := contract.GetReconciliationContract().GetCategory()
	if category == "" {
		category = contract.GetReconciliationContractId()
	}
	return MovementActionRuntimeContract{
		ID:                       contract.GetId(),
		AbilityKey:               abilityKey,
		ActionType:               contract.GetActionType(),
		DurationMS:               contract.GetDurationMs(),
		ActiveMS:                 contract.GetActiveMs(),
		RecoveryMS:               contract.GetRecoveryMs(),
		DistanceCM:               contract.GetDistanceCm(),
		BaseSpeedCMS:             contract.GetBaseSpeedCmS(),
		ReconciliationContractID: contract.GetReconciliationContractId(),
		ReconciliationCategory:   category,
		PhaseWindowPolicy:        contract.GetPhaseWindowPolicy(),
		PredictionErrorPolicy:    contract.GetPredictionErrorPolicy(),
		RootMotionOwner:          contract.GetRootMotionOwner(),
		ContactPolicy:            contract.GetContactPolicy(),
	}
}

func recoveredMovementProfile() *gamev1.MovementReconciliationProfile {
	return &gamev1.MovementReconciliationProfile{
		ProfileId:                         "recovered_default_movement_profile",
		MaxSpeed:                          470,
		SprintSpeedMultiplier:             1.45,
		Acceleration:                      2048,
		Deceleration:                      2048,
		GroundFriction:                    8,
		AirAcceleration:                   768,
		JumpHeight:                        180,
		JumpDurationMs:                    620,
		RotationRateYaw:                   720,
		GravityScale:                      1,
		BrakingFrictionFactor:             2,
		MaxSlopeDeg:                       45,
		StepHeight:                        45,
		BaseDeadzone:                      25,
		GroundedSpeedDeadzoneFactor:       0.08,
		GroundedSpeedDeadzoneMin:          35,
		GroundedSpeedDeadzoneMax:          90,
		MoveSustainDeadzone:               45,
		MoveSustainTransitionDeadzone:     65,
		AirborneDeadzone:                  120,
		LeapRecentDeadzone:                140,
		LeapAirborneSnapshotDeadzone:      165,
		LeapLandingDeadzoneFactor:         0.12,
		LeapLandingDeadzoneMin:            80,
		LeapLandingDeadzoneMax:            180,
		DodgeRecentDeadzone:               90,
		DodgeActiveDeadzone:               90,
		DodgeExitDeadzoneFactor:           0.12,
		DodgeExitDeadzoneMin:              65,
		DodgeExitDeadzoneMax:              180,
		PostActionGroundedDeadzone:        55,
		CorrectionMaxStep:                 80,
		HardSnapDistance:                  1400,
		SevereDesyncDistance:              2200,
		VisualSmoothingMs:                 80,
		VisualSmoothingMaxDistance:        260,
		RemoteVisualInterpolationMs:       100,
		RemoteVisualMaxExtrapolationMs:    100,
		RemoteVisualHardSnapDistance:      600,
		MovementTurnResubmitDotThreshold:  0.92,
		MovementTurnResubmitMinIntervalMs: 33,
		MovementSubmitIntervalMs:          33,
		SnapshotPollIntervalMs:            33,
	}
}

func distanceFromContract(contract MovementActionRuntimeContract, fallback float64) float64 {
	if math.Abs(contract.DistanceCM) > 0 {
		return contract.DistanceCM
	}
	return fallback
}
