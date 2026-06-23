package ai

import domainmath "server-apeiron/internal/domain/math"

type Policy struct {
	ContractID                     string
	ContractHash                   string
	CapabilityID                   string
	DesiredRangeCM                 float64
	ChaseRangeCM                   float64
	RetreatRangeCM                 float64
	OrbitSpeedCMS                  float64
	ChaseSpeedCMS                  float64
	LungeSpeedCMS                  float64
	MaulSpeedCMS                   float64
	RetreatSpeedCMS                float64
	DodgeSkillID                   string
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
	OrbitLocomotionMode            string
	OrbitSpeedScale                float64
	MinOrbitDurationTicks          uint64
	SideSwitchCooldownTicks        uint64
	AllowSideSwitchWhenTargetFaces bool
	PreferLongSideCommit           bool
	SideFlipChanceMultiplier       float64
	LockSideDuringSetup            bool
	RepeatSkillPenaltyWindowTicks  uint64
	RepeatSkillPenaltyMultiplier   float64
	Bindings                       []SkillBinding
	SetupPolicies                  map[string]SkillSetupPolicy
}

type SkillBinding struct {
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

type SkillSetupPolicy struct {
	ID                  string
	SkillID             string
	SetupType           string
	MinSetupTicks       uint64
	MaxSetupTicks       uint64
	CommitDistanceCM    float64
	PreferredMinRangeCM float64
	PreferredMaxRangeCM float64
	MovementTactic      string
	LockSideDuringSetup bool
	Enabled             bool
}

type Input struct {
	Tick                    uint64
	CreaturePosition        domainmath.Position
	TargetPosition          domainmath.Position
	TargetFacingYaw         float64
	ActiveSkillID           string
	ActiveSkillElapsedTicks uint64
	LineOfSight             bool
	Pressure                float64
	Perception              Perception
	ResourceCurrent         float64
	ResourceMax             float64
	SkillCosts              map[string]float64
	UnavailableSkill        map[string]string
}

type Decision struct {
	Action         string
	SelectedSkill  string
	TacticalState  string
	DecisionPhase  string
	MovementTactic string
	CombatTactic   string
	Commitment     string
	OrbitSide      string
	Reason         string
	Score          float64
	ResourceCost   float64
	ResourceState  string
	SpeedCMPerSec  float64
	Direction      domainmath.Vec3
	Destination    domainmath.Position
	RangeCM        float64
	SetupPolicyID  string
	Threat         ThreatAssessment
}
