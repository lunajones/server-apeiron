package movement

import (
	"time"

	"server-apeiron/internal/domain/ids"
	domainmath "server-apeiron/internal/domain/math"
)

type Intent struct {
	EntityID                  ids.RuntimeEntityID
	CommandID                 string
	Sequence                  uint64
	ClientActionSequence      uint64
	ClientTick                uint64
	ServerReceivedTick        uint64
	ServerActionStartedTick   uint64
	ClientPredictedActionTick uint64
	SkillID                   string
	AbilityKey                string
	ActionFamily              string
	Direction                 domainmath.Vec3
	Force                     float64
	SpeedScale                float64
	Action                    string
	MovementMode              string
	Phase                     string
	StartedAt                 time.Time
	Duration                  time.Duration
	ActionStartPosition       domainmath.Position
	HasActionStartPosition    bool
	ActionDuration            time.Duration
	StartupDuration           time.Duration
	ActiveDuration            time.Duration
	RecoveryDuration          time.Duration
	Distance                  float64
	SpeedCurveID              string
	SpeedCurveSamples         []MovementActionCurvePoint
	PredictionErrorPolicy     string
	TimelineClassification    string
	Effect                    SkillMovementEffect
	Contract                  MovementActionContract
	HasContract               bool
	ReceivedAt                time.Time
	StoredAt                  time.Time
}

type Result struct {
	Position         domainmath.Position
	Velocity         domainmath.Vec3
	DistanceTraveled float64
	Blocked          bool
}

type SkillMovementEffect struct {
	ID                     string
	SkillID                string
	Type                   string
	MovementType           string
	Distance               float64
	Speed                  float64
	Duration               time.Duration
	DurationMS             int32
	DesiredLandingDistance float64
	MinLandingDistance     float64
	StopAtContactRatio     float64
	ArcHeight              float64
	ArcCurve               string
	TakeoffMS              int32
	LandingLockMS          int32
	MovementStartPhase     string
	MovementStartOffsetMS  int32
	SteeringPolicy         string
	MaxTurnDegPerSec       float64
	MaxTotalRedirectAngle  float64
	RedirectLockoutMS      int32
	CanPhaseThroughTargets bool
	AppliesKnockback       bool
	KnockbackDistance      float64
	KnockbackSpeed         float64
	RespectsNavMesh        bool
}

type GroundedSkillMovementIntentSpec struct {
	EntityID                  ids.RuntimeEntityID
	CommandID                 string
	Sequence                  uint64
	ClientActionSequence      uint64
	ClientTick                uint64
	ServerReceivedTick        uint64
	ServerActionStartedTick   uint64
	ClientPredictedActionTick uint64
	SkillID                   string
	AbilityKey                string
	ActionFamily              string
	MovementType              string
	ActionPriority            int
	Direction                 domainmath.Vec3
	Force                     float64
	SpeedScale                float64
	ActionStartPosition       domainmath.Position
	HasActionStartPosition    bool
	ActionDuration            time.Duration
	StartupDuration           time.Duration
	ActiveDuration            time.Duration
	RecoveryDuration          time.Duration
	Distance                  float64
	SpeedCurveID              string
	SpeedCurveSamples         []MovementActionCurvePoint
	PredictionErrorPolicy     string
	TimelineClassification    string
	StartedAt                 time.Time
	Duration                  time.Duration
	Effect                    SkillMovementEffect
	Contract                  MovementActionContract
	HasContract               bool
	ReceivedAt                time.Time
	StoredAt                  time.Time
}

func NewGroundedSkillMovementIntent(spec GroundedSkillMovementIntentSpec) Intent {
	return Intent{
		EntityID:                  spec.EntityID,
		CommandID:                 spec.CommandID,
		Sequence:                  spec.Sequence,
		ClientActionSequence:      spec.ClientActionSequence,
		ClientTick:                spec.ClientTick,
		ServerReceivedTick:        spec.ServerReceivedTick,
		ServerActionStartedTick:   spec.ServerActionStartedTick,
		ClientPredictedActionTick: spec.ClientPredictedActionTick,
		SkillID:                   spec.SkillID,
		AbilityKey:                spec.AbilityKey,
		ActionFamily:              spec.ActionFamily,
		Direction:                 spec.Direction,
		Force:                     spec.Force,
		SpeedScale:                spec.SpeedScale,
		Action:                    "skill_grounded_action",
		MovementMode:              "skill",
		Phase:                     "active",
		StartedAt:                 firstTime(spec.StartedAt, spec.StoredAt),
		Duration:                  firstDuration(spec.Duration, spec.ActionDuration),
		ActionStartPosition:       spec.ActionStartPosition,
		HasActionStartPosition:    spec.HasActionStartPosition,
		ActionDuration:            spec.ActionDuration,
		StartupDuration:           spec.StartupDuration,
		ActiveDuration:            spec.ActiveDuration,
		RecoveryDuration:          spec.RecoveryDuration,
		Distance:                  spec.Distance,
		SpeedCurveID:              spec.SpeedCurveID,
		SpeedCurveSamples:         spec.SpeedCurveSamples,
		PredictionErrorPolicy:     spec.PredictionErrorPolicy,
		TimelineClassification:    spec.TimelineClassification,
		Effect:                    spec.Effect,
		Contract:                  spec.Contract,
		HasContract:               spec.HasContract,
		ReceivedAt:                spec.ReceivedAt,
		StoredAt:                  spec.StoredAt,
	}
}

func firstTime(values ...time.Time) time.Time {
	for _, value := range values {
		if !value.IsZero() {
			return value
		}
	}
	return time.Time{}
}

func firstDuration(values ...time.Duration) time.Duration {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

type ServerPlacementOptions struct {
	Velocity    domainmath.Vec3
	HasVelocity bool
	Reason      string
	Tick        uint64
}

type placeable interface {
	SetPosition(domainmath.Position)
	SetVelocity(domainmath.Vec3)
}

func ApplyServerPlacement(target placeable, position domainmath.Position, options ServerPlacementOptions, extraTick ...uint64) {
	if target == nil {
		return
	}
	target.SetPosition(position)
	if options.HasVelocity {
		target.SetVelocity(options.Velocity)
	}
}

func DeltaSeconds(delta time.Duration) float64 {
	return delta.Seconds()
}

type MovementActionCurvePoint struct {
	T     float64
	Value float64
}

type MovementActionCurve struct {
	ID      string
	Points  []MovementActionCurvePoint
	Samples []MovementActionCurvePoint
}

func (c MovementActionCurve) Sample(t float64) float64 {
	points := c.Points
	if len(points) == 0 {
		points = c.Samples
	}
	if len(points) == 0 {
		return 1
	}
	if t <= points[0].T {
		return points[0].Value
	}
	for i := 1; i < len(points); i++ {
		prev := points[i-1]
		next := points[i]
		if t <= next.T {
			span := next.T - prev.T
			if span <= domainmath.Epsilon {
				return next.Value
			}
			alpha := (t - prev.T) / span
			return prev.Value + (next.Value-prev.Value)*alpha
		}
	}
	return points[len(points)-1].Value
}

const MovementActionCurveHorizontalSpeedScale = "horizontal_speed_scale"

type MovementActionContract struct {
	ID                    string
	MovementAction        string
	MovementType          string
	ReconciliationMode    string
	PredictionErrorPolicy string
	Enabled               bool
	DurationMS            int32
	StartupMS             int32
	ActiveMS              int32
	RecoveryMS            int32
	HorizontalDistanceCM  float64
	BaseSpeedCMPerSec     float64
	MaxTurnDegPerSec      float64
	MaxRedirectDeg        float64
	MovementMode          string
	Phase                 string
	AxisPolicies          []MovementActionAxisPolicy
	Curves                map[string]MovementActionCurve
}

type MovementActionAxisPolicy struct {
	Enabled              bool
	Axis                 string
	ReconciliationPolicy string
}

func (c MovementActionContract) Curve(id string) (MovementActionCurve, bool) {
	if c.Curves == nil {
		return MovementActionCurve{}, false
	}
	curve, ok := c.Curves[id]
	return curve, ok
}
