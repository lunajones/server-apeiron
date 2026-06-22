package movement

import (
	"math"
	"sort"
	"time"

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

type ActionMotionProgressInput struct {
	Position           domainmath.Position
	Direction          domainmath.Vec3
	Contract           RuntimeActionContract
	FallbackDistanceCM float64
	Elapsed            time.Duration
}

type ActionMotionProgressResult struct {
	MotionResult
	Progress        float64
	Elapsed         time.Duration
	Duration        time.Duration
	TotalDistanceCM float64
	Complete        bool
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

func ResolveActionMotionProgress(in ActionMotionProgressInput) ActionMotionProgressResult {
	dir := Normalize(in.Direction)
	duration := ActionDuration(in.Contract)
	totalDistance := ActionDistance(in.Contract, in.FallbackDistanceCM)
	base := MotionResult{Start: in.Position, Projected: in.Position, Direction: dir, Stopped: true}
	out := ActionMotionProgressResult{
		MotionResult:    base,
		Elapsed:         in.Elapsed,
		Duration:        duration,
		TotalDistanceCM: totalDistance,
	}
	if dir.IsZero() || totalDistance <= 0 || duration <= 0 {
		out.Complete = in.Elapsed >= duration && duration > 0
		return out
	}

	progress := ActionMotionProgress(in.Contract, in.Elapsed)
	distance := totalDistance * progress
	projected := in.Position.Add(dir.Scale(distance))
	elapsedRatio := math.Max(0, math.Min(1, in.Elapsed.Seconds()/duration.Seconds()))
	speed := ActionProgressSpeed(in.Contract, totalDistance, duration, elapsedRatio)
	out.MotionResult = MotionResult{
		Start:            in.Position,
		Projected:        projected,
		Direction:        dir,
		Velocity:         dir.Scale(speed),
		SpeedCMPerSecond: speed,
		DistanceCM:       distance,
		Stopped:          false,
	}
	out.Progress = progress
	out.Complete = progress >= 1
	return out
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
	duration := ActionDuration(contract)
	if duration <= 0 || distanceCM <= 0 {
		return 0
	}
	return distanceCM / duration.Seconds()
}

func ActionDuration(contract RuntimeActionContract) time.Duration {
	durationMS := contract.DurationMS
	if durationMS <= 0 {
		durationMS = contract.ActiveMS + contract.RecoveryMS
	}
	if durationMS <= 0 {
		return 0
	}
	return time.Duration(durationMS) * time.Millisecond
}

func ActionMotionProgress(contract RuntimeActionContract, elapsed time.Duration) float64 {
	duration := ActionDuration(contract)
	if elapsed <= 0 || duration <= 0 {
		return 0
	}
	if elapsed >= duration {
		return 1
	}
	t := elapsed.Seconds() / duration.Seconds()
	samples := normalizedCurveSamples(contract.SpeedCurveSamples)
	if len(samples) == 0 {
		return math.Max(0, math.Min(1, t))
	}
	totalArea := integrateCurveArea(samples, 0, 1)
	if totalArea <= domainmath.Epsilon {
		return math.Max(0, math.Min(1, t))
	}
	return math.Max(0, math.Min(1, integrateCurveArea(samples, 0, t)/totalArea))
}

func ActionProgressSpeed(contract RuntimeActionContract, distanceCM float64, duration time.Duration, elapsedRatio float64) float64 {
	if duration <= 0 || distanceCM <= 0 || elapsedRatio >= 1 {
		return 0
	}
	samples := normalizedCurveSamples(contract.SpeedCurveSamples)
	if len(samples) == 0 {
		return ActionSpeed(contract, distanceCM)
	}
	totalArea := integrateCurveArea(samples, 0, 1)
	if totalArea <= domainmath.Epsilon {
		return ActionSpeed(contract, distanceCM)
	}
	scale := MovementActionCurve{Points: samples}.Sample(elapsedRatio)
	if scale < 0 {
		scale = 0
	}
	return (distanceCM / duration.Seconds()) * (scale / totalArea)
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

func normalizedCurveSamples(samples []MovementActionCurvePoint) []MovementActionCurvePoint {
	if len(samples) == 0 {
		return nil
	}
	points := make([]MovementActionCurvePoint, 0, len(samples)+2)
	for _, sample := range samples {
		if math.IsNaN(sample.T) || math.IsNaN(sample.Value) {
			continue
		}
		t := math.Max(0, math.Min(1, sample.T))
		points = append(points, MovementActionCurvePoint{T: t, Value: sample.Value})
	}
	if len(points) == 0 {
		return nil
	}
	sort.SliceStable(points, func(i, j int) bool {
		return points[i].T < points[j].T
	})
	if points[0].T > 0 {
		points = append([]MovementActionCurvePoint{{T: 0, Value: points[0].Value}}, points...)
	}
	last := points[len(points)-1]
	if last.T < 1 {
		points = append(points, MovementActionCurvePoint{T: 1, Value: last.Value})
	}
	return points
}

func integrateCurveArea(points []MovementActionCurvePoint, start, end float64) float64 {
	start = math.Max(0, math.Min(1, start))
	end = math.Max(start, math.Min(1, end))
	if len(points) == 0 || end <= start {
		return 0
	}
	curve := MovementActionCurve{Points: points}
	area := 0.0
	for i := 1; i < len(points); i++ {
		a := points[i-1]
		b := points[i]
		if b.T <= start || a.T >= end {
			continue
		}
		segStart := math.Max(start, a.T)
		segEnd := math.Min(end, b.T)
		if segEnd <= segStart {
			continue
		}
		startValue := math.Max(0, curve.Sample(segStart))
		endValue := math.Max(0, curve.Sample(segEnd))
		area += ((startValue + endValue) * 0.5) * (segEnd - segStart)
	}
	return area
}
