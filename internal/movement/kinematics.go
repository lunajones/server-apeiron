package movement

import (
	"math"

	domainmath "server-apeiron/internal/domain/math"
)

type SpeedProfile struct {
	MaxSpeed                       float64
	SprintSpeedMultiplier          float64
	StrafeSpeedMultiplier          float64
	BackpedalSpeedMultiplier       float64
	StrafeSprintSpeedMultiplier    float64
	BackpedalSprintSpeedMultiplier float64
}

type MotionResult struct {
	Start            domainmath.Position
	Projected        domainmath.Position
	Direction        domainmath.Vec3
	Velocity         domainmath.Vec3
	SpeedCMPerSecond float64
	DistanceCM       float64
	Stopped          bool
}

type GroundedMoveInput struct {
	Position        domainmath.Position
	Direction       domainmath.Vec3
	FacingYawDeg    float64
	AnalogMagnitude float64
	Sprint          bool
	TickRate        float64
	Profile         SpeedProfile
}

type ActionMotionInput struct {
	Position           domainmath.Position
	Direction          domainmath.Vec3
	Contract           RuntimeActionContract
	FallbackDistanceCM float64
}

type ConstantStepInput struct {
	Position         domainmath.Position
	Direction        domainmath.Vec3
	SpeedCMPerSecond float64
	TickRate         float64
}

func ResolveGroundedMove(in GroundedMoveInput) MotionResult {
	dir := Normalize(in.Direction)
	if dir.IsZero() {
		return MotionResult{Start: in.Position, Projected: in.Position, Stopped: true}
	}
	speed := GroundedMoveSpeed(in.Profile, in.Sprint, in.AnalogMagnitude, dir, in.FacingYawDeg)
	tickRate := positiveFloat(in.TickRate, 30)
	distance := speed / tickRate
	projected := in.Position.Add(dir.Scale(distance))
	return MotionResult{
		Start:            in.Position,
		Projected:        projected,
		Direction:        dir,
		Velocity:         dir.Scale(speed),
		SpeedCMPerSecond: speed,
		DistanceCM:       distance,
	}
}

func ResolveActionMotion(in ActionMotionInput) MotionResult {
	dir := Normalize(in.Direction)
	if dir.IsZero() {
		return MotionResult{Start: in.Position, Projected: in.Position, Stopped: true}
	}
	distance := ActionDistance(in.Contract, in.FallbackDistanceCM)
	if distance <= 0 {
		return MotionResult{Start: in.Position, Projected: in.Position, Direction: dir, Stopped: true}
	}
	speed := ActionSpeed(in.Contract, distance)
	projected := in.Position.Add(dir.Scale(distance))
	return MotionResult{
		Start:            in.Position,
		Projected:        projected,
		Direction:        dir,
		Velocity:         dir.Scale(speed),
		SpeedCMPerSecond: speed,
		DistanceCM:       distance,
	}
}

func ResolveConstantStep(in ConstantStepInput) MotionResult {
	dir := Normalize(in.Direction)
	if dir.IsZero() || in.SpeedCMPerSecond <= 0 {
		return MotionResult{Start: in.Position, Projected: in.Position, Stopped: true}
	}
	tickRate := positiveFloat(in.TickRate, 30)
	distance := in.SpeedCMPerSecond / tickRate
	projected := in.Position.Add(dir.Scale(distance))
	return MotionResult{
		Start:            in.Position,
		Projected:        projected,
		Direction:        dir,
		Velocity:         dir.Scale(in.SpeedCMPerSecond),
		SpeedCMPerSecond: in.SpeedCMPerSecond,
		DistanceCM:       distance,
	}
}

func GroundedMoveSpeed(profile SpeedProfile, sprint bool, analogMagnitude float64, dir domainmath.Vec3, facingYawDeg float64) float64 {
	walkSpeed := profile.MaxSpeed
	if walkSpeed <= 0 {
		return 0
	}
	sprintMultiplier := positiveFloat(profile.SprintSpeedMultiplier, 1)
	analog := math.Max(0, math.Min(1, analogMagnitude))
	if analog <= 0 {
		analog = 1
	}

	modeSpeed := walkSpeed
	if sprint {
		modeSpeed = walkSpeed * sprintMultiplier
	}
	requestedSpeed := modeSpeed * analog
	if requestedSpeed <= 0 {
		return 0
	}

	moveDir := Normalize(domainmath.V3(dir.X, dir.Y, 0))
	facingDir := Normalize(YawVector(facingYawDeg))
	if moveDir.IsZero() || facingDir.IsZero() {
		return requestedSpeed
	}

	dot := math.Max(-1, math.Min(1, moveDir.Dot(facingDir)))
	capMultiplier := 0.0
	if dot <= -0.35 {
		capMultiplier = profile.BackpedalSpeedMultiplier
		if sprint {
			capMultiplier = profile.BackpedalSprintSpeedMultiplier
		}
	} else if dot < 0.50 {
		capMultiplier = profile.StrafeSpeedMultiplier
		if sprint {
			capMultiplier = profile.StrafeSprintSpeedMultiplier
		}
	}
	if capMultiplier <= 0 {
		return requestedSpeed
	}
	return math.Min(requestedSpeed, modeSpeed*capMultiplier*analog)
}

func ActionDistance(contract RuntimeActionContract, fallbackCM float64) float64 {
	if math.Abs(contract.DistanceCM) > domainmath.Epsilon {
		return contract.DistanceCM
	}
	return fallbackCM
}

func ActionSpeed(contract RuntimeActionContract, distanceCM float64) float64 {
	if contract.BaseSpeedCMS > 0 {
		return contract.BaseSpeedCMS
	}
	durationMS := contract.DurationMS
	if durationMS <= 0 {
		durationMS = contract.ActiveMS + contract.RecoveryMS
	}
	if durationMS <= 0 || distanceCM <= 0 {
		return 0
	}
	return distanceCM / (float64(durationMS) / 1000)
}

func Normalize(v domainmath.Vec3) domainmath.Vec3 {
	return v.Normalize()
}

func YawVector(yawDeg float64) domainmath.Vec3 {
	radians := yawDeg * math.Pi / 180
	return domainmath.V3(math.Cos(radians), math.Sin(radians), 0)
}

func YawFromVector(v domainmath.Vec3) float64 {
	return math.Atan2(v.Y, v.X) * 180 / math.Pi
}

func positiveFloat(value, fallback float64) float64 {
	if value > 0 {
		return value
	}
	return fallback
}
