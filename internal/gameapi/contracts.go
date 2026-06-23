package gameapi

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	dbv1 "db-apeiron/gen/apeiron/v1"
	gamev1 "server-apeiron/gen/apeiron/game/v1"
	"server-apeiron/internal/movement"

	"google.golang.org/grpc"
)

type ContractSource interface {
	GetSkill(context.Context, *dbv1.IdRequest, ...grpc.CallOption) (*dbv1.SkillResponse, error)
	GetSkillActionTiming(context.Context, *dbv1.IdRequest, ...grpc.CallOption) (*dbv1.SkillActionTimingResponse, error)
	GetSkillMovementActionBinding(context.Context, *dbv1.IdRequest, ...grpc.CallOption) (*dbv1.SkillMovementActionBindingResponse, error)
	GetSkillHitboxProfiles(context.Context, *dbv1.IdRequest, ...grpc.CallOption) (*dbv1.SkillHitboxProfilesResponse, error)
	GetWeaponCombatModeSlots(context.Context, *dbv1.IdRequest, ...grpc.CallOption) (*dbv1.WeaponCombatModeSlotsResponse, error)
}

type ProfileContractSource interface {
	GetMovementActionContract(context.Context, *dbv1.IdRequest, ...grpc.CallOption) (*dbv1.MovementActionContractResponse, error)
	GetMovementReconciliationContract(context.Context, *dbv1.IdRequest, ...grpc.CallOption) (*dbv1.MovementReconciliationContractResponse, error)
	GetRuntimeMovementReconciliationProfile(context.Context, *dbv1.IdRequest, ...grpc.CallOption) (*dbv1.RuntimeMovementReconciliationProfileResponse, error)
	GetCreatureBehaviorRuntimeContract(context.Context, *dbv1.IdRequest, ...grpc.CallOption) (*dbv1.CreatureBehaviorRuntimeContractResponse, error)
	GetCreatureTargetOpportunityPolicy(context.Context, *dbv1.IdRequest, ...grpc.CallOption) (*dbv1.CreatureTargetOpportunityPolicyResponse, error)
	GetCreatureOrbitPolicy(context.Context, *dbv1.IdRequest, ...grpc.CallOption) (*dbv1.CreatureOrbitPolicyResponse, error)
	GetCreatureEvasionPolicies(context.Context, *dbv1.IdRequest, ...grpc.CallOption) (*dbv1.CreatureEvasionPoliciesResponse, error)
	GetCreatureSkillSetupPolicies(context.Context, *dbv1.IdRequest, ...grpc.CallOption) (*dbv1.CreatureSkillSetupPoliciesResponse, error)
	GetCreatureSkillBehaviorBindings(context.Context, *dbv1.IdRequest, ...grpc.CallOption) (*dbv1.CreatureSkillBehaviorBindingsResponse, error)
}

type RuntimeContracts struct {
	Source string

	MovementProfile *gamev1.MovementReconciliationProfile
	ActionContracts map[string]MovementActionRuntimeContract
	SkillContracts  map[string]SkillRuntimeContract
	WolfPolicy      WolfRuntimePolicy
	CombatModes     []*gamev1.CombatModeSlot
	LoadIssues      []string
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
	// Damage / PostureDamage / Range are DB-authoritative: loadSkillRuntimeContract sets
	// them from the DB Skill (GetSkill -> base_damage/posture_damage/max_range). The
	// recovered runtime falls back to the canonical seed values (recoveredPlayerSkillDamage)
	// when the DB is unavailable.
	Damage        float64
	PostureDamage float64
	StaminaCost   float64
	Range         float64
	MaxTargets    int32
	Blockable     bool
	Hitboxes      []*dbv1.SkillHitboxProfile
	Enabled       bool
}

type WolfRuntimePolicy struct {
	ContractID                     string
	ContractHash                   string
	CapabilityID                   string
	DesiredRangeCM                 float64
	ChaseRangeCM                   float64
	LungeRangeCM                   float64
	RetreatRangeCM                 float64
	OrbitSpeedCMS                  float64
	ChaseSpeedCMS                  float64
	LungeSpeedCMS                  float64
	MaulSpeedCMS                   float64
	RetreatSpeedCMS                float64
	LungeWindupMS                  int32
	LungeActiveEndMS               int32
	LungeRecoveryMS                int32
	LungeDistanceCM                float64
	LungeDurationMS                int32
	LungeArcHeightCM               float64
	DodgeSkillID                   string
	EvasionChainCount              int32
	TargetOpportunityPolicyID      string
	CommitAngleMaxDeg              float64
	MinCommitDistanceCM            float64
	MaxCommitDistanceCM            float64
	ApproachMinDistanceCM          float64
	ApproachMaxDistanceCM          float64
	BiteRangeCM                    float64
	LungeMinRangeCM                float64
	LungeMaxRangeCM                float64
	MaulPressureThreshold          float64
	TargetMemoryMS                 int32
	NoReadySkillMemoryPolicy       string
	CandidateCooldownVisibility    bool
	AllowBacksideCommit            bool
	OrbitPolicyID                  string
	OrbitLocomotionMode            string
	OrbitSpeedScale                float64
	MinOrbitDurationMS             int32
	SideSwitchCooldownMS           int32
	AllowSideSwitchWhenTargetFaces bool
	PreferLongSideCommit           bool
	SideFlipChanceMultiplier       float64
	LockSideDuringSetup            bool
	RepeatSkillPenaltyWindowMS     int32
	RepeatSkillPenaltyMultiplier   float64
	DodgeStaminaCostMultiplier     float64
	StaminaRegenPerSecond          float64
	MaxStamina                     float64
	SkillBehaviorBindings          []CreatureSkillBehaviorRuntimeBinding
}

type CreatureSkillBehaviorRuntimeBinding struct {
	ID                  string
	SkillID             string
	TacticalState       string
	DecisionPhase       string
	SetupPolicyID       string
	MinRangeCM          float64
	MaxRangeCM          float64
	Priority            int32
	UsageWeight         float64
	CooldownGroup       string
	RequiresLineOfSight bool
	Enabled             bool
}

type creaturePressurePolicyJSON struct {
	RepeatSkillPenaltyMultiplier float64 `json:"repeatSkillPenaltyMultiplier"`
}

type creatureStaminaPolicyJSON struct {
	Max                 float64 `json:"max"`
	DodgeCostMultiplier float64 `json:"dodgeCostMultiplier"`
	RegenPerSecond      float64 `json:"regenPerSecond"`
}

type movementActionContractMetadata struct {
	AbilityKey             string  `json:"ability_key"`
	AirborneDurationMS     int32   `json:"airborne_duration_ms"`
	JumpZVelocity          float64 `json:"jump_z_velocity"`
	GravityScale           float64 `json:"gravity_scale"`
	ExpectedApexMS         int32   `json:"expected_apex_ms"`
	LandingDetectionPolicy string  `json:"landing_detection_policy"`
	GroundZPolicy          string  `json:"ground_z_policy"`
	CapsuleBaseOffset      float64 `json:"capsule_base_offset"`
	AllowsAirControl       bool    `json:"allows_air_control"`
	AirControlModifier     float64 `json:"air_control_modifier"`
	YawRateDegPerSec       float64 `json:"yaw_rate_deg_per_sec"`
}

const runtimeMovementReconciliationProfileID = "player_default_movement_profile"
const wolfRuntimeContractID = "contract_wolf_pack_harasser_v1"

func requiredBaseMovementActions() []struct {
	abilityKey string
	contractID string
} {
	return []struct {
		abilityKey string
		contractID string
	}{
		{"move", "grounded_move_v1"},
		{"turn", "turn_v1_rate_limited_contextual"},
		{"dodge", "dodge_v1_full_iframe"},
		{"jump", "jump_v1_authoritative_grounded_handoff"},
	}
}

func requiredRuntimeSkillIDs() []string {
	return []string{
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
}

func LoadRuntimeContractsFromDB(ctx context.Context, skills ContractSource, profiles ProfileContractSource) RuntimeContracts {
	contracts := emptyDBRuntimeContracts()

	if resp, err := profiles.GetRuntimeMovementReconciliationProfile(ctx, &dbv1.IdRequest{Id: runtimeMovementReconciliationProfileID}); err == nil && resp.GetFound() {
		contracts.MovementProfile = runtimeMovementReconciliationProfileFromDB(resp.GetProfile())
	} else {
		contracts.MovementProfile = nil
		contracts.LoadIssues = append(contracts.LoadIssues, "missing runtime movement reconciliation profile "+runtimeMovementReconciliationProfileID)
	}

	for _, ability := range requiredBaseMovementActions() {
		if resp, err := profiles.GetMovementActionContract(ctx, &dbv1.IdRequest{Id: ability.contractID}); err == nil && resp.GetFound() {
			contract := runtimeContractFromDB(resp.GetContract(), ability.abilityKey)
			if contract.ID != "" {
				contracts.ActionContracts[ability.abilityKey] = contract
				continue
			}
		}
		contracts.LoadIssues = append(contracts.LoadIssues, fmt.Sprintf("missing movement action %s -> %s", ability.abilityKey, ability.contractID))
	}

	for _, skillID := range requiredRuntimeSkillIDs() {
		loaded, ok := loadSkillRuntimeContract(ctx, skills, skillID)
		if !ok {
			contracts.LoadIssues = append(contracts.LoadIssues, "missing skill runtime "+skillID)
			continue
		}
		contracts.SkillContracts[skillID] = loaded
		contracts.ActionContracts[skillID] = loaded.MovementAction
	}

	loadWolfBrainRuntimeContracts(ctx, profiles, &contracts)

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
	if modeResp, err := skills.GetWeaponCombatModeSlots(ctx, &dbv1.IdRequest{Id: "weaponkit_sword_shield"}); err == nil && modeResp.GetFound() {
		modeSlots := combatModeSlotsFromDB(modeResp.GetSlots())
		if len(modeSlots) > 0 {
			contracts.CombatModes = modeSlots
		}
	} else {
		contracts.LoadIssues = append(contracts.LoadIssues, "missing weapon combat mode slots weaponkit_sword_shield")
	}
	if len(contracts.LoadIssues) == 0 {
		contracts.Source = "db_contracts"
	}
	return contracts
}

func emptyDBRuntimeContracts() RuntimeContracts {
	return RuntimeContracts{
		Source:          "db_contracts_incomplete",
		ActionContracts: map[string]MovementActionRuntimeContract{},
		SkillContracts:  map[string]SkillRuntimeContract{},
		WolfPolicy: WolfRuntimePolicy{
			ContractID: wolfRuntimeContractID,
		},
	}
}

func loadWolfBrainRuntimeContracts(ctx context.Context, profiles ProfileContractSource, contracts *RuntimeContracts) {
	if contracts == nil || profiles == nil || contracts.WolfPolicy.ContractID == "" {
		return
	}

	resp, err := profiles.GetCreatureBehaviorRuntimeContract(ctx, &dbv1.IdRequest{Id: contracts.WolfPolicy.ContractID})
	if err != nil || !resp.GetFound() {
		contracts.LoadIssues = append(contracts.LoadIssues, "missing creature behavior runtime "+contracts.WolfPolicy.ContractID)
		return
	}
	behavior := resp.GetContract()
	if behavior == nil {
		contracts.LoadIssues = append(contracts.LoadIssues, "empty creature behavior runtime "+contracts.WolfPolicy.ContractID)
		return
	}
	contracts.WolfPolicy.ContractID = behavior.GetId()
	contracts.WolfPolicy.ContractHash = behavior.GetId()
	applyWolfBehaviorPolicyJSON(&contracts.WolfPolicy, behavior)

	targetPolicy := behavior.GetTargetOpportunityPolicy()
	targetPolicyID := behavior.GetTargetOpportunityPolicyId()
	if targetPolicy == nil && targetPolicyID != "" {
		if targetResp, err := profiles.GetCreatureTargetOpportunityPolicy(ctx, &dbv1.IdRequest{Id: targetPolicyID}); err == nil && targetResp.GetFound() {
			targetPolicy = targetResp.GetPolicy()
		}
	}
	if targetPolicy != nil {
		applyWolfTargetOpportunityPolicy(&contracts.WolfPolicy, targetPolicy)
	} else if targetPolicyID != "" {
		contracts.LoadIssues = append(contracts.LoadIssues, "missing creature target opportunity policy "+targetPolicyID)
	}

	orbitPolicy := behavior.GetOrbitPolicy()
	orbitPolicyID := behavior.GetOrbitPolicyId()
	if orbitPolicy == nil && orbitPolicyID != "" {
		if orbitResp, err := profiles.GetCreatureOrbitPolicy(ctx, &dbv1.IdRequest{Id: orbitPolicyID}); err == nil && orbitResp.GetFound() {
			orbitPolicy = orbitResp.GetPolicy()
		}
	}
	if orbitPolicy != nil {
		applyWolfOrbitPolicy(&contracts.WolfPolicy, orbitPolicy)
	} else if orbitPolicyID != "" {
		contracts.LoadIssues = append(contracts.LoadIssues, "missing creature orbit policy "+orbitPolicyID)
	}

	if bindingResp, err := profiles.GetCreatureSkillBehaviorBindings(ctx, &dbv1.IdRequest{Id: contracts.WolfPolicy.ContractID}); err == nil && bindingResp.GetFound() {
		contracts.WolfPolicy.SkillBehaviorBindings = creatureSkillBehaviorBindingsFromDB(bindingResp.GetBindings())
		if len(contracts.WolfPolicy.SkillBehaviorBindings) == 0 {
			contracts.LoadIssues = append(contracts.LoadIssues, "empty creature skill behavior bindings "+contracts.WolfPolicy.ContractID)
		}
	} else {
		contracts.LoadIssues = append(contracts.LoadIssues, "missing creature skill behavior bindings "+contracts.WolfPolicy.ContractID)
	}
}

func applyWolfTargetOpportunityPolicy(policy *WolfRuntimePolicy, target *dbv1.CreatureTargetOpportunityPolicy) {
	if policy == nil || target == nil {
		return
	}
	policy.TargetOpportunityPolicyID = target.GetId()
	policy.CommitAngleMaxDeg = target.GetCommitAngleMaxDeg()
	policy.MinCommitDistanceCM = target.GetMinCommitDistanceCm()
	policy.MaxCommitDistanceCM = target.GetMaxCommitDistanceCm()
	policy.ApproachMinDistanceCM = target.GetApproachMinDistanceCm()
	policy.ApproachMaxDistanceCM = target.GetApproachMaxDistanceCm()
	policy.BiteRangeCM = target.GetBiteRangeCm()
	policy.LungeMinRangeCM = target.GetLungeMinRangeCm()
	policy.LungeMaxRangeCM = target.GetLungeMaxRangeCm()
	policy.MaulPressureThreshold = target.GetMaulPressureThreshold()
	policy.TargetMemoryMS = target.GetTargetMemoryMs()
	policy.RepeatSkillPenaltyWindowMS = target.GetTargetMemoryMs()
	policy.NoReadySkillMemoryPolicy = target.GetNoReadySkillMemoryPolicy()
	policy.CandidateCooldownVisibility = target.GetCandidateCooldownVisibility()
	policy.AllowBacksideCommit = target.GetAllowBacksideCommit()
	if policy.LungeMinRangeCM > 0 {
		policy.LungeRangeCM = policy.LungeMinRangeCM
	}
	if policy.ApproachMaxDistanceCM > 0 {
		policy.ChaseRangeCM = policy.ApproachMaxDistanceCM
	}
	if policy.BiteRangeCM > 0 {
		policy.RetreatRangeCM = math.Min(policy.RetreatRangeCM, policy.BiteRangeCM)
	}
}

func applyWolfBehaviorPolicyJSON(policy *WolfRuntimePolicy, behavior *dbv1.CreatureBehaviorRuntimeContract) {
	if policy == nil || behavior == nil {
		return
	}
	if staminaJSON := strings.TrimSpace(behavior.GetStaminaPolicyJson()); staminaJSON != "" {
		var staminaPolicy creatureStaminaPolicyJSON
		if err := json.Unmarshal([]byte(staminaJSON), &staminaPolicy); err == nil {
			policy.MaxStamina = staminaPolicy.Max
			policy.DodgeStaminaCostMultiplier = staminaPolicy.DodgeCostMultiplier
			policy.StaminaRegenPerSecond = staminaPolicy.RegenPerSecond
		}
	}
	if pressureJSON := strings.TrimSpace(behavior.GetPressurePolicyJson()); pressureJSON != "" {
		var pressurePolicy creaturePressurePolicyJSON
		if err := json.Unmarshal([]byte(pressureJSON), &pressurePolicy); err == nil {
			policy.RepeatSkillPenaltyMultiplier = pressurePolicy.RepeatSkillPenaltyMultiplier
		}
	}
}

func applyWolfOrbitPolicy(policy *WolfRuntimePolicy, orbit *dbv1.CreatureOrbitPolicy) {
	if policy == nil || orbit == nil {
		return
	}
	policy.OrbitPolicyID = orbit.GetId()
	policy.OrbitLocomotionMode = orbit.GetOrbitLocomotionMode()
	policy.OrbitSpeedScale = orbit.GetOrbitSpeedScale()
	policy.MinOrbitDurationMS = orbit.GetMinOrbitDurationMs()
	policy.SideSwitchCooldownMS = orbit.GetSideSwitchCooldownMs()
	policy.AllowSideSwitchWhenTargetFaces = orbit.GetAllowSideSwitchWhenTargetFaces()
	policy.PreferLongSideCommit = orbit.GetPreferLongSideCommit()
	policy.SideFlipChanceMultiplier = orbit.GetSideFlipChanceMultiplier()
	policy.LockSideDuringSetup = orbit.GetLockSideDuringSetup()
}

func creatureSkillBehaviorBindingsFromDB(bindings []*dbv1.CreatureSkillBehaviorBinding) []CreatureSkillBehaviorRuntimeBinding {
	runtimeBindings := make([]CreatureSkillBehaviorRuntimeBinding, 0, len(bindings))
	for _, binding := range bindings {
		if binding == nil || !binding.GetIsEnabled() || binding.GetSkillId() == "" {
			continue
		}
		runtimeBindings = append(runtimeBindings, CreatureSkillBehaviorRuntimeBinding{
			ID:                  binding.GetId(),
			SkillID:             binding.GetSkillId(),
			TacticalState:       binding.GetTacticalState(),
			DecisionPhase:       binding.GetDecisionPhase(),
			SetupPolicyID:       binding.GetSetupPolicyId(),
			MinRangeCM:          binding.GetMinRangeCm(),
			MaxRangeCM:          binding.GetMaxRangeCm(),
			Priority:            binding.GetPriority(),
			UsageWeight:         binding.GetUsageWeight(),
			CooldownGroup:       binding.GetCooldownGroup(),
			RequiresLineOfSight: binding.GetRequiresLineOfSight(),
			Enabled:             binding.GetIsEnabled(),
		})
	}
	return runtimeBindings
}

func (c RuntimeContracts) ValidateRequiredCoverage(strictLoadedSource bool) error {
	var missing []string
	if c.MovementProfile == nil {
		missing = append(missing, "movement reconciliation profile")
	} else if strictLoadedSource {
		missing = append(missing, validateRuntimeMovementReconciliationProfile(c.MovementProfile)...)
	}
	for _, ability := range requiredBaseMovementActions() {
		contract, ok := c.ActionContracts[ability.abilityKey]
		if !ok || contract.ID == "" {
			missing = append(missing, fmt.Sprintf("movement action %s", ability.abilityKey))
			continue
		}
		if contract.ReconciliationContractID == "" {
			missing = append(missing, fmt.Sprintf("movement action %s reconciliation", ability.abilityKey))
		}
	}
	for _, skillID := range requiredRuntimeSkillIDs() {
		skill, ok := c.SkillContracts[skillID]
		if !ok || skill.SkillID == "" {
			missing = append(missing, "skill runtime "+skillID)
			continue
		}
		if !skill.Enabled {
			missing = append(missing, "skill runtime disabled "+skillID)
		}
		if skill.MovementActionContractID == "" {
			missing = append(missing, "skill movement binding "+skillID)
		}
		if skill.MovementAction.ID == "" {
			missing = append(missing, "skill movement action "+skillID)
		}
		if skill.MovementAction.ReconciliationContractID == "" {
			missing = append(missing, "skill movement reconciliation "+skillID)
		}
		if action, ok := c.ActionContracts[skillID]; !ok || action.ID == "" {
			missing = append(missing, "skill action manifest "+skillID)
		}
	}
	if c.WolfPolicy.ContractID == "" || c.WolfPolicy.DodgeSkillID == "" {
		missing = append(missing, "wolf runtime policy")
	}
	if strictLoadedSource {
		if c.WolfPolicy.TargetOpportunityPolicyID == "" {
			missing = append(missing, "wolf target opportunity policy")
		}
		if c.WolfPolicy.OrbitPolicyID == "" {
			missing = append(missing, "wolf orbit policy")
		}
		if len(c.WolfPolicy.SkillBehaviorBindings) == 0 {
			missing = append(missing, "wolf skill behavior bindings")
		}
	}
	if len(c.CombatModes) == 0 {
		missing = append(missing, "sword shield combat mode slots")
	}
	if strictLoadedSource && len(c.LoadIssues) > 0 {
		missing = append(missing, c.LoadIssues...)
	}
	if len(missing) > 0 {
		return fmt.Errorf("runtime contract coverage incomplete: %s", strings.Join(missing, "; "))
	}
	return nil
}

func validateRuntimeMovementReconciliationProfile(profile *gamev1.MovementReconciliationProfile) []string {
	var missing []string
	if profile == nil {
		return []string{"runtime movement reconciliation profile"}
	}
	if profile.GetProfileId() == "" {
		missing = append(missing, "runtime movement profile id")
	}
	required := []struct {
		name  string
		value float64
	}{
		{"runtime movement max speed", profile.GetMaxSpeed()},
		{"runtime movement sprint multiplier", profile.GetSprintSpeedMultiplier()},
		{"runtime movement acceleration", profile.GetAcceleration()},
		{"runtime movement deceleration", profile.GetDeceleration()},
		{"runtime movement ground friction", profile.GetGroundFriction()},
		{"runtime movement air acceleration", profile.GetAirAcceleration()},
		{"runtime movement jump height", profile.GetJumpHeight()},
		{"runtime movement jump duration", float64(profile.GetJumpDurationMs())},
		{"runtime movement rotation yaw", profile.GetRotationRateYaw()},
		{"runtime movement gravity scale", profile.GetGravityScale()},
		{"runtime movement braking friction factor", profile.GetBrakingFrictionFactor()},
		{"runtime movement max slope", profile.GetMaxSlopeDeg()},
		{"runtime movement step height", profile.GetStepHeight()},
		{"runtime movement base deadzone", profile.GetBaseDeadzone()},
		{"runtime movement grounded deadzone factor", profile.GetGroundedSpeedDeadzoneFactor()},
		{"runtime movement grounded deadzone min", profile.GetGroundedSpeedDeadzoneMin()},
		{"runtime movement grounded deadzone max", profile.GetGroundedSpeedDeadzoneMax()},
		{"runtime movement grounded transition deadzone", profile.GetGroundedTransitionDeadzoneMin()},
		{"runtime movement sustain deadzone", profile.GetMoveSustainDeadzone()},
		{"runtime movement sustain transition deadzone", profile.GetMoveSustainTransitionDeadzone()},
		{"runtime movement airborne deadzone", profile.GetAirborneDeadzone()},
		{"runtime movement leap recent deadzone", profile.GetLeapRecentDeadzone()},
		{"runtime movement leap airborne snapshot deadzone", profile.GetLeapAirborneSnapshotDeadzone()},
		{"runtime movement leap landing deadzone factor", profile.GetLeapLandingDeadzoneFactor()},
		{"runtime movement leap landing deadzone min", profile.GetLeapLandingDeadzoneMin()},
		{"runtime movement leap landing deadzone max", profile.GetLeapLandingDeadzoneMax()},
		{"runtime movement leap landing clamp ignore deadzone", profile.GetLeapLandingClampIgnoreDeadzone()},
		{"runtime movement leap landing soft snap deadzone", profile.GetLeapLandingSoftSnapDeadzone()},
		{"runtime movement dodge recent deadzone", profile.GetDodgeRecentDeadzone()},
		{"runtime movement dodge active deadzone", profile.GetDodgeActiveDeadzone()},
		{"runtime movement dodge exit deadzone factor", profile.GetDodgeExitDeadzoneFactor()},
		{"runtime movement dodge exit deadzone min", profile.GetDodgeExitDeadzoneMin()},
		{"runtime movement dodge exit deadzone max", profile.GetDodgeExitDeadzoneMax()},
		{"runtime movement post action grounded deadzone", profile.GetPostActionGroundedDeadzone()},
		{"runtime movement correction max step", profile.GetCorrectionMaxStep()},
		{"runtime movement hard snap distance", profile.GetHardSnapDistance()},
		{"runtime movement severe desync distance", profile.GetSevereDesyncDistance()},
		{"runtime movement visual smoothing", float64(profile.GetVisualSmoothingMs())},
		{"runtime movement visual smoothing max distance", profile.GetVisualSmoothingMaxDistance()},
		{"runtime movement remote visual interpolation", float64(profile.GetRemoteVisualInterpolationMs())},
		{"runtime movement remote visual max extrapolation", float64(profile.GetRemoteVisualMaxExtrapolationMs())},
		{"runtime movement remote visual hard snap distance", profile.GetRemoteVisualHardSnapDistance()},
		{"runtime movement dodge carry handoff", float64(profile.GetDodgeCarryHandoffMs())},
		{"runtime movement leap landing correction grace", float64(profile.GetLeapLandingCorrectionGraceMs())},
		{"runtime movement leap grounded carry handoff", float64(profile.GetLeapGroundedCarryHandoffMs())},
		{"runtime movement turn resubmit dot threshold", profile.GetMovementTurnResubmitDotThreshold()},
		{"runtime movement turn resubmit min interval", float64(profile.GetMovementTurnResubmitMinIntervalMs())},
		{"runtime movement submit interval", float64(profile.GetMovementSubmitIntervalMs())},
		{"runtime movement snapshot poll interval", float64(profile.GetSnapshotPollIntervalMs())},
		{"runtime movement strafe multiplier", profile.GetStrafeSpeedMultiplier()},
		{"runtime movement backpedal multiplier", profile.GetBackpedalSpeedMultiplier()},
		{"runtime movement strafe sprint multiplier", profile.GetStrafeSprintSpeedMultiplier()},
		{"runtime movement backpedal sprint multiplier", profile.GetBackpedalSprintSpeedMultiplier()},
	}
	for _, field := range required {
		if field.value <= 0 {
			missing = append(missing, field.name)
		}
	}
	return missing
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
	contract := SkillRuntimeContract{
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
	}

	// DB-authoritative damage/range (brick 2b): enrich from the base Skill. Best-effort —
	// if GetSkill is unavailable the contract still loads via timing/binding.
	if skillResp, err := skills.GetSkill(ctx, &dbv1.IdRequest{Id: skillID}); err == nil && skillResp.GetFound() {
		s := skillResp.GetSkill()
		contract.Damage = s.GetBaseDamage()
		contract.PostureDamage = s.GetPostureDamage()
		contract.StaminaCost = s.GetStaminaCost()
		contract.Range = s.GetMaxRange()
		contract.MaxTargets = s.GetMaxTargets()
		contract.Blockable = s.GetIsBlockable()
	}
	if hitboxResp, err := skills.GetSkillHitboxProfiles(ctx, &dbv1.IdRequest{Id: skillID}); err == nil && hitboxResp.GetFound() {
		contract.Hitboxes = hitboxResp.GetProfiles()
	}
	return contract, true
}

func runtimeMovementReconciliationProfileFromDB(profile *dbv1.RuntimeMovementReconciliationProfile) *gamev1.MovementReconciliationProfile {
	if profile == nil {
		return nil
	}
	return &gamev1.MovementReconciliationProfile{
		ProfileId:                         profile.GetProfileId(),
		MaxSpeed:                          profile.GetMaxSpeed(),
		SprintSpeedMultiplier:             profile.GetSprintSpeedMultiplier(),
		Acceleration:                      profile.GetAcceleration(),
		Deceleration:                      profile.GetDeceleration(),
		GroundFriction:                    profile.GetGroundFriction(),
		AirAcceleration:                   profile.GetAirAcceleration(),
		JumpHeight:                        profile.GetJumpHeight(),
		JumpDurationMs:                    profile.GetJumpDurationMs(),
		RotationRateYaw:                   profile.GetRotationRateYaw(),
		GravityScale:                      profile.GetGravityScale(),
		BrakingFrictionFactor:             profile.GetBrakingFrictionFactor(),
		MaxSlopeDeg:                       profile.GetMaxSlopeDeg(),
		StepHeight:                        profile.GetStepHeight(),
		BaseDeadzone:                      profile.GetBaseDeadzone(),
		GroundedSpeedDeadzoneFactor:       profile.GetGroundedSpeedDeadzoneFactor(),
		GroundedSpeedDeadzoneMin:          profile.GetGroundedSpeedDeadzoneMin(),
		GroundedSpeedDeadzoneMax:          profile.GetGroundedSpeedDeadzoneMax(),
		GroundedTransitionDeadzoneMin:     profile.GetGroundedTransitionDeadzoneMin(),
		MoveSustainDeadzone:               profile.GetMoveSustainDeadzone(),
		MoveSustainTransitionDeadzone:     profile.GetMoveSustainTransitionDeadzone(),
		AirborneDeadzone:                  profile.GetAirborneDeadzone(),
		LeapRecentDeadzone:                profile.GetLeapRecentDeadzone(),
		LeapAirborneSnapshotDeadzone:      profile.GetLeapAirborneSnapshotDeadzone(),
		LeapLandingDeadzoneFactor:         profile.GetLeapLandingDeadzoneFactor(),
		LeapLandingDeadzoneMin:            profile.GetLeapLandingDeadzoneMin(),
		LeapLandingDeadzoneMax:            profile.GetLeapLandingDeadzoneMax(),
		LeapLandingClampIgnoreDeadzone:    profile.GetLeapLandingClampIgnoreDeadzone(),
		LeapLandingSoftSnapDeadzone:       profile.GetLeapLandingSoftSnapDeadzone(),
		DodgeRecentDeadzone:               profile.GetDodgeRecentDeadzone(),
		DodgeActiveDeadzone:               profile.GetDodgeActiveDeadzone(),
		DodgeExitDeadzoneFactor:           profile.GetDodgeExitDeadzoneFactor(),
		DodgeExitDeadzoneMin:              profile.GetDodgeExitDeadzoneMin(),
		DodgeExitDeadzoneMax:              profile.GetDodgeExitDeadzoneMax(),
		PostActionGroundedDeadzone:        profile.GetPostActionGroundedDeadzone(),
		CorrectionMaxStep:                 profile.GetCorrectionMaxStep(),
		HardSnapDistance:                  profile.GetHardSnapDistance(),
		SevereDesyncDistance:              profile.GetSevereDesyncDistance(),
		VisualSmoothingMs:                 profile.GetVisualSmoothingMs(),
		VisualSmoothingMaxDistance:        profile.GetVisualSmoothingMaxDistance(),
		RemoteVisualInterpolationMs:       profile.GetRemoteVisualInterpolationMs(),
		RemoteVisualMaxExtrapolationMs:    profile.GetRemoteVisualMaxExtrapolationMs(),
		RemoteVisualHardSnapDistance:      profile.GetRemoteVisualHardSnapDistance(),
		DodgeCarryHandoffMs:               profile.GetDodgeCarryHandoffMs(),
		LeapLandingCorrectionGraceMs:      profile.GetLeapLandingCorrectionGraceMs(),
		LeapGroundedCarryHandoffMs:        profile.GetLeapGroundedCarryHandoffMs(),
		MovementTurnResubmitDotThreshold:  profile.GetMovementTurnResubmitDotThreshold(),
		MovementTurnResubmitMinIntervalMs: profile.GetMovementTurnResubmitMinIntervalMs(),
		MovementSubmitIntervalMs:          profile.GetMovementSubmitIntervalMs(),
		SnapshotPollIntervalMs:            profile.GetSnapshotPollIntervalMs(),
		StrafeSpeedMultiplier:             profile.GetStrafeSpeedMultiplier(),
		BackpedalSpeedMultiplier:          profile.GetBackpedalSpeedMultiplier(),
		StrafeSprintSpeedMultiplier:       profile.GetStrafeSprintSpeedMultiplier(),
		BackpedalSprintSpeedMultiplier:    profile.GetBackpedalSprintSpeedMultiplier(),
	}
}

// RecoveryFixtureRuntimeContracts is a dev/test reconstruction fixture. Normal app
// boot must load DB-backed contracts through LoadRuntimeContractsFromDB and strict
// coverage validation; do not call this from production startup.
func RecoveryFixtureRuntimeContracts() RuntimeContracts {
	contracts := RuntimeContracts{
		Source:          "recovered_runtime_fallback",
		MovementProfile: recoveredMovementProfile(),
		ActionContracts: map[string]MovementActionRuntimeContract{},
		SkillContracts:  map[string]SkillRuntimeContract{},
		WolfPolicy: WolfRuntimePolicy{
			ContractID:                     wolfRuntimeContractID,
			ContractHash:                   wolfRuntimeContractID,
			CapabilityID:                   "wolf_pack_harasser",
			DesiredRangeCM:                 420,
			ChaseRangeCM:                   760,
			LungeRangeCM:                   220,
			RetreatRangeCM:                 130,
			OrbitSpeedCMS:                  360,
			ChaseSpeedCMS:                  620,
			LungeSpeedCMS:                  760,
			MaulSpeedCMS:                   420,
			RetreatSpeedCMS:                520,
			LungeWindupMS:                  3600,
			LungeActiveEndMS:               4030,
			LungeRecoveryMS:                500,
			LungeDistanceCM:                620,
			LungeDurationMS:                980,
			LungeArcHeightCM:               120,
			DodgeSkillID:                   "wolf_dodge",
			EvasionChainCount:              4,
			TargetOpportunityPolicyID:      "opportunity_wolf_harasser_v1",
			CommitAngleMaxDeg:              180,
			MinCommitDistanceCM:            120,
			MaxCommitDistanceCM:            760,
			ApproachMinDistanceCM:          260,
			ApproachMaxDistanceCM:          760,
			BiteRangeCM:                    260,
			LungeMinRangeCM:                180,
			LungeMaxRangeCM:                760,
			MaulPressureThreshold:          0.72,
			TargetMemoryMS:                 1200,
			NoReadySkillMemoryPolicy:       "observe_only",
			CandidateCooldownVisibility:    true,
			AllowBacksideCommit:            true,
			OrbitPolicyID:                  "orbit_wolf_harasser_combat_walk_v1",
			OrbitLocomotionMode:            "combat_walk",
			OrbitSpeedScale:                0.55,
			MinOrbitDurationMS:             700,
			SideSwitchCooldownMS:           900,
			AllowSideSwitchWhenTargetFaces: true,
			PreferLongSideCommit:           true,
			SideFlipChanceMultiplier:       0.35,
			LockSideDuringSetup:            true,
			RepeatSkillPenaltyWindowMS:     1200,
			RepeatSkillPenaltyMultiplier:   0.65,
			DodgeStaminaCostMultiplier:     0.5,
			StaminaRegenPerSecond:          12,
			MaxStamina:                     100,
			SkillBehaviorBindings: []CreatureSkillBehaviorRuntimeBinding{
				{ID: "recovered_bind_bite_circle", SkillID: "bite", TacticalState: "circle", DecisionPhase: "reposition", MinRangeCM: 0, MaxRangeCM: 300, Priority: 70, UsageWeight: 0.85, CooldownGroup: "wolf_bite", RequiresLineOfSight: true, Enabled: true},
				{ID: "recovered_bind_lunge_circle", SkillID: "lunge", TacticalState: "circle", DecisionPhase: "reposition", MinRangeCM: 180, MaxRangeCM: 760, Priority: 85, UsageWeight: 0.75, CooldownGroup: "wolf_lunge", RequiresLineOfSight: true, Enabled: true},
				{ID: "recovered_bind_lunge_approach", SkillID: "lunge", TacticalState: "approach", DecisionPhase: "acquire", MinRangeCM: 420, MaxRangeCM: 980, Priority: 95, UsageWeight: 1, CooldownGroup: "wolf_lunge", RequiresLineOfSight: true, Enabled: true},
				{ID: "recovered_bind_maul_pressure", SkillID: "maul", TacticalState: "pressure", DecisionPhase: "counter", MinRangeCM: 0, MaxRangeCM: 260, Priority: 100, UsageWeight: 0.9, CooldownGroup: "wolf_maul", RequiresLineOfSight: true, Enabled: true},
				{ID: "recovered_bind_dodge_pressure", SkillID: "wolf_dodge", TacticalState: "pressure", DecisionPhase: "evade", MinRangeCM: 0, MaxRangeCM: 420, Priority: 110, UsageWeight: 1.15, CooldownGroup: "wolf_dodge", RequiresLineOfSight: false, Enabled: true},
			},
		},
		CombatModes: recoveredCombatModeSlots(),
	}
	for _, contract := range []MovementActionRuntimeContract{
		{ID: "move_contract", AbilityKey: "move", ActionType: "move", DurationMS: 180, ActiveMS: 120, RecoveryMS: 60, ReconciliationContractID: "grounded_move_reconciliation", ReconciliationCategory: "grounded_move_reconciliation", PhaseWindowPolicy: "server_authoritative", PredictionErrorPolicy: "bounded_smooth_correction"},
		{ID: "turn_contract", AbilityKey: "turn", ActionType: "turn", DurationMS: 180, ActiveMS: 120, RecoveryMS: 60, YawRateDegPerSec: 720, ReconciliationContractID: "turn_reconciliation", ReconciliationCategory: "turn_reconciliation", PhaseWindowPolicy: "server_authoritative", PredictionErrorPolicy: "bounded_smooth_correction"},
		{ID: "dodge_contract", AbilityKey: "dodge", ActionType: "dodge", DurationMS: 320, ActiveMS: 260, RecoveryMS: 60, DistanceCM: 260, BaseSpeedCMS: 812.5, SpeedCurveSamples: recoveredMovementCurve("dodge_v1_full_iframe"), VerticalCurveSamples: recoveredVerticalCurve("dodge_v1_full_iframe"), ReconciliationContractID: "dodge_reconciliation", ReconciliationCategory: "dodge_reconciliation", PhaseWindowPolicy: "server_authoritative", PredictionErrorPolicy: "bounded_smooth_correction"},
		{ID: "jump_contract", AbilityKey: "jump", ActionType: "leap", DurationMS: 620, AirborneDurationMS: 560, ActiveMS: 560, RecoveryMS: 60, DistanceCM: 280, BaseSpeedCMS: 452, SpeedCurveSamples: recoveredMovementCurve("jump_v1_authoritative_grounded_handoff"), VerticalCurveSamples: recoveredVerticalCurve("jump_v1_authoritative_grounded_handoff"), JumpZVelocity: 620, GravityScale: 1, ExpectedApexMS: 310, LandingDetectionPolicy: "server_grounded_handoff", GroundZPolicy: "server_position_is_actor_root", AllowsAirControl: true, AirControlModifier: 0.35, ReconciliationContractID: "leap_reconciliation", ReconciliationCategory: "leap_reconciliation", PhaseWindowPolicy: "server_authoritative", PredictionErrorPolicy: "bounded_smooth_correction"},
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

// recoveredPlayerSkillDamage returns the authoritative base/posture damage for a player
// skill, taken verbatim from db-apeiron bootstrap/013_player_sword_shield_skill_seed.sql.
// This materializes the canonical seed values in the recovered runtime until the DB skill
// proto exposes base_damage/posture_damage (damage-pipeline brick 2b), at which point
// loadSkillRuntimeContract should source them from the DB instead.
func recoveredPlayerSkillDamage(skillID string) (damage float64, posture float64) {
	switch skillID {
	case "player_basic_attack_1":
		return 8, 10
	case "player_basic_attack_2":
		return 7, 9
	case "player_basic_attack_3":
		return 6, 18
	case "player_shield_bash":
		return 10, 26
	case "player_shield_rush":
		return 14, 34
	default:
		return 0, 0
	}
}

func recoveredPlayerSkillMaxTargets(skillID string) int32 {
	switch skillID {
	case "player_basic_attack_2":
		return 2
	case "player_basic_attack_3":
		return 3
	case "player_shield_bash":
		return 4
	case "player_shield_rush":
		return 5
	default:
		return 1
	}
}

func recoveredPlayerSkillHitboxes(skillID string) []*dbv1.SkillHitboxProfile {
	targetType := "enemy"
	maxTargets := recoveredPlayerSkillMaxTargets(skillID)
	profile := &dbv1.SkillHitboxProfile{
		Id:                  skillID + "_recovered_temporal_hitbox",
		SkillId:             skillID,
		HitboxShape:         "temporal_sweep",
		TargetType:          &targetType,
		MaxTargets:          &maxTargets,
		RequiresLineOfSight: true,
		CanHitNeutral:       true,
	}
	switch skillID {
	case "player_basic_attack_1":
		profile.HitboxStartMs, profile.HitboxEndMs = 90, 230
		profile.Length, profile.Angle, profile.Radius = 230, 90, 50
	case "player_basic_attack_2":
		profile.HitboxStartMs, profile.HitboxEndMs = 100, 250
		profile.Length, profile.Angle, profile.Radius = 250, 90, 58
	case "player_basic_attack_3":
		profile.HitboxStartMs, profile.HitboxEndMs = 180, 440
		profile.Length, profile.Angle, profile.Radius = 440, 95, 60
	case "player_shield_bash":
		profile.HitboxStartMs, profile.HitboxEndMs = 120, 340
		profile.Length, profile.Radius = 210, 95
	case "player_shield_rush":
		profile.HitboxStartMs, profile.HitboxEndMs = 160, 590
		profile.Length, profile.Radius = 290, 96
	default:
		return nil
	}
	profile.MotionProfile = recoveredPlayerSkillHitboxMotionProfile(skillID)
	profile.DamageGroupId = recoveredPlayerSkillHitboxDamageGroupID(skillID)
	return []*dbv1.SkillHitboxProfile{profile}
}

func recoveredPlayerSkillHitboxDamageGroupID(skillID string) string {
	switch skillID {
	case "player_basic_attack_1":
		return "player_basic_attack_1_damage"
	case "player_basic_attack_2":
		return "player_basic_attack_2_damage"
	case "player_basic_attack_3":
		return "player_basic_attack_3_damage"
	case "player_shield_bash":
		return "player_shield_bash_front_push"
	case "player_shield_rush":
		return "player_shield_rush_front_contact"
	default:
		return ""
	}
}

func recoveredPlayerSkillHitboxMotionProfile(skillID string) *dbv1.SkillHitboxMotionProfile {
	id, sweepShape, samples := recoveredPlayerSkillHitboxMotionSamples(skillID)
	if id == "" || len(samples) == 0 {
		return nil
	}
	return &dbv1.SkillHitboxMotionProfile{
		Id:            id,
		Enabled:       true,
		MotionType:    "timeline_sweep",
		TimeBasis:     "hitbox_window_normalized",
		Interpolation: "linear",
		SweepShape:    sweepShape,
		DamageGroupId: recoveredPlayerSkillHitboxDamageGroupID(skillID),
		Samples:       samples,
	}
}

func recoveredPlayerSkillHitboxMotionSamples(skillID string) (string, string, []*dbv1.SkillHitboxMotionSample) {
	switch skillID {
	case "player_basic_attack_1":
		return "motion_player_basic_attack_1_forward_v1", "capsule_strip", []*dbv1.SkillHitboxMotionSample{
			recoveredHitboxMotionSample(0, 0.00, 35, 0, 90, 95, 0, 150, 48, 70, 0, 0),
			recoveredHitboxMotionSample(1, 0.50, 85, 0, 90, 100, 0, 150, 50, 150, 0, 0),
			recoveredHitboxMotionSample(2, 1.00, 130, 0, 90, 100, 0, 150, 50, 210, 0, 0),
		}
	case "player_basic_attack_2":
		return "motion_player_basic_attack_2_right_to_left_v1", "arc_slice", []*dbv1.SkillHitboxMotionSample{
			recoveredHitboxMotionSample(0, 0.00, 70, 35, 95, 0, 0, 150, 55, 155, -45, -15),
			recoveredHitboxMotionSample(1, 0.50, 85, 0, 95, 0, 0, 150, 58, 165, -15, 15),
			recoveredHitboxMotionSample(2, 1.00, 70, -35, 95, 0, 0, 150, 55, 155, 15, 45),
		}
	case "player_basic_attack_3":
		return "motion_player_basic_attack_3_shield_drive_v1", "capsule_strip", []*dbv1.SkillHitboxMotionSample{
			recoveredHitboxMotionSample(0, 0.00, 45, 0, 95, 120, 0, 155, 60, 90, 0, 0),
			recoveredHitboxMotionSample(1, 0.55, 90, 0, 95, 120, 0, 155, 60, 175, 0, 0),
			recoveredHitboxMotionSample(2, 1.00, 115, 0, 95, 120, 0, 155, 60, 210, 0, 0),
		}
	case "player_shield_bash":
		return "motion_player_shield_bash_front_push_v1", "capsule_strip", []*dbv1.SkillHitboxMotionSample{
			recoveredHitboxMotionSample(0, 0.00, 45, 0, 95, 190, 0, 160, 95, 95, 0, 0),
			recoveredHitboxMotionSample(1, 0.50, 85, 0, 95, 190, 0, 160, 95, 160, 0, 0),
			recoveredHitboxMotionSample(2, 1.00, 120, 0, 95, 190, 0, 160, 95, 210, 0, 0),
		}
	case "player_shield_rush":
		return "motion_player_shield_rush_front_contact_v1", "capsule_strip", []*dbv1.SkillHitboxMotionSample{
			recoveredHitboxMotionSample(0, 0.00, 45, 0, 100, 190, 0, 160, 96, 105, 0, 0),
			recoveredHitboxMotionSample(1, 0.50, 105, 0, 100, 190, 0, 160, 96, 220, 0, 0),
			recoveredHitboxMotionSample(2, 1.00, 145, 0, 100, 190, 0, 160, 96, 290, 0, 0),
		}
	default:
		return "", "", nil
	}
}

func recoveredHitboxMotionSample(index int32, t float64, offsetX, offsetY, offsetZ, sizeX, sizeY, sizeZ, radius, length, startAngleDeg, endAngleDeg float64) *dbv1.SkillHitboxMotionSample {
	return &dbv1.SkillHitboxMotionSample{
		SampleIndex:   index,
		T:             t,
		OffsetX:       offsetX,
		OffsetY:       offsetY,
		OffsetZ:       offsetZ,
		SizeX:         sizeX,
		SizeY:         sizeY,
		SizeZ:         sizeZ,
		Radius:        radius,
		Length:        length,
		StartAngleDeg: startAngleDeg,
		EndAngleDeg:   endAngleDeg,
	}
}

func curvePoint(t, value float64) movement.MovementActionCurvePoint {
	return movement.MovementActionCurvePoint{T: t, Value: value}
}

func recoveredMovementCurve(contractID string) []movement.MovementActionCurvePoint {
	switch contractID {
	case "dodge_v1_full_iframe":
		return []movement.MovementActionCurvePoint{curvePoint(0, 0.35), curvePoint(0.35, 1), curvePoint(1, 0.2)}
	case "jump_v1_authoritative_grounded_handoff":
		return []movement.MovementActionCurvePoint{curvePoint(0, 0.25), curvePoint(0.35, 0.85), curvePoint(1, 0.35)}
	case "player_basic_attack_1":
		return []movement.MovementActionCurvePoint{curvePoint(0, 0.35), curvePoint(0.35, 1), curvePoint(1, 0.2)}
	case "player_basic_attack_2":
		return []movement.MovementActionCurvePoint{curvePoint(0, 0.25), curvePoint(0.5, 0.8), curvePoint(1, 0.15)}
	case "player_basic_attack_3":
		return []movement.MovementActionCurvePoint{curvePoint(0, 0.2), curvePoint(0.25, 0.75), curvePoint(0.65, 1), curvePoint(1, 0.25)}
	case "player_shield_bash":
		return []movement.MovementActionCurvePoint{curvePoint(0, 0.2), curvePoint(0.4, 1), curvePoint(1, 0.15)}
	case "player_shield_rush":
		return []movement.MovementActionCurvePoint{curvePoint(0, 0.1), curvePoint(0.2, 0.85), curvePoint(0.75, 1), curvePoint(1, 0.25)}
	case "lunge":
		return []movement.MovementActionCurvePoint{curvePoint(0, 0.4), curvePoint(0.18, 1), curvePoint(0.72, 0.85), curvePoint(1, 0.35)}
	case "wolf_dodge":
		return []movement.MovementActionCurvePoint{curvePoint(0, 0.4), curvePoint(0.35, 1), curvePoint(1, 0.2)}
	case "maul":
		return []movement.MovementActionCurvePoint{curvePoint(0, 0.2), curvePoint(0.45, 1), curvePoint(1, 0.25)}
	default:
		return nil
	}
}

func recoveredVerticalCurve(contractID string) []movement.MovementActionCurvePoint {
	switch contractID {
	case "dodge_v1_full_iframe":
		return []movement.MovementActionCurvePoint{curvePoint(0, 0), curvePoint(0.4, 18), curvePoint(1, 0)}
	case "jump_v1_authoritative_grounded_handoff":
		return []movement.MovementActionCurvePoint{curvePoint(0, 0), curvePoint(0.5, 180), curvePoint(1, 0)}
	case "lunge":
		return []movement.MovementActionCurvePoint{curvePoint(0, 0), curvePoint(0.36, 120), curvePoint(1, 0)}
	case "wolf_dodge":
		return []movement.MovementActionCurvePoint{curvePoint(0, 0), curvePoint(0.4, 28), curvePoint(1, 0)}
	default:
		return nil
	}
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
		SpeedCurveSamples:        recoveredMovementCurve(skillID),
		ReconciliationContractID: "grounded_skill_action_reconciliation",
		// Published reconciliation_mode MUST be a string the Unreal client recognizes
		// (ApeironReconciliationModeFromServerString). The category is the wire mode
		// "grounded_skill_action" -> EApeironPlayerReconciliationMode::SkillGroundedAction.
		// The verbose "_reconciliation" form parsed as None and made player skills rubberband.
		ReconciliationCategory: "grounded_skill_action",
		PhaseWindowPolicy:      "server_authoritative",
		PredictionErrorPolicy:  "bounded_smooth_correction",
		RootMotionOwner:        "skill",
		ContactPolicy:          "authoritative_contact",
	}
	damage, posture := recoveredPlayerSkillDamage(skillID)
	return SkillRuntimeContract{
		SkillID:                  skillID,
		MovementActionContractID: action.ID,
		MovementAction:           action,
		Damage:                   damage,
		PostureDamage:            posture,
		MaxTargets:               recoveredPlayerSkillMaxTargets(skillID),
		Blockable:                true,
		Hitboxes:                 recoveredPlayerSkillHitboxes(skillID),
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

func recoveredCreatureSkillStaminaCost(skillID string) float64 {
	switch skillID {
	case "wolf_dodge":
		return 6
	case "bite":
		return 10
	case "maul":
		return 22
	case "lunge":
		return 24
	default:
		return 0
	}
}

func recoveredCreatureSkillDamage(skillID string) (damage float64, posture float64) {
	switch skillID {
	case "bite":
		return 9, 17.6
	case "lunge":
		return 6.5, 19.2
	case "maul":
		return 6.5, 19.2
	default:
		return 0, 0
	}
}

func recoveredCreatureSkillMaxTargets(skillID string) int32 {
	switch skillID {
	case "maul":
		return 2
	default:
		return 1
	}
}

func recoveredCreatureSkillHitboxes(skillID string) []*dbv1.SkillHitboxProfile {
	damageGroupID := recoveredCreatureSkillHitboxDamageGroupID(skillID)
	motionProfile := recoveredCreatureSkillHitboxMotionProfile(skillID)
	if damageGroupID == "" || motionProfile == nil {
		return nil
	}
	targetType := "enemy"
	maxTargets := recoveredCreatureSkillMaxTargets(skillID)
	profile := &dbv1.SkillHitboxProfile{
		Id:                  recoveredCreatureSkillHitboxID(skillID),
		SkillId:             skillID,
		HitboxShape:         "temporal_sweep",
		TargetType:          &targetType,
		MaxTargets:          &maxTargets,
		RequiresLineOfSight: true,
		CanHitNeutral:       true,
		DamageGroupId:       damageGroupID,
		MotionProfile:       motionProfile,
	}
	switch skillID {
	case "bite":
		profile.HitboxStartMs, profile.HitboxEndMs = 120, 340
		profile.OffsetX, profile.OffsetY, profile.OffsetZ = 80, 0, 90
		profile.SizeX, profile.SizeY, profile.SizeZ = 95, 0, 115
		profile.Radius, profile.Length = 48, 145
	case "lunge":
		profile.HitboxStartMs, profile.HitboxEndMs = 3600, 4030
		profile.OffsetX, profile.OffsetY, profile.OffsetZ = 130, 0, 105
		profile.SizeX, profile.SizeY, profile.SizeZ = 100, 0, 120
		profile.Radius, profile.Length = 50, 320
	case "maul":
		profile.HitboxStartMs, profile.HitboxEndMs = 180, 440
		profile.OffsetX, profile.OffsetY, profile.OffsetZ = 80, 0, 100
		profile.SizeZ = 130
		profile.Radius, profile.Length, profile.Angle = 62, 170, 140
	default:
		return nil
	}
	return []*dbv1.SkillHitboxProfile{profile}
}

func recoveredCreatureSkillHitboxID(skillID string) string {
	switch skillID {
	case "bite":
		return "hitbox_bite_0"
	case "lunge":
		return "hitbox_lunge_0"
	case "maul":
		return "hitbox_maul_0"
	default:
		return ""
	}
}

func recoveredCreatureSkillHitboxDamageGroupID(skillID string) string {
	switch skillID {
	case "bite":
		return "wolf_bite_damage"
	case "lunge":
		return "wolf_lunge_damage"
	case "maul":
		return "wolf_maul_damage"
	default:
		return ""
	}
}

func recoveredCreatureSkillHitboxMotionProfile(skillID string) *dbv1.SkillHitboxMotionProfile {
	id, sweepShape, samples := recoveredCreatureSkillHitboxMotionSamples(skillID)
	if id == "" || len(samples) == 0 {
		return nil
	}
	return &dbv1.SkillHitboxMotionProfile{
		Id:            id,
		Enabled:       true,
		MotionType:    "timeline_sweep",
		TimeBasis:     "hitbox_window_normalized",
		Interpolation: "linear",
		SweepShape:    sweepShape,
		DamageGroupId: recoveredCreatureSkillHitboxDamageGroupID(skillID),
		Samples:       samples,
	}
}

func recoveredCreatureSkillHitboxMotionSamples(skillID string) (string, string, []*dbv1.SkillHitboxMotionSample) {
	switch skillID {
	case "bite":
		return "motion_wolf_bite_melee_v1", "capsule_strip", []*dbv1.SkillHitboxMotionSample{
			recoveredHitboxMotionSample(0, 0.00, 45, 0, 85, 90, 0, 115, 45, 70, 0, 0),
			recoveredHitboxMotionSample(1, 0.55, 80, 0, 90, 95, 0, 115, 48, 125, 0, 0),
			recoveredHitboxMotionSample(2, 1.00, 95, 0, 85, 90, 0, 115, 45, 145, 0, 0),
		}
	case "lunge":
		return "motion_wolf_lunge_cross_v1", "capsule_strip", []*dbv1.SkillHitboxMotionSample{
			recoveredHitboxMotionSample(0, 0.00, 60, 0, 90, 100, 0, 120, 50, 100, 0, 0),
			recoveredHitboxMotionSample(1, 0.55, 140, 0, 110, 100, 0, 120, 50, 230, 0, 0),
			recoveredHitboxMotionSample(2, 1.00, 210, 0, 90, 100, 0, 120, 50, 320, 0, 0),
		}
	case "maul":
		return "motion_wolf_maul_lateral_counter_v1", "arc_slice", []*dbv1.SkillHitboxMotionSample{
			recoveredHitboxMotionSample(0, 0.00, 65, 40, 95, 0, 0, 125, 58, 120, -70, -25),
			recoveredHitboxMotionSample(1, 0.45, 90, 0, 100, 0, 0, 130, 62, 170, -25, 25),
			recoveredHitboxMotionSample(2, 1.00, 65, -40, 95, 0, 0, 125, 58, 120, 25, 70),
		}
	default:
		return "", "", nil
	}
}

func recoveredCreatureSkillContract(skillID string, contractID string, actionType string, reconciliation string, contactPolicy string, distance float64, durationMS, activeMS, recoveryMS, windupMS, skillRecoveryMS, cooldownMS int32) SkillRuntimeContract {
	action := MovementActionRuntimeContract{
		ID:                       contractID,
		AbilityKey:               skillID,
		ActionType:               actionType,
		DurationMS:               durationMS,
		AirborneDurationMS:       activeMS,
		ActiveMS:                 activeMS,
		RecoveryMS:               recoveryMS,
		DistanceCM:               distance,
		SpeedCurveSamples:        recoveredMovementCurve(skillID),
		VerticalCurveSamples:     recoveredVerticalCurve(skillID),
		GravityScale:             1,
		ReconciliationContractID: reconciliation,
		ReconciliationCategory:   reconciliation,
		PhaseWindowPolicy:        actionType,
		PredictionErrorPolicy:    "bounded_smooth_correction",
		RootMotionOwner:          "movement",
		ContactPolicy:            contactPolicy,
	}
	if skillID == "lunge" {
		action.JumpZVelocity = 700
		action.ExpectedApexMS = 350
		action.LandingDetectionPolicy = "server_grounded_handoff"
		action.GroundZPolicy = "server_position_is_actor_root"
	}
	damage, posture := recoveredCreatureSkillDamage(skillID)
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
		Damage:                   damage,
		PostureDamage:            posture,
		StaminaCost:              recoveredCreatureSkillStaminaCost(skillID),
		TargetPolicy:             "target_direction",
		ContactPolicy:            contactPolicy,
		MaxTargets:               recoveredCreatureSkillMaxTargets(skillID),
		Blockable:                true,
		Hitboxes:                 recoveredCreatureSkillHitboxes(skillID),
		Enabled:                  true,
	}
}

func recoveredCombatModeSlots() []*gamev1.CombatModeSlot {
	return []*gamev1.CombatModeSlot{
		{CombatModeId: swordShieldBulwarkModeID, SlotIndex: 2, SkillId: "player_shield_bash", Enabled: true},
		{CombatModeId: swordShieldBulwarkModeID, SlotIndex: 3, SkillId: "player_shield_rush", Enabled: true},
	}
}

func combatModeSlotsFromDB(slots []*dbv1.WeaponCombatModeSlot) []*gamev1.CombatModeSlot {
	out := make([]*gamev1.CombatModeSlot, 0, len(slots))
	for _, slot := range slots {
		if slot == nil {
			continue
		}
		slotIndex := combatInputSlotIndex(slot.GetInputSlot())
		if slotIndex == 0 {
			continue
		}
		out = append(out, &gamev1.CombatModeSlot{
			CombatModeId: normalizeCombatModeID(slot.GetCombatModeId()),
			SlotIndex:    slotIndex,
			SkillId:      slot.GetSkillId(),
			Enabled:      slot.GetIsEnabled() && slot.GetSkillId() != "",
		})
	}
	return out
}

func combatInputSlotIndex(input string) uint32 {
	switch strings.ToUpper(strings.TrimSpace(input)) {
	case "Q":
		return 1
	case "R":
		return 2
	case "F":
		return 3
	case "G":
		return 4
	default:
		return 0
	}
}

const (
	swordShieldVanguardModeID = "mode_sword_shield_vanguard"
	swordShieldBulwarkModeID  = "mode_sword_shield_bulwark"
)

func normalizeCombatModeID(mode string) string {
	normalized := strings.ToLower(strings.TrimSpace(mode))
	switch normalized {
	case "vanguard", swordShieldVanguardModeID:
		return swordShieldVanguardModeID
	case "bulwark", swordShieldBulwarkModeID:
		return swordShieldBulwarkModeID
	default:
		return strings.TrimSpace(mode)
	}
}

func isBulwarkCombatMode(mode string) bool {
	return normalizeCombatModeID(mode) == swordShieldBulwarkModeID
}

func (c RuntimeContracts) contractForAbility(ability string) MovementActionRuntimeContract {
	registry := movement.NewActionContractRegistry(c.ActionContracts)
	if contract, ok := registry.Resolve(ability); ok {
		return contract
	}
	return MovementActionRuntimeContract{AbilityKey: ability}
}

func (c RuntimeContracts) skillContract(skillID string) SkillRuntimeContract {
	if contract, ok := c.SkillContracts[skillID]; ok {
		return contract
	}
	return SkillRuntimeContract{SkillID: skillID}
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
	}
	preferred = append(preferred, requiredRuntimeSkillIDs()...)
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
	meta := movementActionContractMetadata{}
	_ = json.Unmarshal([]byte(contract.GetMetadataJson()), &meta)
	yawRate := meta.YawRateDegPerSec
	if yawRate <= 0 {
		yawRate = contract.GetYawDegrees()
	}
	return MovementActionRuntimeContract{
		ID:                       contract.GetId(),
		AbilityKey:               abilityKey,
		ActionType:               contract.GetActionType(),
		DurationMS:               contract.GetDurationMs(),
		AirborneDurationMS:       meta.AirborneDurationMS,
		ActiveMS:                 contract.GetActiveMs(),
		RecoveryMS:               contract.GetRecoveryMs(),
		DistanceCM:               contract.GetDistanceCm(),
		BaseSpeedCMS:             contract.GetBaseSpeedCmS(),
		SpeedCurveSamples:        movementCurvePointsFromDB(contract.GetSpeedCurve()),
		VerticalCurveSamples:     movementCurvePointsFromDB(contract.GetVerticalCurve()),
		JumpZVelocity:            meta.JumpZVelocity,
		GravityScale:             meta.GravityScale,
		ExpectedApexMS:           meta.ExpectedApexMS,
		LandingDetectionPolicy:   meta.LandingDetectionPolicy,
		GroundZPolicy:            meta.GroundZPolicy,
		CapsuleBaseOffset:        meta.CapsuleBaseOffset,
		AllowsAirControl:         meta.AllowsAirControl,
		AirControlModifier:       meta.AirControlModifier,
		YawRateDegPerSec:         yawRate,
		ReconciliationContractID: contract.GetReconciliationContractId(),
		ReconciliationCategory:   category,
		PhaseWindowPolicy:        contract.GetPhaseWindowPolicy(),
		PredictionErrorPolicy:    contract.GetPredictionErrorPolicy(),
		RootMotionOwner:          contract.GetRootMotionOwner(),
		ContactPolicy:            contract.GetContactPolicy(),
	}
}

func movementCurvePointsFromDB(samples []*dbv1.MovementCurveSample) []movement.MovementActionCurvePoint {
	if len(samples) == 0 {
		return nil
	}
	out := make([]movement.MovementActionCurvePoint, 0, len(samples))
	for _, sample := range samples {
		if sample == nil {
			continue
		}
		out = append(out, movement.MovementActionCurvePoint{
			T:     sample.GetT(),
			Value: sample.GetValue(),
		})
	}
	return out
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
		GroundedTransitionDeadzoneMin:     34,
		MoveSustainDeadzone:               45,
		MoveSustainTransitionDeadzone:     65,
		AirborneDeadzone:                  120,
		LeapRecentDeadzone:                140,
		LeapAirborneSnapshotDeadzone:      165,
		LeapLandingDeadzoneFactor:         0.12,
		LeapLandingDeadzoneMin:            80,
		LeapLandingDeadzoneMax:            180,
		LeapLandingClampIgnoreDeadzone:    145,
		LeapLandingSoftSnapDeadzone:       145,
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
		DodgeCarryHandoffMs:               120,
		LeapLandingCorrectionGraceMs:      120,
		LeapGroundedCarryHandoffMs:        70,
		MovementTurnResubmitDotThreshold:  0.92,
		MovementTurnResubmitMinIntervalMs: 33,
		MovementSubmitIntervalMs:          33,
		SnapshotPollIntervalMs:            33,
		StrafeSpeedMultiplier:             0.92,
		BackpedalSpeedMultiplier:          0.65,
		StrafeSprintSpeedMultiplier:       0.75,
		BackpedalSprintSpeedMultiplier:    0.75,
	}
}

func distanceFromContract(contract MovementActionRuntimeContract, fallback float64) float64 {
	if math.Abs(contract.DistanceCM) > 0 {
		return contract.DistanceCM
	}
	return fallback
}

func positiveOr(value, fallback float64) float64 {
	if value > 0 {
		return value
	}
	return fallback
}
