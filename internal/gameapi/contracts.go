package gameapi

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	dbv1 "db-apeiron/gen/apeiron/v1"
	gamev1 "server-apeiron/gen/apeiron/game/v1"
	creatureai "server-apeiron/internal/ai"
	"server-apeiron/internal/movement"

	"google.golang.org/grpc"
)

type ContractSource interface {
	GetSkill(context.Context, *dbv1.IdRequest, ...grpc.CallOption) (*dbv1.SkillResponse, error)
	GetSkillImpactProfile(context.Context, *dbv1.IdRequest, ...grpc.CallOption) (*dbv1.SkillImpactProfileResponse, error)
	GetSkillActionTiming(context.Context, *dbv1.IdRequest, ...grpc.CallOption) (*dbv1.SkillActionTimingResponse, error)
	GetSkillMovementActionBinding(context.Context, *dbv1.IdRequest, ...grpc.CallOption) (*dbv1.SkillMovementActionBindingResponse, error)
	GetSkillHitboxProfiles(context.Context, *dbv1.IdRequest, ...grpc.CallOption) (*dbv1.SkillHitboxProfilesResponse, error)
	GetWeaponCombatModeSlots(context.Context, *dbv1.IdRequest, ...grpc.CallOption) (*dbv1.WeaponCombatModeSlotsResponse, error)
}

type ProfileContractSource interface {
	GetCombatCoreProfile(context.Context, *dbv1.IdRequest, ...grpc.CallOption) (*dbv1.CombatCoreProfileResponse, error)
	GetCombatDefenseContract(context.Context, *dbv1.IdRequest, ...grpc.CallOption) (*dbv1.CombatDefenseContractResponse, error)
	GetMovementActionContract(context.Context, *dbv1.IdRequest, ...grpc.CallOption) (*dbv1.MovementActionContractResponse, error)
	GetActionOrientationPolicy(context.Context, *dbv1.IdRequest, ...grpc.CallOption) (*dbv1.ActionOrientationPolicyResponse, error)
	GetActionEnvelopePolicy(context.Context, *dbv1.IdRequest, ...grpc.CallOption) (*dbv1.ActionEnvelopePolicyResponse, error)
	GetSkillActionPolicyBinding(context.Context, *dbv1.IdRequest, ...grpc.CallOption) (*dbv1.SkillActionPolicyBindingResponse, error)
	GetMovementReconciliationContract(context.Context, *dbv1.IdRequest, ...grpc.CallOption) (*dbv1.MovementReconciliationContractResponse, error)
	GetRuntimeMovementReconciliationProfile(context.Context, *dbv1.IdRequest, ...grpc.CallOption) (*dbv1.RuntimeMovementReconciliationProfileResponse, error)
	GetCreatureBehaviorRuntimeContract(context.Context, *dbv1.IdRequest, ...grpc.CallOption) (*dbv1.CreatureBehaviorRuntimeContractResponse, error)
	GetCreatureTargetOpportunityPolicy(context.Context, *dbv1.IdRequest, ...grpc.CallOption) (*dbv1.CreatureTargetOpportunityPolicyResponse, error)
	GetCreatureOrbitPolicy(context.Context, *dbv1.IdRequest, ...grpc.CallOption) (*dbv1.CreatureOrbitPolicyResponse, error)
	GetCreatureEvasionPolicies(context.Context, *dbv1.IdRequest, ...grpc.CallOption) (*dbv1.CreatureEvasionPoliciesResponse, error)
	GetCreatureSkillSetupPolicies(context.Context, *dbv1.IdRequest, ...grpc.CallOption) (*dbv1.CreatureSkillSetupPoliciesResponse, error)
	GetCreatureSkillBehaviorBindings(context.Context, *dbv1.IdRequest, ...grpc.CallOption) (*dbv1.CreatureSkillBehaviorBindingsResponse, error)
}

type CreatureContractSource interface {
	GetCreatureRuntimeData(context.Context, *dbv1.IdRequest, ...grpc.CallOption) (*dbv1.CreatureRuntimeDataResponse, error)
}

type RuntimeContracts struct {
	Source string

	MovementProfile *gamev1.MovementReconciliationProfile
	ActionContracts map[string]MovementActionRuntimeContract
	SkillContracts  map[string]SkillRuntimeContract
	CombatCore      CombatCoreRuntimeContracts
	Defense         DefenseRuntimeContracts
	WolfPolicy      WolfRuntimePolicy
	CombatModes     []*gamev1.CombatModeSlot
	LoadIssues      []string

	// PlayerImpactResponseProfile is the player hit-material/VFX response, mirroring the
	// wolf's creature_template.impact_response_profile (loaded into WolfPolicy). The DB
	// source (a player material/equipment profile) is the follow-up; empty falls back to
	// "flesh_blood_red" via playerImpactResponse() so current behavior is preserved.
	PlayerImpactResponseProfile string
}

// playerImpactResponse returns the player's contract-driven impact response profile, or
// the canonical default when no profile is configured.
func (c RuntimeContracts) playerImpactResponse() string {
	if p := strings.TrimSpace(c.PlayerImpactResponseProfile); p != "" {
		return p
	}
	return "flesh_blood_red"
}

type RuntimeContractCoverageReport struct {
	Source     string
	Strict     bool
	Ready      bool
	Categories []RuntimeContractCoverageCategory
}

type RuntimeContractCoverageCategory struct {
	Name     string
	Ready    bool
	Blockers []string
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
	// dev fixture mirrors the canonical seed values (fixturePlayerSkillDamage)
	// for isolated unit tests; normal runtime must load them from DB.
	Damage         float64
	PostureDamage  float64
	StaminaCost    float64
	Range          float64
	MaxTargets     int32
	Blockable      bool
	Impact         *dbv1.SkillImpactProfile
	ControlEffects []*dbv1.SkillControlEffect
	Hitboxes       []*dbv1.SkillHitboxProfile
	Orientation    *dbv1.ActionOrientationPolicy
	Envelope       *dbv1.ActionEnvelopePolicy
	ActionPolicy   *dbv1.SkillActionPolicyBinding
	Enabled        bool
}

type CombatCoreRuntimeContracts struct {
	PlayerProfileID   string
	CreatureProfileID string
	Profiles          map[string]*dbv1.CombatCoreProfile
}

type DefenseRuntimeContracts struct {
	PlayerGuardContractID   string
	CreatureGuardContractID string
	Contracts               map[string]*dbv1.CombatDefenseContract
}

type WolfRuntimePolicy struct {
	ContractID                     string
	ContractHash                   string
	CapabilityID                   string
	TemplateID                     string
	ImpactResponseProfile          string
	DesiredRangeCM                 float64
	ChaseRangeCM                   float64
	LungeRangeCM                   float64
	RetreatRangeCM                 float64
	OrbitSpeedCMS                  float64
	ChaseSpeedCMS                  float64
	LungeSpeedCMS                  float64
	MaulSpeedCMS                   float64
	RetreatSpeedCMS                float64
	TurnRateDegPerSec              float64
	LungeWindupMS                  int32
	LungeActiveEndMS               int32
	LungeRecoveryMS                int32
	LungeDistanceCM                float64
	LungeDurationMS                int32
	LungeArcHeightCM               float64
	DodgeSkillID                   string
	EvasionChainCount              int32
	EvasionLateralBias             float64
	EvasionBackstepBias            float64
	EvasionPressureThreshold       float64
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
	DodgeUnderPressure             bool
	MaulCounterUnderPressure       bool
	MaulCounterChance              float64
	DodgeRetreatMultiplier         float64
	GlobalDodgeMultiplier          float64
	CommitThreatWeight             float64
	ClosingThreatWeight            float64
	DefensiveBiteWeight            float64
	FleeingLungeWeight             float64
	LowResourceRiskFloor           float64
	DodgeCommittedThreatMultiplier float64
	VulnerableBiteMultiplier       float64
	VulnerableMaulMultiplier       float64
	TacticalDestinationDistanceCM  float64
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
	SkillSetupPolicies             []CreatureSkillSetupRuntimePolicy
	Threat                         ThreatRuntimeProfile
}

// ThreatRuntimeProfile is the creature's data-driven threat/aggro tuning, loaded from the
// creature behavior contract metadata ("threat" object). See
// docs/roadmap/aaa-threat-aggro-runtime-roadmap.md. Slice 1 uses the damage/posture/decay
// fields; selection/proximity/leash fields land in later slices.
type ThreatRuntimeProfile struct {
	DamageThreatPerPoint      float64
	PostureThreatPerPoint     float64
	ProximityThreatPerSec     float64
	ProximityRangeCM          float64
	FirstAggroBonus           float64
	DecayPerSec               float64
	LOSBreakDecayMultiplier   float64
	OutOfRangeDecayMultiplier float64
	SwitchThresholdRatio      float64
	SwitchCooldownMS          int32
	LeashDistanceCM           float64
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

type CreatureSkillSetupRuntimePolicy struct {
	ID                  string
	SkillID             string
	SetupType           string
	MinSetupMS          int32
	MaxSetupMS          int32
	CommitDistanceCM    float64
	PreferredMinRangeCM float64
	PreferredMaxRangeCM float64
	MovementTactic      string
	LockSideDuringSetup bool
	Enabled             bool
}

type creaturePressurePolicyJSON struct {
	RepeatSkillPenaltyMultiplier   float64 `json:"repeatSkillPenaltyMultiplier"`
	DodgeUnderPressure             bool    `json:"dodgeUnderPressure"`
	MaulCounterUnderPressure       bool    `json:"maulCounterUnderPressure"`
	MaulCounterChance              float64 `json:"maulCounterChance"`
	DodgeRetreatMultiplier         float64 `json:"dodgeRetreatMultiplier"`
	GlobalDodgeMultiplier          float64 `json:"globalDodgeMultiplier"`
	CommitThreatWeight             float64 `json:"commitThreatWeight"`
	ClosingThreatWeight            float64 `json:"closingThreatWeight"`
	DefensiveBiteWeight            float64 `json:"defensiveBiteWeight"`
	FleeingLungeWeight             float64 `json:"fleeingLungeWeight"`
	LowResourceRiskFloor           float64 `json:"lowResourceRiskFloor"`
	DodgeCommittedThreatMultiplier float64 `json:"dodgeCommittedThreatMultiplier"`
	VulnerableBiteMultiplier       float64 `json:"vulnerableBiteMultiplier"`
	VulnerableMaulMultiplier       float64 `json:"vulnerableMaulMultiplier"`
	TacticalDestinationDistanceCM  float64 `json:"tacticalDestinationDistanceCm"`
}

type creatureRangePolicyJSON struct {
	DesiredRangeCM    float64 `json:"desiredRangeCm"`
	ChaseRangeCM      float64 `json:"chaseRangeCm"`
	RetreatRangeCM    float64 `json:"retreatRangeCm"`
	OrbitSpeedCMS     float64 `json:"orbitSpeedCmS"`
	ChaseSpeedCMS     float64 `json:"chaseSpeedCmS"`
	LungeSpeedCMS     float64 `json:"lungeSpeedCmS"`
	MaulSpeedCMS      float64 `json:"maulSpeedCmS"`
	RetreatSpeedCMS   float64 `json:"retreatSpeedCmS"`
	TurnRateDegPerSec float64 `json:"turnRateDegPerSec"`
}

type creatureStaminaPolicyJSON struct {
	Max                 float64 `json:"max"`
	DodgeCostMultiplier float64 `json:"dodgeCostMultiplier"`
	RegenPerSecond      float64 `json:"regenPerSecond"`
}

type creatureBehaviorMetadataJSON struct {
	Threat creatureThreatPolicyJSON `json:"threat"`
}

type creatureThreatPolicyJSON struct {
	DamageThreatPerPoint      float64 `json:"damageThreatPerPoint"`
	PostureThreatPerPoint     float64 `json:"postureThreatPerPoint"`
	ProximityThreatPerSec     float64 `json:"proximityThreatPerSec"`
	ProximityRangeCm          float64 `json:"proximityRangeCm"`
	FirstAggroBonus           float64 `json:"firstAggroBonus"`
	DecayPerSec               float64 `json:"decayPerSec"`
	LosBreakDecayMultiplier   float64 `json:"losBreakDecayMultiplier"`
	OutOfRangeDecayMultiplier float64 `json:"outOfRangeDecayMultiplier"`
	SwitchThresholdRatio      float64 `json:"switchThresholdRatio"`
	SwitchCooldownMs          int32   `json:"switchCooldownMs"`
	LeashDistanceCm           float64 `json:"leashDistanceCm"`
}

type movementActionContractMetadata struct {
	AbilityKey             string  `json:"ability_key"`
	AirborneDurationMS     int32   `json:"airborne_duration_ms"`
	VerticalMotionModel    string  `json:"vertical_motion_model"`
	JumpZVelocity          float64 `json:"jump_z_velocity"`
	GravityScale           float64 `json:"gravity_scale"`
	GravityZCMSS           float64 `json:"gravity_z_cm_s2"`
	ExpectedApexMS         int32   `json:"expected_apex_ms"`
	LandingDetectionPolicy string  `json:"landing_detection_policy"`
	GroundZPolicy          string  `json:"ground_z_policy"`
	CapsuleBaseOffset      float64 `json:"capsule_base_offset"`
	AllowsAirControl       bool    `json:"allows_air_control"`
	AirControlModifier     float64 `json:"air_control_modifier"`
	YawRateDegPerSec       float64 `json:"yaw_rate_deg_per_sec"`
	OrientationPolicyID    string  `json:"orientation_policy_id"`
	EnvelopePolicyID       string  `json:"envelope_policy_id"`
	PreCommitMS            int32   `json:"pre_commit_ms"`
	LandingInertiaMS       int32   `json:"landing_inertia_ms"`
}

const runtimeContractSourceDB = "db_contracts"
const runtimeContractSourceDBIncomplete = "db_contracts_incomplete"
const runtimeContractSourceDevFixture = "dev_test_fixture_contracts"
const runtimeContractSourceUnconfigured = "unconfigured_runtime_contracts"

const runtimeMovementReconciliationProfileID = "player_default_movement_profile"
const wolfRuntimeContractID = "contract_wolf_pack_harasser_v1"
const playerCombatCoreProfileID = "combat_core_player_sword_shield_v1"
const creatureCombatCoreProfileID = "combat_core_steppe_wolf"
const playerGuardDefenseContractID = "player_shield_guard_v1"
const creatureGuardDefenseContractID = "wolf_attack_vs_guard_v1"

func requiredBaseMovementActions() []struct {
	abilityKey string
	contractID string
} {
	requirements := runtimeRequirementsByCategory(runtimeRequirementBaseMovementAction)
	out := make([]struct {
		abilityKey string
		contractID string
	}, 0, len(requirements))
	for _, requirement := range requirements {
		out = append(out, struct {
			abilityKey string
			contractID string
		}{requirement.Key, requirement.ContractID})
	}
	return out
}

func requiredRuntimeSkillIDs() []string {
	requirements := runtimeRequirementsByCategory(runtimeRequirementSkill)
	out := make([]string, 0, len(requirements))
	for _, requirement := range requirements {
		out = append(out, requirement.Key)
	}
	return out
}

func LoadRuntimeContractsFromDB(ctx context.Context, skills ContractSource, profiles ProfileContractSource, creatures CreatureContractSource) RuntimeContracts {
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
		loaded, ok := loadSkillRuntimeContract(ctx, skills, profiles, skillID)
		if !ok {
			contracts.LoadIssues = append(contracts.LoadIssues, "missing skill runtime "+skillID)
			continue
		}
		contracts.SkillContracts[skillID] = loaded
		contracts.ActionContracts[skillID] = loaded.MovementAction
	}

	loadCombatRuntimeContracts(ctx, profiles, &contracts)
	loadCreatureTemplateRuntimeContracts(ctx, creatures, &contracts)
	loadWolfBrainRuntimeContracts(ctx, profiles, &contracts)

	if setupResp, err := profiles.GetCreatureSkillSetupPolicies(ctx, &dbv1.IdRequest{Id: contracts.WolfPolicy.ContractID}); err == nil && setupResp.GetFound() {
		contracts.WolfPolicy.SkillSetupPolicies = creatureSkillSetupPoliciesFromDB(setupResp.GetPolicies())
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
			contracts.WolfPolicy.EvasionLateralBias = evasion.GetLateralBias()
			contracts.WolfPolicy.EvasionBackstepBias = evasion.GetBackstepBias()
			contracts.WolfPolicy.EvasionPressureThreshold = evasion.GetPressureThreshold()
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
		contracts.Source = runtimeContractSourceDB
	}
	return contracts
}

func emptyDBRuntimeContracts() RuntimeContracts {
	return RuntimeContracts{
		Source:          runtimeContractSourceDBIncomplete,
		ActionContracts: map[string]MovementActionRuntimeContract{},
		SkillContracts:  map[string]SkillRuntimeContract{},
		CombatCore: CombatCoreRuntimeContracts{
			PlayerProfileID:   playerCombatCoreProfileID,
			CreatureProfileID: creatureCombatCoreProfileID,
			Profiles:          map[string]*dbv1.CombatCoreProfile{},
		},
		Defense: DefenseRuntimeContracts{
			PlayerGuardContractID:   playerGuardDefenseContractID,
			CreatureGuardContractID: creatureGuardDefenseContractID,
			Contracts:               map[string]*dbv1.CombatDefenseContract{},
		},
		WolfPolicy: WolfRuntimePolicy{
			ContractID: wolfRuntimeContractID,
			TemplateID: "steppe_wolf",
		},
	}
}

func loadCreatureTemplateRuntimeContracts(ctx context.Context, creatures CreatureContractSource, contracts *RuntimeContracts) {
	if contracts == nil {
		return
	}
	templateID := contracts.WolfPolicy.TemplateID
	if templateID == "" {
		templateID = "steppe_wolf"
		contracts.WolfPolicy.TemplateID = templateID
	}
	if creatures == nil {
		contracts.LoadIssues = append(contracts.LoadIssues, "missing creature data source "+templateID)
		return
	}
	resp, err := creatures.GetCreatureRuntimeData(ctx, &dbv1.IdRequest{Id: templateID})
	if err != nil || !resp.GetFound() || resp.GetTemplate() == nil {
		contracts.LoadIssues = append(contracts.LoadIssues, "missing creature runtime template "+templateID)
		return
	}
	template := resp.GetTemplate()
	contracts.WolfPolicy.TemplateID = template.GetId()
	contracts.WolfPolicy.ImpactResponseProfile = strings.TrimSpace(template.GetImpactResponseProfile())
	if contracts.WolfPolicy.ImpactResponseProfile == "" {
		contracts.LoadIssues = append(contracts.LoadIssues, "missing creature impact response profile "+templateID)
	}
}

func loadCombatRuntimeContracts(ctx context.Context, profiles ProfileContractSource, contracts *RuntimeContracts) {
	if contracts == nil || profiles == nil {
		return
	}
	for _, profileID := range []string{contracts.CombatCore.PlayerProfileID, contracts.CombatCore.CreatureProfileID} {
		if profileID == "" {
			continue
		}
		resp, err := profiles.GetCombatCoreProfile(ctx, &dbv1.IdRequest{Id: profileID})
		if err != nil || !resp.GetFound() || resp.GetProfile() == nil {
			contracts.LoadIssues = append(contracts.LoadIssues, "missing combat core profile "+profileID)
			continue
		}
		contracts.CombatCore.Profiles[profileID] = resp.GetProfile()
	}
	for _, contractID := range []string{contracts.Defense.PlayerGuardContractID, contracts.Defense.CreatureGuardContractID} {
		if contractID == "" {
			continue
		}
		resp, err := profiles.GetCombatDefenseContract(ctx, &dbv1.IdRequest{Id: contractID})
		if err != nil || !resp.GetFound() || resp.GetContract() == nil {
			contracts.LoadIssues = append(contracts.LoadIssues, "missing combat defense contract "+contractID)
			continue
		}
		contracts.Defense.Contracts[contractID] = resp.GetContract()
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
	if rangeJSON := strings.TrimSpace(behavior.GetRangePolicyJson()); rangeJSON != "" {
		var rangePolicy creatureRangePolicyJSON
		if err := json.Unmarshal([]byte(rangeJSON), &rangePolicy); err == nil {
			policy.DesiredRangeCM = rangePolicy.DesiredRangeCM
			policy.ChaseRangeCM = rangePolicy.ChaseRangeCM
			policy.RetreatRangeCM = rangePolicy.RetreatRangeCM
			policy.OrbitSpeedCMS = rangePolicy.OrbitSpeedCMS
			policy.ChaseSpeedCMS = rangePolicy.ChaseSpeedCMS
			policy.LungeSpeedCMS = rangePolicy.LungeSpeedCMS
			policy.MaulSpeedCMS = rangePolicy.MaulSpeedCMS
			policy.RetreatSpeedCMS = rangePolicy.RetreatSpeedCMS
			policy.TurnRateDegPerSec = rangePolicy.TurnRateDegPerSec
		}
	}
	if pressureJSON := strings.TrimSpace(behavior.GetPressurePolicyJson()); pressureJSON != "" {
		var pressurePolicy creaturePressurePolicyJSON
		if err := json.Unmarshal([]byte(pressureJSON), &pressurePolicy); err == nil {
			policy.RepeatSkillPenaltyMultiplier = pressurePolicy.RepeatSkillPenaltyMultiplier
			policy.DodgeUnderPressure = pressurePolicy.DodgeUnderPressure
			policy.MaulCounterUnderPressure = pressurePolicy.MaulCounterUnderPressure
			policy.MaulCounterChance = pressurePolicy.MaulCounterChance
			policy.DodgeRetreatMultiplier = pressurePolicy.DodgeRetreatMultiplier
			policy.GlobalDodgeMultiplier = pressurePolicy.GlobalDodgeMultiplier
			policy.CommitThreatWeight = pressurePolicy.CommitThreatWeight
			policy.ClosingThreatWeight = pressurePolicy.ClosingThreatWeight
			policy.DefensiveBiteWeight = pressurePolicy.DefensiveBiteWeight
			policy.FleeingLungeWeight = pressurePolicy.FleeingLungeWeight
			policy.LowResourceRiskFloor = pressurePolicy.LowResourceRiskFloor
			policy.DodgeCommittedThreatMultiplier = pressurePolicy.DodgeCommittedThreatMultiplier
			policy.VulnerableBiteMultiplier = pressurePolicy.VulnerableBiteMultiplier
			policy.VulnerableMaulMultiplier = pressurePolicy.VulnerableMaulMultiplier
			policy.TacticalDestinationDistanceCM = pressurePolicy.TacticalDestinationDistanceCM
		}
	}
	if metaJSON := strings.TrimSpace(behavior.GetMetadataJson()); metaJSON != "" {
		var meta creatureBehaviorMetadataJSON
		if err := json.Unmarshal([]byte(metaJSON), &meta); err == nil {
			t := meta.Threat
			policy.Threat = ThreatRuntimeProfile{
				DamageThreatPerPoint:      t.DamageThreatPerPoint,
				PostureThreatPerPoint:     t.PostureThreatPerPoint,
				ProximityThreatPerSec:     t.ProximityThreatPerSec,
				ProximityRangeCM:          t.ProximityRangeCm,
				FirstAggroBonus:           t.FirstAggroBonus,
				DecayPerSec:               t.DecayPerSec,
				LOSBreakDecayMultiplier:   t.LosBreakDecayMultiplier,
				OutOfRangeDecayMultiplier: t.OutOfRangeDecayMultiplier,
				SwitchThresholdRatio:      t.SwitchThresholdRatio,
				SwitchCooldownMS:          t.SwitchCooldownMs,
				LeashDistanceCM:           t.LeashDistanceCm,
			}
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

func creatureSkillSetupPoliciesFromDB(policies []*dbv1.CreatureSkillSetupPolicy) []CreatureSkillSetupRuntimePolicy {
	runtimePolicies := make([]CreatureSkillSetupRuntimePolicy, 0, len(policies))
	for _, policy := range policies {
		if policy == nil || !policy.GetIsEnabled() || policy.GetSkillId() == "" {
			continue
		}
		runtimePolicies = append(runtimePolicies, CreatureSkillSetupRuntimePolicy{
			ID:                  policy.GetId(),
			SkillID:             policy.GetSkillId(),
			SetupType:           policy.GetSetupType(),
			MinSetupMS:          policy.GetMinSetupMs(),
			MaxSetupMS:          policy.GetMaxSetupMs(),
			CommitDistanceCM:    policy.GetCommitDistanceCm(),
			PreferredMinRangeCM: policy.GetPreferredMinRangeCm(),
			PreferredMaxRangeCM: policy.GetPreferredMaxRangeCm(),
			MovementTactic:      policy.GetMovementTactic(),
			LockSideDuringSetup: policy.GetLockSideDuringSetup(),
			Enabled:             policy.GetIsEnabled(),
		})
	}
	return runtimePolicies
}

func (c RuntimeContracts) CoverageReport(strictLoadedSource bool) RuntimeContractCoverageReport {
	report := RuntimeContractCoverageReport{Source: c.Source, Strict: strictLoadedSource, Ready: true}
	report.addCategory("runtime_movement_profile", c.runtimeMovementProfileBlockers(strictLoadedSource))
	report.addCategory("base_movement_actions", c.baseMovementActionBlockers(strictLoadedSource))
	report.addCategory("skill_runtime_contracts", c.skillRuntimeContractBlockers(strictLoadedSource))
	report.addCategory("wolf_brain_policy", c.wolfBrainPolicyBlockers(strictLoadedSource))
	report.addCategory("combat_core_profiles", c.combatCoreProfileBlockers(strictLoadedSource))
	report.addCategory("combat_defense_contracts", c.combatDefenseContractBlockers(strictLoadedSource))
	report.addCategory("combat_mode_slots", c.combatModeSlotBlockers())
	report.addCategory("compat_runtime_surfaces", compatRuntimeSurfaceBlockers())
	if strictLoadedSource {
		report.addCategory("contract_load_issues", append([]string(nil), c.LoadIssues...))
	}
	return report
}

func (r *RuntimeContractCoverageReport) addCategory(name string, blockers []string) {
	category := RuntimeContractCoverageCategory{Name: name, Ready: len(blockers) == 0, Blockers: blockers}
	if !category.Ready {
		r.Ready = false
	}
	r.Categories = append(r.Categories, category)
}

func (r RuntimeContractCoverageReport) Blockers() []string {
	var blockers []string
	for _, category := range r.Categories {
		for _, blocker := range category.Blockers {
			if strings.TrimSpace(blocker) != "" {
				blockers = append(blockers, category.Name+": "+blocker)
			}
		}
	}
	return blockers
}

func (c RuntimeContracts) runtimeMovementProfileBlockers(strictLoadedSource bool) []string {
	if c.MovementProfile == nil {
		return []string{"movement reconciliation profile"}
	}
	if strictLoadedSource {
		return validateRuntimeMovementReconciliationProfile(c.MovementProfile)
	}
	return nil
}

func (c RuntimeContracts) baseMovementActionBlockers(strictLoadedSource bool) []string {
	var missing []string
	for _, ability := range requiredBaseMovementActions() {
		contract, ok := c.ActionContracts[ability.abilityKey]
		if !ok || contract.ID == "" {
			missing = append(missing, fmt.Sprintf("movement action %s", ability.abilityKey))
			continue
		}
		if contract.ReconciliationContractID == "" && contract.ReconciliationCategory == "" {
			missing = append(missing, fmt.Sprintf("movement action %s reconciliation", ability.abilityKey))
		}
		if strictLoadedSource {
			missing = append(missing, validateMovementActionRuntimeContract("movement action "+ability.abilityKey, contract, false)...)
		}
	}
	return missing
}

func (c RuntimeContracts) skillRuntimeContractBlockers(strictLoadedSource bool) []string {
	var missing []string
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
		if skill.MovementActionContractID != "" && skill.MovementAction.ID != "" && skill.MovementActionContractID != skill.MovementAction.ID {
			missing = append(missing, fmt.Sprintf("skill movement binding/action mismatch %s: %s != %s", skillID, skill.MovementActionContractID, skill.MovementAction.ID))
		}
		if skill.StartsAtPhase == "" {
			missing = append(missing, "skill movement starts phase "+skillID)
		}
		if skill.HandoffPolicy == "" {
			missing = append(missing, "skill movement handoff policy "+skillID)
		}
		if skill.NormalInputPolicy == "" {
			missing = append(missing, "skill movement normal input policy "+skillID)
		}
		if skill.MovementAction.ReconciliationContractID == "" && skill.MovementAction.ReconciliationCategory == "" {
			missing = append(missing, "skill movement reconciliation "+skillID)
		}
		if action, ok := c.ActionContracts[skillID]; !ok || action.ID == "" {
			missing = append(missing, "skill action manifest "+skillID)
		} else if skill.MovementActionContractID != "" && action.ID != skill.MovementActionContractID {
			missing = append(missing, fmt.Sprintf("skill action manifest mismatch %s: %s != %s", skillID, action.ID, skill.MovementActionContractID))
		}
		if strictLoadedSource {
			missing = append(missing, validateMovementActionRuntimeContract("skill movement "+skillID, skill.MovementAction, true)...)
			missing = append(missing, validateRuntimeSkillContract(skillID, skill)...)
		}
	}
	return missing
}

func (c RuntimeContracts) wolfBrainPolicyBlockers(strictLoadedSource bool) []string {
	var missing []string
	if c.WolfPolicy.ContractID == "" || c.WolfPolicy.DodgeSkillID == "" {
		missing = append(missing, "wolf runtime policy")
	}
	if strictLoadedSource {
		if c.WolfPolicy.TemplateID == "" {
			missing = append(missing, "wolf template id")
		}
		if c.WolfPolicy.ImpactResponseProfile == "" {
			missing = append(missing, "wolf impact response profile")
		}
		if c.WolfPolicy.TargetOpportunityPolicyID == "" {
			missing = append(missing, "wolf target opportunity policy")
		}
		if c.WolfPolicy.OrbitPolicyID == "" {
			missing = append(missing, "wolf orbit policy")
		}
		if len(c.WolfPolicy.SkillBehaviorBindings) == 0 {
			missing = append(missing, "wolf skill behavior bindings")
		}
		if len(c.WolfPolicy.SkillSetupPolicies) == 0 {
			missing = append(missing, "wolf skill setup policies")
		}
		for _, issue := range creatureai.ValidatePolicy(wolfBrainPolicy(c.WolfPolicy)) {
			missing = append(missing, "wolf brain policy "+issue)
		}
	}
	return missing
}

func (c RuntimeContracts) combatCoreProfileBlockers(strictLoadedSource bool) []string {
	var missing []string
	for _, profileID := range []string{c.CombatCore.PlayerProfileID, c.CombatCore.CreatureProfileID} {
		if profileID == "" {
			missing = append(missing, "combat core profile id")
			continue
		}
		profile := c.CombatCore.Profiles[profileID]
		if profile == nil {
			missing = append(missing, "combat core profile "+profileID)
			continue
		}
		if strictLoadedSource {
			missing = append(missing, validateCombatCoreProfile(profileID, profile)...)
		}
	}
	return missing
}

func (c RuntimeContracts) combatDefenseContractBlockers(strictLoadedSource bool) []string {
	var missing []string
	for _, contractID := range []string{c.Defense.PlayerGuardContractID, c.Defense.CreatureGuardContractID} {
		if contractID == "" {
			missing = append(missing, "combat defense contract id")
			continue
		}
		contract := c.Defense.Contracts[contractID]
		if contract == nil {
			missing = append(missing, "combat defense contract "+contractID)
			continue
		}
		if strictLoadedSource {
			missing = append(missing, validateCombatDefenseContract(contractID, contract)...)
		}
	}
	return missing
}

func (c RuntimeContracts) combatModeSlotBlockers() []string {
	if len(c.CombatModes) == 0 {
		return []string{"sword shield combat mode slots"}
	}
	var missing []string
	for _, slot := range c.CombatModes {
		if slot == nil || !slot.GetEnabled() || strings.TrimSpace(slot.GetSkillId()) == "" {
			continue
		}
		skillID := slot.GetSkillId()
		if skill, ok := c.SkillContracts[skillID]; !ok || skill.SkillID == "" || !skill.Enabled {
			missing = append(missing, fmt.Sprintf("combat mode slot %s:%d references missing runtime skill %s", slot.GetCombatModeId(), slot.GetSlotIndex(), skillID))
		}
	}
	if len(missing) > 0 {
		return missing
	}
	return nil
}

func (c RuntimeContracts) ValidateRequiredCoverage(strictLoadedSource bool) error {
	missing := c.CoverageReport(strictLoadedSource).Blockers()
	if len(missing) > 0 {
		return fmt.Errorf("runtime contract coverage incomplete: %s", strings.Join(missing, "; "))
	}
	return nil
}

func validateMovementActionRuntimeContract(label string, contract MovementActionRuntimeContract, skillOwned bool) []string {
	var missing []string
	if contract.ID == "" {
		missing = append(missing, label+" id")
	}
	if contract.AbilityKey == "" {
		missing = append(missing, label+" ability key")
	}
	if contract.ActionType == "" {
		missing = append(missing, label+" action type")
	}
	if movement.ActionDuration(contract) <= 0 {
		missing = append(missing, label+" duration")
	}
	if contract.ReconciliationContractID == "" && contract.ReconciliationCategory == "" {
		missing = append(missing, label+" reconciliation")
	}
	if contract.PhaseWindowPolicy == "" {
		missing = append(missing, label+" phase window policy")
	}
	if contract.PredictionErrorPolicy == "" {
		missing = append(missing, label+" prediction error policy")
	}
	if skillOwned {
		if contract.RootMotionOwner == "" {
			missing = append(missing, label+" root motion owner")
		}
		if contract.ContactPolicy == "" {
			missing = append(missing, label+" contact policy")
		}
	}
	return missing
}

func validateRuntimeSkillContract(skillID string, skill SkillRuntimeContract) []string {
	var missing []string
	if skill.SkillID == "" {
		missing = append(missing, "skill id "+skillID)
	}
	if skill.Damage < 0 {
		missing = append(missing, "skill damage negative "+skillID)
	}
	if skill.PostureDamage < 0 {
		missing = append(missing, "skill posture damage negative "+skillID)
	}
	if skill.Damage <= 0 && skill.PostureDamage <= 0 {
		return missing
	}
	if skill.MaxTargets <= 0 {
		missing = append(missing, "skill max targets "+skillID)
	}
	if len(skill.Hitboxes) == 0 {
		missing = append(missing, "skill temporal hitbox "+skillID)
		return missing
	}
	hasTemporal := false
	for _, profile := range skill.Hitboxes {
		if profile == nil {
			continue
		}
		if profile.GetDamageGroupId() == "" {
			missing = append(missing, "skill hitbox damage group "+skillID)
		}
		if profile.GetHitboxEndMs() > 0 && profile.GetHitboxEndMs() <= profile.GetHitboxStartMs() {
			missing = append(missing, "skill hitbox timing "+skillID)
		}
		motion := profile.GetMotionProfile()
		if motion == nil || !motion.GetEnabled() {
			continue
		}
		if motion.GetDamageGroupId() == "" {
			missing = append(missing, "skill motion damage group "+skillID)
		}
		if len(motion.GetSamples()) == 0 {
			missing = append(missing, "skill motion samples "+skillID)
			continue
		}
		for _, sample := range motion.GetSamples() {
			missing = append(missing, validateSkillMotionSampleGeometry(skillID, profile, motion, sample, skill.Range)...)
		}
		hasTemporal = true
	}
	if !hasTemporal {
		missing = append(missing, "skill temporal motion profile "+skillID)
	}
	controlEffects := enabledSkillControlEffects(skill.ControlEffects)
	if skillContactPolicyRequiresControlEffect(skill.ContactPolicy) && len(controlEffects) == 0 {
		missing = append(missing, "skill impact control effect "+skillID)
	}
	for _, effect := range controlEffects {
		missing = append(missing, validateSkillControlEffectContract(skillID, effect)...)
	}
	return missing
}

func skillContactPolicyRequiresControlEffect(policy string) bool {
	normalized := strings.ToLower(strings.TrimSpace(policy))
	return strings.Contains(normalized, "push") ||
		strings.Contains(normalized, "carry") ||
		strings.Contains(normalized, "knockback") ||
		strings.Contains(normalized, "lateral_counter") ||
		strings.Contains(normalized, "control")
}

func enabledSkillControlEffects(effects []*dbv1.SkillControlEffect) []*dbv1.SkillControlEffect {
	if len(effects) == 0 {
		return nil
	}
	out := make([]*dbv1.SkillControlEffect, 0, len(effects))
	for _, effect := range effects {
		if effect == nil || !effect.GetEnabled() || strings.TrimSpace(effect.GetStatusEffectId()) == "" {
			continue
		}
		out = append(out, effect)
	}
	return out
}

func validateSkillControlEffectContract(skillID string, effect *dbv1.SkillControlEffect) []string {
	if effect == nil || !effect.GetEnabled() {
		return nil
	}
	effectID := strings.TrimSpace(effect.GetId())
	if effectID == "" {
		effectID = strings.TrimSpace(effect.GetStatusEffectId())
	}
	if effectID == "" {
		effectID = "unnamed"
	}
	prefix := "skill control effect " + skillID + "/" + effectID + " "
	var missing []string
	if strings.TrimSpace(effect.GetControlType()) == "" {
		missing = append(missing, prefix+"control type")
	}
	if strings.TrimSpace(effect.GetReleasePolicyId()) == "" {
		missing = append(missing, prefix+"release policy")
	}
	if strings.TrimSpace(effect.GetDirectionPolicy()) == "" {
		missing = append(missing, prefix+"direction policy")
	}
	if effect.GetDurationMs() <= 0 {
		missing = append(missing, prefix+"duration")
	}
	if effect.GetDistanceCm() <= 0 {
		missing = append(missing, prefix+"distance")
	}
	if effect.GetSpeedCmS() <= 0 {
		missing = append(missing, prefix+"speed")
	}
	return missing
}

func validateSkillMotionSampleGeometry(skillID string, profile *dbv1.SkillHitboxProfile, motion *dbv1.SkillHitboxMotionProfile, sample *dbv1.SkillHitboxMotionSample, skillRange float64) []string {
	if profile == nil || motion == nil || sample == nil {
		return nil
	}
	sampleID := fmt.Sprintf("%d", sample.GetSampleIndex())
	prefix := "skill motion sample " + skillID + "/" + motion.GetId() + "/" + sampleID + " "
	shape := strings.ToLower(strings.TrimSpace(motion.GetSweepShape()))
	switch shape {
	case "arc_slice", "arc", "asymmetric_arc":
		reach := firstPositiveFloat64(sample.GetLength(), sample.GetRadius(), profile.GetLength(), profile.GetRadius(), skillRangeToCM(skillRange))
		if reach <= 0 {
			return []string{prefix + "reach"}
		}
		return nil
	case "box_strip", "box", "rectangle", "rect":
		lengthCM := firstPositiveFloat64(sample.GetSizeX(), sample.GetLength(), profile.GetSizeX(), profile.GetLength(), skillRangeToCM(skillRange))
		halfWidthCM := firstPositiveFloat64(sample.GetSizeY()/2, sample.GetRadius(), profile.GetSizeY()/2, profile.GetRadius())
		var missing []string
		if lengthCM <= 0 {
			missing = append(missing, prefix+"length")
		}
		if halfWidthCM <= 0 {
			missing = append(missing, prefix+"width")
		}
		return missing
	default:
		lengthCM := firstPositiveFloat64(sample.GetLength(), profile.GetLength(), skillRangeToCM(skillRange))
		radiusCM := firstPositiveFloat64(sample.GetRadius(), sample.GetSizeY()/2, profile.GetRadius(), profile.GetSizeY()/2)
		var missing []string
		if lengthCM <= 0 {
			missing = append(missing, prefix+"length")
		}
		if radiusCM <= 0 {
			missing = append(missing, prefix+"radius")
		}
		return missing
	}
}

func validateCombatCoreProfile(profileID string, profile *dbv1.CombatCoreProfile) []string {
	var missing []string
	if profile.GetDamageDealtMultiplier() <= 0 {
		missing = append(missing, "combat core damage dealt "+profileID)
	}
	if profile.GetDamageTakenMultiplier() <= 0 {
		missing = append(missing, "combat core damage taken "+profileID)
	}
	if profile.GetMaxPosture() <= 0 {
		missing = append(missing, "combat core max posture "+profileID)
	}
	if profile.GetPostureDamageMultiplier() <= 0 {
		missing = append(missing, "combat core posture damage "+profileID)
	}
	if profile.GetBlockDamageReduction() < 0 {
		missing = append(missing, "combat core block reduction "+profileID)
	}
	if profile.GetParryRewardMultiplier() <= 0 {
		missing = append(missing, "combat core parry reward "+profileID)
	}
	if profile.GetMaxStamina() <= 0 {
		missing = append(missing, "combat core max stamina "+profileID)
	}
	if profile.GetStaminaRegenPerSec() <= 0 {
		missing = append(missing, "combat core stamina regen "+profileID)
	}
	if profile.GetSprintStaminaCostPerSec() <= 0 {
		missing = append(missing, "combat core sprint stamina drain "+profileID)
	}
	if profile.GetStaminaZeroRegenMultiplier() <= 0 || profile.GetStaminaZeroRegenMultiplier() > 1 {
		missing = append(missing, "combat core zero stamina regen multiplier "+profileID)
	}
	return missing
}

func validateCombatDefenseContract(contractID string, contract *dbv1.CombatDefenseContract) []string {
	var missing []string
	if contract.GetId() == "" {
		missing = append(missing, "combat defense id "+contractID)
	}
	if contract.GetDefenseType() == "" {
		missing = append(missing, "combat defense type "+contractID)
	}
	if contract.GetFrontalArcDeg() <= 0 {
		missing = append(missing, "combat defense frontal arc "+contractID)
	}
	if contract.GetGuardDamageMultiplier() <= 0 {
		missing = append(missing, "combat defense guard multiplier "+contractID)
	}
	return missing
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

func loadSkillRuntimeContract(ctx context.Context, skills ContractSource, profiles ProfileContractSource, skillID string) (SkillRuntimeContract, bool) {
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
	if impactResp, err := skills.GetSkillImpactProfile(ctx, &dbv1.IdRequest{Id: skillID}); err == nil && impactResp.GetFound() {
		impact := impactResp.GetProfile()
		contract.Impact = impact
		if impact != nil {
			contract.ControlEffects = enabledSkillControlEffects(impact.GetControlEffects())
		}
	}
	if hitboxResp, err := skills.GetSkillHitboxProfiles(ctx, &dbv1.IdRequest{Id: skillID}); err == nil && hitboxResp.GetFound() {
		contract.Hitboxes = hitboxResp.GetProfiles()
	}
	if profiles != nil {
		if policyResp, err := profiles.GetSkillActionPolicyBinding(ctx, &dbv1.IdRequest{Id: skillID}); err == nil && policyResp.GetFound() {
			contract.ActionPolicy = policyResp.GetBinding()
			if contract.ActionPolicy != nil {
				contract.Orientation = contract.ActionPolicy.GetActionOrientationPolicy()
				contract.Envelope = contract.ActionPolicy.GetActionEnvelopePolicy()
			}
		}
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

// DevFixtureRuntimeContracts is a dev/test fixture. Normal app
// boot must load DB-backed contracts through LoadRuntimeContractsFromDB and strict
// coverage validation; do not call this from production startup.
func DevFixtureRuntimeContracts() RuntimeContracts {
	contracts := RuntimeContracts{
		Source:          runtimeContractSourceDevFixture,
		MovementProfile: fixtureMovementProfile(),
		ActionContracts: map[string]MovementActionRuntimeContract{},
		SkillContracts:  map[string]SkillRuntimeContract{},
		CombatCore: CombatCoreRuntimeContracts{
			PlayerProfileID:   playerCombatCoreProfileID,
			CreatureProfileID: creatureCombatCoreProfileID,
			Profiles: map[string]*dbv1.CombatCoreProfile{
				playerCombatCoreProfileID:   fixturePlayerCombatCoreProfile(),
				creatureCombatCoreProfileID: fixtureCreatureCombatCoreProfile(),
			},
		},
		Defense: DefenseRuntimeContracts{
			PlayerGuardContractID:   playerGuardDefenseContractID,
			CreatureGuardContractID: creatureGuardDefenseContractID,
			Contracts: map[string]*dbv1.CombatDefenseContract{
				playerGuardDefenseContractID:   fixturePlayerGuardDefenseContract(),
				creatureGuardDefenseContractID: fixtureCreatureGuardDefenseContract(),
			},
		},
		WolfPolicy: WolfRuntimePolicy{
			ContractID:                     wolfRuntimeContractID,
			ContractHash:                   wolfRuntimeContractID,
			CapabilityID:                   "wolf_pack_harasser",
			TemplateID:                     "steppe_wolf",
			ImpactResponseProfile:          "creature_flesh_blood_red",
			DesiredRangeCM:                 560,
			ChaseRangeCM:                   860,
			LungeRangeCM:                   220,
			RetreatRangeCM:                 340,
			OrbitSpeedCMS:                  150,
			ChaseSpeedCMS:                  310,
			LungeSpeedCMS:                  380,
			MaulSpeedCMS:                   345,
			RetreatSpeedCMS:                260,
			LungeWindupMS:                  3600,
			LungeActiveEndMS:               3980,
			LungeRecoveryMS:                520,
			LungeDistanceCM:                918,
			LungeDurationMS:                860,
			LungeArcHeightCM:               120,
			DodgeSkillID:                   "wolf_dodge",
			EvasionChainCount:              4,
			EvasionLateralBias:             0.72,
			EvasionBackstepBias:            0.28,
			EvasionPressureThreshold:       0.42,
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
			DodgeUnderPressure:             true,
			MaulCounterUnderPressure:       true,
			MaulCounterChance:              0.30,
			DodgeRetreatMultiplier:         0.70,
			GlobalDodgeMultiplier:          0.85,
			CommitThreatWeight:             0.28,
			ClosingThreatWeight:            0.18,
			DefensiveBiteWeight:            0.14,
			FleeingLungeWeight:             0.20,
			LowResourceRiskFloor:           0.16,
			DodgeCommittedThreatMultiplier: 1.12,
			VulnerableBiteMultiplier:       1.16,
			VulnerableMaulMultiplier:       1.16,
			TacticalDestinationDistanceCM:  180,
			TargetMemoryMS:                 5200,
			NoReadySkillMemoryPolicy:       "observe_only",
			CandidateCooldownVisibility:    true,
			AllowBacksideCommit:            true,
			OrbitPolicyID:                  "orbit_wolf_harasser_combat_walk_v1",
			OrbitLocomotionMode:            "combat_walk",
			OrbitSpeedScale:                0.75,
			MinOrbitDurationMS:             2600,
			SideSwitchCooldownMS:           2600,
			AllowSideSwitchWhenTargetFaces: true,
			PreferLongSideCommit:           true,
			SideFlipChanceMultiplier:       0.55,
			LockSideDuringSetup:            true,
			RepeatSkillPenaltyWindowMS:     5200,
			RepeatSkillPenaltyMultiplier:   0.35,
			DodgeStaminaCostMultiplier:     0.5,
			StaminaRegenPerSecond:          12,
			MaxStamina:                     100,
			SkillBehaviorBindings: []CreatureSkillBehaviorRuntimeBinding{
				{ID: "fixture_bind_bite_circle", SkillID: "bite", TacticalState: "circle", DecisionPhase: "reposition", MinRangeCM: 0, MaxRangeCM: 300, Priority: 70, UsageWeight: 0.85, CooldownGroup: "wolf_bite", RequiresLineOfSight: true, Enabled: true},
				{ID: "fixture_bind_lunge_circle", SkillID: "lunge", TacticalState: "circle", DecisionPhase: "reposition", SetupPolicyID: "wolf_lunge_flank_windup_v1", MinRangeCM: 180, MaxRangeCM: 980, Priority: 84, UsageWeight: 0.42, CooldownGroup: "wolf_lunge", RequiresLineOfSight: true, Enabled: true},
				{ID: "fixture_bind_lunge_approach", SkillID: "lunge", TacticalState: "approach", DecisionPhase: "acquire", SetupPolicyID: "wolf_lunge_chase_windup_v1", MinRangeCM: 420, MaxRangeCM: 1400, Priority: 86, UsageWeight: 0.50, CooldownGroup: "wolf_lunge", RequiresLineOfSight: true, Enabled: true},
				{ID: "fixture_bind_maul_pressure", SkillID: "maul", TacticalState: "pressure", DecisionPhase: "counter", SetupPolicyID: "wolf_maul_pressure_counter_v1", MinRangeCM: 0, MaxRangeCM: 260, Priority: 100, UsageWeight: 0.9, CooldownGroup: "wolf_maul", RequiresLineOfSight: true, Enabled: true},
				{ID: "fixture_bind_dodge_pressure", SkillID: "wolf_dodge", TacticalState: "pressure", DecisionPhase: "evade", MinRangeCM: 0, MaxRangeCM: 420, Priority: 110, UsageWeight: 1.15, CooldownGroup: "wolf_dodge", RequiresLineOfSight: false, Enabled: true},
			},
			SkillSetupPolicies: []CreatureSkillSetupRuntimePolicy{
				{ID: "wolf_lunge_flank_windup_v1", SkillID: "lunge", SetupType: "moving_windup", MinSetupMS: 3000, MaxSetupMS: 4200, CommitDistanceCM: 760, PreferredMinRangeCM: 180, PreferredMaxRangeCM: 980, MovementTactic: "circle_then_curve_to_target", LockSideDuringSetup: true, Enabled: true},
				{ID: "wolf_lunge_chase_windup_v1", SkillID: "lunge", SetupType: "chase_windup", MinSetupMS: 1200, MaxSetupMS: 2600, CommitDistanceCM: 900, PreferredMinRangeCM: 520, PreferredMaxRangeCM: 1400, MovementTactic: "run_chase_then_jump", LockSideDuringSetup: false, Enabled: true},
				{ID: "wolf_maul_pressure_counter_v1", SkillID: "maul", SetupType: "pressure_counter", MinSetupMS: 160, MaxSetupMS: 420, CommitDistanceCM: 220, PreferredMinRangeCM: 0, PreferredMaxRangeCM: 260, MovementTactic: "lateral_counter_dash", LockSideDuringSetup: true, Enabled: true},
			},
			Threat: ThreatRuntimeProfile{
				DamageThreatPerPoint:      1.0,
				PostureThreatPerPoint:     0.8,
				ProximityThreatPerSec:     2.0,
				ProximityRangeCM:          400,
				FirstAggroBonus:           25,
				DecayPerSec:               6.0,
				LOSBreakDecayMultiplier:   3.0,
				OutOfRangeDecayMultiplier: 2.0,
				SwitchThresholdRatio:      1.25,
				SwitchCooldownMS:          1500,
				LeashDistanceCM:           3500,
			},
		},
		CombatModes: fixtureCombatModeSlots(),
	}
	for _, contract := range []MovementActionRuntimeContract{
		{ID: "grounded_move_v1", AbilityKey: "move", ActionType: "move", DurationMS: 180, ActiveMS: 120, RecoveryMS: 60, ReconciliationContractID: "grounded_move_reconciliation", ReconciliationCategory: "grounded_move_reconciliation", PhaseWindowPolicy: "server_authoritative", PredictionErrorPolicy: "bounded_smooth_correction"},
		{ID: "turn_v1_rate_limited_contextual", AbilityKey: "turn", ActionType: "turn", DurationMS: 180, ActiveMS: 120, RecoveryMS: 60, YawRateDegPerSec: 720, ReconciliationContractID: "turn_reconciliation", ReconciliationCategory: "turn_reconciliation", PhaseWindowPolicy: "server_authoritative", PredictionErrorPolicy: "bounded_smooth_correction"},
		{ID: "dodge_v1_full_iframe", AbilityKey: "dodge", ActionType: "dodge", DurationMS: 320, ActiveMS: 260, RecoveryMS: 60, DistanceCM: 360, BaseSpeedCMS: 1125, SpeedCurveSamples: fixtureMovementCurve("dodge_v1_full_iframe"), VerticalCurveSamples: fixtureVerticalCurve("dodge_v1_full_iframe"), ReconciliationContractID: "dodge_reconciliation", ReconciliationCategory: "dodge_reconciliation", PhaseWindowPolicy: "server_authoritative", PredictionErrorPolicy: "bounded_smooth_correction"},
		{ID: "jump_v1_authoritative_grounded_handoff", AbilityKey: "jump", ActionType: "leap", DurationMS: 980, AirborneDurationMS: 980, ActiveMS: 920, RecoveryMS: 60, DistanceCM: 280, BaseSpeedCMS: 285.7, SpeedCurveSamples: fixtureMovementCurve("jump_v1_authoritative_grounded_handoff"), VerticalCurveSamples: fixtureVerticalCurve("jump_v1_authoritative_grounded_handoff"), VerticalMotionModel: "ballistic", JumpZVelocity: 480, GravityScale: 1, GravityZCMSS: movement.DefaultUnrealGravityZCMSS, ExpectedApexMS: 490, LandingDetectionPolicy: "server_grounded_handoff", GroundZPolicy: "server_position_is_actor_root", AllowsAirControl: true, AirControlModifier: 0.35, ReconciliationContractID: "leap_reconciliation", ReconciliationCategory: "leap_reconciliation", PhaseWindowPolicy: "server_authoritative", PredictionErrorPolicy: "bounded_smooth_correction"},
	} {
		contracts.ActionContracts[contract.AbilityKey] = contract
	}
	for _, skill := range []SkillRuntimeContract{
		fixtureSkillContract("player_basic_attack_1", 84, 350, 140, 120),
		fixtureSkillContract("player_basic_attack_2", 42, 370, 150, 120),
		fixtureSkillContract("player_basic_attack_3", 252, 620, 260, 180),
		fixtureSkillContract("player_shield_bash", 95, 300, 170, 120),
		fixtureSkillContract("player_shield_rush", 864, 1100, 720, 260),
		fixtureCreatureSkillContract("bite", "wolf_bite_melee_commit_v1", "grounded_skill", "grounded_skill_action_reconciliation", "melee_contact", 0, 520, 220, 180, 120, 180, 900),
		fixtureCreatureSkillContract("lunge", "low_fast_lunge_v1", "leap", "leap_reconciliation", "airborne_passthrough", 1652, 860, 380, 180, 3600, 520, 9000),
		fixtureCreatureSkillContract("wolf_dodge", "wolf_dodge_lateral_leap_v1", "dodge", "dodge_reconciliation", "iframe", 210, 520, 420, 100, 0, 100, 0),
		fixtureCreatureSkillContract("maul", "wolf_maul_lateral_counter_v1", "grounded_skill", "grounded_skill_action_reconciliation", "lateral_counter_contact", 420, 920, 520, 220, 220, 220, 5200),
	} {
		contracts.SkillContracts[skill.SkillID] = skill
		contracts.ActionContracts[skill.SkillID] = skill.MovementAction
	}
	return contracts
}

func fixturePlayerCombatCoreProfile() *dbv1.CombatCoreProfile {
	return &dbv1.CombatCoreProfile{
		DamageDealtMultiplier:      1,
		DamageTakenMultiplier:      1,
		CanBlock:                   true,
		BlockDamageReduction:       1,
		MaxPosture:                 100,
		PostureDamageMultiplier:    1,
		PostureBreakDurationMs:     2200,
		CanParry:                   true,
		ParryWindowMs:              220,
		ParryRewardMultiplier:      1.4,
		DodgeIframeMs:              320,
		MaxStamina:                 100,
		StaminaRegenPerSec:         14,
		DodgeStaminaCost:           24,
		SprintStaminaCostPerSec:    7,
		BlockStaminaCostPerSec:     2,
		AttackStaminaCost:          0,
		StaminaZeroRegenMultiplier: 0.5,
	}
}

func fixtureCreatureCombatCoreProfile() *dbv1.CombatCoreProfile {
	return &dbv1.CombatCoreProfile{
		DamageDealtMultiplier:      0.95,
		DamageTakenMultiplier:      1.05,
		CanBlock:                   false,
		BlockDamageReduction:       0,
		MaxPosture:                 65,
		PostureDamageMultiplier:    1.15,
		PostureBreakDurationMs:     1800,
		CanParry:                   false,
		ParryWindowMs:              0,
		ParryRewardMultiplier:      1,
		DodgeIframeMs:              220,
		MaxStamina:                 100,
		StaminaRegenPerSec:         12,
		DodgeStaminaCost:           24,
		SprintStaminaCostPerSec:    5,
		BlockStaminaCostPerSec:     0,
		AttackStaminaCost:          0,
		StaminaZeroRegenMultiplier: 0.5,
	}
}

func fixturePlayerGuardDefenseContract() *dbv1.CombatDefenseContract {
	return &dbv1.CombatDefenseContract{
		Id:                         playerGuardDefenseContractID,
		Name:                       "Player Shield Guard",
		Description:                "Dev fixture frontal shield guard.",
		DefenseType:                "shield_block",
		FrontalArcDeg:              120,
		DefenderMarginLeftRatio:    0.30,
		DefenderMarginRightRatio:   0.30,
		StaminaDamageOnlyOnBlock:   true,
		HealthDamageOnUnblockedHit: true,
		PostureDamageOnBlock:       true,
		GuardDamageMultiplier:      1,
		BlockStaminaDrainPerSecond: 2,
		MetadataJson:               `{"source":"dev_fixture","frontFacing":"control_rotation_yaw"}`,
	}
}

func fixtureCreatureGuardDefenseContract() *dbv1.CombatDefenseContract {
	return &dbv1.CombatDefenseContract{
		Id:                         creatureGuardDefenseContractID,
		Name:                       "Wolf Attack Vs Guard",
		Description:                "Dev fixture creature incoming melee guard interaction.",
		DefenseType:                "incoming_melee",
		FrontalArcDeg:              120,
		DefenderMarginLeftRatio:    0.30,
		DefenderMarginRightRatio:   0.30,
		StaminaDamageOnlyOnBlock:   true,
		HealthDamageOnUnblockedHit: true,
		PostureDamageOnBlock:       true,
		GuardDamageMultiplier:      1,
		MetadataJson:               `{"source":"dev_fixture"}`,
	}
}

// fixturePlayerSkillDamage returns the authoritative base/posture damage for a player
// skill, taken verbatim from db-apeiron bootstrap/013_player_sword_shield_skill_seed.sql.
// This materializes the canonical seed values in the fixture runtime for tests until the DB skill
// proto exposes base_damage/posture_damage (damage-pipeline brick 2b), at which point
// loadSkillRuntimeContract should source them from the DB instead.
func fixturePlayerSkillDamage(skillID string) (damage float64, posture float64) {
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

func fixturePlayerSkillMaxTargets(skillID string) int32 {
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

func fixturePlayerSkillHitboxes(skillID string) []*dbv1.SkillHitboxProfile {
	targetType := "enemy"
	maxTargets := fixturePlayerSkillMaxTargets(skillID)
	profile := &dbv1.SkillHitboxProfile{
		Id:                  skillID + "_fixture_temporal_hitbox",
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
		profile.Length, profile.Angle, profile.Radius = 84, 90, 42
	case "player_basic_attack_2":
		profile.HitboxStartMs, profile.HitboxEndMs = 100, 250
		profile.Length, profile.Angle, profile.Radius = 135, 90, 52
	case "player_basic_attack_3":
		profile.HitboxStartMs, profile.HitboxEndMs = 180, 440
		profile.Length, profile.Angle, profile.Radius = 252, 95, 42
	case "player_shield_bash":
		profile.HitboxStartMs, profile.HitboxEndMs = 110, 280
		profile.Length, profile.Radius = 210, 95
	case "player_shield_rush":
		profile.HitboxStartMs, profile.HitboxEndMs = 160, 880
		profile.Length, profile.Radius = 315, 96
	default:
		return nil
	}
	profile.MotionProfile = fixturePlayerSkillHitboxMotionProfile(skillID)
	profile.DamageGroupId = fixturePlayerSkillHitboxDamageGroupID(skillID)
	return []*dbv1.SkillHitboxProfile{profile}
}

func fixturePlayerSkillHitboxDamageGroupID(skillID string) string {
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

func fixturePlayerSkillHitboxMotionProfile(skillID string) *dbv1.SkillHitboxMotionProfile {
	id, sweepShape, samples := fixturePlayerSkillHitboxMotionSamples(skillID)
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
		DamageGroupId: fixturePlayerSkillHitboxDamageGroupID(skillID),
		Samples:       samples,
	}
}

func fixturePlayerSkillHitboxMotionSamples(skillID string) (string, string, []*dbv1.SkillHitboxMotionSample) {
	switch skillID {
	case "player_basic_attack_1":
		return "motion_player_basic_attack_1_forward_v1", "box_strip", []*dbv1.SkillHitboxMotionSample{
			fixtureHitboxMotionSample(0, 0.00, 0, 0, 90, 28, 52, 150, 26, 28, 0, 0),
			fixtureHitboxMotionSample(1, 0.50, 0, 0, 90, 46, 52, 150, 26, 46, 0, 0),
			fixtureHitboxMotionSample(2, 1.00, 0, 0, 90, 64, 52, 150, 26, 64, 0, 0),
		}
	case "player_basic_attack_2":
		return "motion_player_basic_attack_2_right_to_left_v1", "arc_slice", []*dbv1.SkillHitboxMotionSample{
			fixtureHitboxMotionSample(0, 0.00, 70, -35, 95, 0, 0, 150, 50, 125, 15, 45),
			fixtureHitboxMotionSample(1, 0.50, 80, 0, 95, 0, 0, 150, 52, 135, -15, 15),
			fixtureHitboxMotionSample(2, 1.00, 70, 35, 95, 0, 0, 150, 50, 125, -45, -15),
		}
	case "player_basic_attack_3":
		return "motion_player_basic_attack_3_shield_drive_v1", "capsule_strip", []*dbv1.SkillHitboxMotionSample{
			fixtureHitboxMotionSample(0, 0.00, 0, 0, 95, 84, 0, 155, 42, 42, 0, 0),
			fixtureHitboxMotionSample(1, 0.55, 0, 0, 95, 84, 0, 155, 42, 140, 0, 0),
			fixtureHitboxMotionSample(2, 1.00, 0, 0, 95, 84, 0, 155, 42, 252, 0, 0),
		}
	case "player_shield_bash":
		return "motion_player_shield_bash_front_push_v1", "capsule_strip", []*dbv1.SkillHitboxMotionSample{
			fixtureHitboxMotionSample(0, 0.00, 45, 0, 95, 132, 0, 160, 66, 75, 0, 0),
			fixtureHitboxMotionSample(1, 0.50, 72, 0, 95, 132, 0, 160, 66, 120, 0, 0),
			fixtureHitboxMotionSample(2, 1.00, 92, 0, 95, 132, 0, 160, 66, 160, 0, 0),
		}
	case "player_shield_rush":
		return "motion_player_shield_rush_front_contact_v1", "box_strip", []*dbv1.SkillHitboxMotionSample{
			fixtureHitboxMotionSample(0, 0.00, 8, 0, 100, 34, 224, 160, 112, 34, 0, 0),
			fixtureHitboxMotionSample(1, 0.50, 10, 0, 100, 44, 224, 160, 112, 44, 0, 0),
			fixtureHitboxMotionSample(2, 1.00, 12, 0, 100, 54, 224, 160, 112, 54, 0, 0),
		}
	default:
		return "", "", nil
	}
}

func fixtureHitboxMotionSample(index int32, t float64, offsetX, offsetY, offsetZ, sizeX, sizeY, sizeZ, radius, length, startAngleDeg, endAngleDeg float64) *dbv1.SkillHitboxMotionSample {
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

func fixtureMovementCurve(contractID string) []movement.MovementActionCurvePoint {
	switch contractID {
	case "dodge_v1_full_iframe":
		return []movement.MovementActionCurvePoint{curvePoint(0, 0.35), curvePoint(0.35, 1), curvePoint(1, 0.2)}
	case "jump_v1_authoritative_grounded_handoff":
		return []movement.MovementActionCurvePoint{curvePoint(0, 0.35), curvePoint(0.35, 0.95), curvePoint(1, 0.62)}
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
		return []movement.MovementActionCurvePoint{curvePoint(0, 0.35), curvePoint(0.16, 1), curvePoint(0.68, 0.92), curvePoint(1, 0.38)}
	case "wolf_dodge":
		return []movement.MovementActionCurvePoint{curvePoint(0, 0.4), curvePoint(0.35, 1), curvePoint(1, 0.2)}
	case "maul":
		return []movement.MovementActionCurvePoint{curvePoint(0, 0.05), curvePoint(0.28, 0.85), curvePoint(0.62, 1), curvePoint(1, 0.18)}
	default:
		return nil
	}
}

func fixtureVerticalCurve(contractID string) []movement.MovementActionCurvePoint {
	switch contractID {
	case "dodge_v1_full_iframe":
		return []movement.MovementActionCurvePoint{curvePoint(0, 0), curvePoint(0.4, 18), curvePoint(1, 0)}
	case "jump_v1_authoritative_grounded_handoff":
		return []movement.MovementActionCurvePoint{curvePoint(0, 0), curvePoint(0.46, 110), curvePoint(1, 0)}
	case "lunge":
		return []movement.MovementActionCurvePoint{curvePoint(0, 0), curvePoint(0.22, 18), curvePoint(1, 0)}
	case "wolf_dodge":
		return []movement.MovementActionCurvePoint{curvePoint(0, 0), curvePoint(0.4, 28), curvePoint(1, 0)}
	default:
		return nil
	}
}

func fixtureSkillContract(skillID string, distance float64, durationMS, activeMS, recoveryMS int32) SkillRuntimeContract {
	contactPolicy := fixturePlayerSkillContactPolicy(skillID)
	action := MovementActionRuntimeContract{
		ID:                       fixturePlayerSkillMovementActionContractID(skillID),
		AbilityKey:               skillID,
		ActionType:               "grounded_skill",
		DurationMS:               durationMS,
		ActiveMS:                 activeMS,
		RecoveryMS:               recoveryMS,
		DistanceCM:               distance,
		BaseSpeedCMS:             fixturePlayerSkillBaseSpeedCMS(skillID),
		SpeedCurveSamples:        fixtureMovementCurve(skillID),
		ReconciliationContractID: "grounded_skill_action_reconciliation",
		// Published reconciliation_mode MUST be a string the Unreal client recognizes
		// (ApeironReconciliationModeFromServerString). The category is the wire mode
		// "grounded_skill_action" -> EApeironPlayerReconciliationMode::SkillGroundedAction.
		// The verbose "_reconciliation" form parsed as None and made player skills rubberband.
		ReconciliationCategory: "grounded_skill_action",
		PhaseWindowPolicy:      "server_authoritative",
		PredictionErrorPolicy:  "bounded_smooth_correction",
		RootMotionOwner:        "skill",
		ContactPolicy:          contactPolicy,
	}
	damage, posture := fixturePlayerSkillDamage(skillID)
	impact := fixtureSkillImpactProfile(skillID, posture)
	return SkillRuntimeContract{
		SkillID:                  skillID,
		MovementActionContractID: action.ID,
		MovementAction:           action,
		Damage:                   damage,
		PostureDamage:            posture,
		MaxTargets:               fixturePlayerSkillMaxTargets(skillID),
		Blockable:                true,
		Impact:                   impact,
		ControlEffects:           enabledSkillControlEffects(impact.GetControlEffects()),
		Hitboxes:                 fixturePlayerSkillHitboxes(skillID),
		ActiveMS:                 activeMS,
		RecoveryMS:               recoveryMS,
		CooldownMS:               fixturePlayerSkillCooldownMS(skillID),
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

func fixturePlayerSkillCooldownMS(skillID string) int32 {
	switch skillID {
	case "player_shield_bash":
		return 26000
	case "player_shield_rush":
		return 32000
	default:
		return 0
	}
}

func fixturePlayerSkillBaseSpeedCMS(skillID string) float64 {
	switch skillID {
	case "player_basic_attack_1":
		return 240
	case "player_basic_attack_2":
		return 114
	case "player_basic_attack_3":
		return 406.4
	case "player_shield_bash":
		return 541
	case "player_shield_rush":
		return 1033.2
	default:
		return 0
	}
}

func fixturePlayerSkillMovementActionContractID(skillID string) string {
	switch skillID {
	case "player_basic_attack_1":
		return "basic_attack_1_forward_cut_v1"
	case "player_basic_attack_2":
		return "basic_attack_2_cross_cut_v1"
	case "player_basic_attack_3":
		return "basic_attack_3_shield_drive_v1"
	case "player_shield_bash":
		return "shield_bash_front_push_v1"
	case "player_shield_rush":
		return "shield_rush_front_contact_v1"
	default:
		return skillID + "_contract"
	}
}

func fixturePlayerSkillContactPolicy(skillID string) string {
	switch skillID {
	case "player_basic_attack_3":
		return "carry_contact_forward_release"
	case "player_shield_bash":
		return "multi_target_push_forward_release"
	case "player_shield_rush":
		return "multi_target_carry_push_forward_release"
	default:
		return "authoritative_contact"
	}
}

func fixtureSkillImpactProfile(skillID string, posture float64) *dbv1.SkillImpactProfile {
	impactType := "physical"
	if strings.Contains(skillID, "shield") || skillID == "player_basic_attack_3" {
		impactType = "blunt"
	}
	profile := &dbv1.SkillImpactProfile{
		SkillId:               skillID,
		ImpactType:            impactType,
		PoiseDamage:           posture,
		GuardDamageMultiplier: 1,
	}
	if control := fixtureSkillControlEffect(skillID); control != nil {
		profile.ControlEffects = []*dbv1.SkillControlEffect{control}
	}
	return profile
}

func fixtureSkillControlEffect(skillID string) *dbv1.SkillControlEffect {
	switch skillID {
	case "player_basic_attack_3":
		return &dbv1.SkillControlEffect{
			Id:              "player_basic_attack_3_impact_control",
			Enabled:         true,
			StatusEffectId:  "impact_shield_drive_push",
			DurationMs:      180,
			ControlType:     "push",
			ReleasePolicyId: "carry_contact_forward_release",
			DistanceCm:      252,
			SpeedCmS:        fixtureControlSpeedCMS(252, 180),
			DirectionPolicy: "source_forward",
		}
	case "player_shield_bash":
		return &dbv1.SkillControlEffect{
			Id:              "player_shield_bash_impact_control",
			Enabled:         true,
			StatusEffectId:  "impact_shield_bash_push",
			DurationMs:      170,
			ControlType:     "push",
			ReleasePolicyId: "multi_target_push_forward_release",
			DistanceCm:      95,
			SpeedCmS:        fixtureControlSpeedCMS(95, 170),
			DirectionPolicy: "source_forward",
		}
	case "player_shield_rush":
		return &dbv1.SkillControlEffect{
			Id:              "player_shield_rush_impact_control",
			Enabled:         true,
			StatusEffectId:  "impact_shield_rush_carry_push",
			DurationMs:      720,
			ControlType:     "carry_push",
			ReleasePolicyId: "multi_target_carry_push_forward_release",
			DistanceCm:      864,
			SpeedCmS:        fixtureControlSpeedCMS(864, 720),
			DirectionPolicy: "source_forward",
		}
	case "maul":
		return &dbv1.SkillControlEffect{
			Id:              "maul_impact_control",
			Enabled:         true,
			StatusEffectId:  "impact_wolf_maul_lateral_grab",
			DurationMs:      520,
			ControlType:     "grab",
			ReleasePolicyId: "lateral_grab_release",
			DistanceCm:      420,
			SpeedCmS:        690,
			DirectionPolicy: "source_action_direction",
		}
	default:
		return nil
	}
}

func fixtureControlSpeedCMS(distanceCM float64, durationMS int32) float64 {
	if distanceCM <= 0 || durationMS <= 0 {
		return 0
	}
	return distanceCM / (float64(durationMS) / 1000.0)
}

func fixtureCreatureSkillStaminaCost(skillID string) float64 {
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

func fixtureCreatureSkillDamage(skillID string) (damage float64, posture float64) {
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

func fixtureCreatureSkillMaxTargets(skillID string) int32 {
	switch skillID {
	case "maul":
		return 2
	default:
		return 1
	}
}

func fixtureCreatureSkillHitboxes(skillID string) []*dbv1.SkillHitboxProfile {
	damageGroupID := fixtureCreatureSkillHitboxDamageGroupID(skillID)
	motionProfile := fixtureCreatureSkillHitboxMotionProfile(skillID)
	if damageGroupID == "" || motionProfile == nil {
		return nil
	}
	targetType := "enemy"
	maxTargets := fixtureCreatureSkillMaxTargets(skillID)
	profile := &dbv1.SkillHitboxProfile{
		Id:                  fixtureCreatureSkillHitboxID(skillID),
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
		profile.HitboxStartMs, profile.HitboxEndMs = 3600, 3980
		profile.OffsetX, profile.OffsetY, profile.OffsetZ = 130, 0, 105
		profile.SizeX, profile.SizeY, profile.SizeZ = 100, 0, 120
		profile.Radius, profile.Length = 50, 320
	case "maul":
		profile.HitboxStartMs, profile.HitboxEndMs = 220, 740
		profile.OffsetX, profile.OffsetY, profile.OffsetZ = 80, 0, 100
		profile.SizeZ = 130
		profile.Radius, profile.Length, profile.Angle = 62, 170, 140
	default:
		return nil
	}
	return []*dbv1.SkillHitboxProfile{profile}
}

func fixtureCreatureSkillHitboxID(skillID string) string {
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

func fixtureCreatureSkillHitboxDamageGroupID(skillID string) string {
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

func fixtureCreatureSkillHitboxMotionProfile(skillID string) *dbv1.SkillHitboxMotionProfile {
	id, sweepShape, samples := fixtureCreatureSkillHitboxMotionSamples(skillID)
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
		DamageGroupId: fixtureCreatureSkillHitboxDamageGroupID(skillID),
		Samples:       samples,
	}
}

func fixtureCreatureSkillHitboxMotionSamples(skillID string) (string, string, []*dbv1.SkillHitboxMotionSample) {
	switch skillID {
	case "bite":
		return "motion_wolf_bite_melee_v1", "capsule_strip", []*dbv1.SkillHitboxMotionSample{
			fixtureHitboxMotionSample(0, 0.00, 45, 0, 85, 90, 0, 115, 45, 70, 0, 0),
			fixtureHitboxMotionSample(1, 0.55, 80, 0, 90, 95, 0, 115, 48, 125, 0, 0),
			fixtureHitboxMotionSample(2, 1.00, 95, 0, 85, 90, 0, 115, 45, 145, 0, 0),
		}
	case "lunge":
		return "motion_wolf_lunge_cross_v1", "capsule_strip", []*dbv1.SkillHitboxMotionSample{
			fixtureHitboxMotionSample(0, 0.00, 60, 0, 90, 100, 0, 120, 50, 100, 0, 0),
			fixtureHitboxMotionSample(1, 0.55, 140, 0, 110, 100, 0, 120, 50, 230, 0, 0),
			fixtureHitboxMotionSample(2, 1.00, 210, 0, 90, 100, 0, 120, 50, 320, 0, 0),
		}
	case "maul":
		return "motion_wolf_maul_lateral_counter_v1", "arc_slice", []*dbv1.SkillHitboxMotionSample{
			fixtureHitboxMotionSample(0, 0.00, 65, 40, 95, 0, 0, 125, 58, 120, -70, -25),
			fixtureHitboxMotionSample(1, 0.45, 90, 0, 100, 0, 0, 130, 62, 170, -25, 25),
			fixtureHitboxMotionSample(2, 1.00, 65, -40, 95, 0, 0, 125, 58, 120, 25, 70),
		}
	default:
		return "", "", nil
	}
}

func fixtureCreatureSkillContract(skillID string, contractID string, actionType string, reconciliation string, contactPolicy string, distance float64, durationMS, activeMS, recoveryMS, windupMS, skillRecoveryMS, cooldownMS int32) SkillRuntimeContract {
	action := MovementActionRuntimeContract{
		ID:                       contractID,
		AbilityKey:               skillID,
		ActionType:               actionType,
		DurationMS:               durationMS,
		AirborneDurationMS:       activeMS,
		ActiveMS:                 activeMS,
		RecoveryMS:               recoveryMS,
		DistanceCM:               distance,
		SpeedCurveSamples:        fixtureMovementCurve(skillID),
		VerticalCurveSamples:     fixtureVerticalCurve(skillID),
		VerticalMotionModel:      "curve",
		GravityScale:             1,
		ReconciliationContractID: reconciliation,
		ReconciliationCategory:   reconciliation,
		PhaseWindowPolicy:        actionType,
		PredictionErrorPolicy:    "bounded_smooth_correction",
		RootMotionOwner:          "movement",
		ContactPolicy:            contactPolicy,
	}
	if skillID == "lunge" {
		action.JumpZVelocity = 180
		action.ExpectedApexMS = 160
		action.LandingDetectionPolicy = "server_grounded_handoff"
		action.GroundZPolicy = "server_position_is_actor_root"
	}
	damage, posture := fixtureCreatureSkillDamage(skillID)
	impact := fixtureSkillImpactProfile(skillID, posture)
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
		StaminaCost:              fixtureCreatureSkillStaminaCost(skillID),
		TargetPolicy:             "target_direction",
		ContactPolicy:            contactPolicy,
		MaxTargets:               fixtureCreatureSkillMaxTargets(skillID),
		Blockable:                true,
		Impact:                   impact,
		ControlEffects:           enabledSkillControlEffects(impact.GetControlEffects()),
		Hitboxes:                 fixtureCreatureSkillHitboxes(skillID),
		Enabled:                  true,
	}
}

func fixtureCombatModeSlots() []*gamev1.CombatModeSlot {
	return []*gamev1.CombatModeSlot{
		{CombatModeId: swordShieldVanguardModeID, SlotIndex: 0, Enabled: false},
		{CombatModeId: swordShieldVanguardModeID, SlotIndex: 1, Enabled: false},
		{CombatModeId: swordShieldVanguardModeID, SlotIndex: 2, Enabled: false},
		{CombatModeId: swordShieldVanguardModeID, SlotIndex: 3, Enabled: false},
		{CombatModeId: swordShieldVanguardModeID, SlotIndex: 4, Enabled: false},
		{CombatModeId: swordShieldBulwarkModeID, SlotIndex: 0, SkillId: "player_basic_attack_1", Enabled: true},
		{CombatModeId: swordShieldBulwarkModeID, SlotIndex: 1, Enabled: false},
		{CombatModeId: swordShieldBulwarkModeID, SlotIndex: 2, SkillId: "player_shield_bash", Enabled: true},
		{CombatModeId: swordShieldBulwarkModeID, SlotIndex: 3, SkillId: "player_shield_rush", Enabled: true},
		{CombatModeId: swordShieldBulwarkModeID, SlotIndex: 4, Enabled: false},
	}
}

func combatModeSlotsFromDB(slots []*dbv1.WeaponCombatModeSlot) []*gamev1.CombatModeSlot {
	out := make([]*gamev1.CombatModeSlot, 0, len(slots))
	for _, slot := range slots {
		if slot == nil {
			continue
		}
		slotIndex, ok := combatInputSlotIndex(slot.GetInputSlot())
		if !ok {
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

func combatInputSlotIndex(input string) (uint32, bool) {
	switch strings.ToUpper(strings.TrimSpace(input)) {
	case "M1", "BASIC", "BASIC_ATTACK", "LEFT_MOUSE":
		return 0, true
	case "Q":
		return 1, true
	case "R":
		return 2, true
	case "F":
		return 3, true
	case "G":
		return 4, true
	default:
		return 0, false
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

func (c RuntimeContracts) combatCoreProfileForEntity(entity *entityState) *dbv1.CombatCoreProfile {
	profileID := c.CombatCore.PlayerProfileID
	if entity != nil && strings.EqualFold(entity.entityType, "creature") {
		profileID = c.CombatCore.CreatureProfileID
	}
	if profileID == "" {
		return nil
	}
	return c.CombatCore.Profiles[profileID]
}

func (c RuntimeContracts) defenseContractForEntity(entity *entityState) *dbv1.CombatDefenseContract {
	contractID := c.Defense.PlayerGuardContractID
	if entity != nil && strings.EqualFold(entity.entityType, "creature") {
		contractID = c.Defense.CreatureGuardContractID
	}
	if contractID == "" {
		return nil
	}
	return c.Defense.Contracts[contractID]
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
		VerticalMotionModel:      meta.VerticalMotionModel,
		JumpZVelocity:            meta.JumpZVelocity,
		GravityScale:             meta.GravityScale,
		GravityZCMSS:             meta.GravityZCMSS,
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

func fixtureMovementProfile() *gamev1.MovementReconciliationProfile {
	return &gamev1.MovementReconciliationProfile{
		ProfileId:                         "fixture_default_movement_profile",
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
		BackpedalSpeedMultiplier:          0.50,
		StrafeSprintSpeedMultiplier:       0.75,
		BackpedalSprintSpeedMultiplier:    0.50,
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
