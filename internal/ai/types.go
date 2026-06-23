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
	OrbitLocomotionMode            string
	OrbitSpeedScale                float64
	MinOrbitDurationTicks          uint64
	SideSwitchCooldownTicks        uint64
	AllowSideSwitchWhenTargetFaces bool
	PreferLongSideCommit           bool
	SideFlipChanceMultiplier       float64
	LockSideDuringSetup            bool
	Bindings                       []SkillBinding
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

type Input struct {
	Tick             uint64
	CreaturePosition domainmath.Position
	TargetPosition   domainmath.Position
	TargetFacingYaw  float64
	ActiveSkillID    string
	LineOfSight      bool
	Pressure         float64
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
	SpeedCMPerSec  float64
	Direction      domainmath.Vec3
	Destination    domainmath.Position
	RangeCM        float64
}
