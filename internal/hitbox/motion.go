package hitbox

import (
	gomath "math"
	"time"

	apeironv1 "db-apeiron/gen/apeiron/v1"
	domainmath "server-apeiron/internal/domain/math"
	"server-apeiron/internal/spatial"
)

type MotionHitboxShape struct {
	Shape                  RuntimeShape
	MotionProfileID        string
	DamageGroupID          string
	TStart                 float64
	TEnd                   float64
	MotionSampleStartIndex int32
	MotionSampleEndIndex   int32
}

func ShapeFromMotionProfile(profile *apeironv1.SkillHitboxProfile, basis Basis, previousBasis Basis, elapsed time.Duration, previousElapsed time.Duration) (MotionHitboxShape, bool) {
	motion := profile.GetMotionProfile()
	if motion == nil || !motion.GetEnabled() {
		return MotionHitboxShape{}, false
	}
	if motion.GetMotionType() != "timeline_sweep" || motion.GetTimeBasis() != "hitbox_window_normalized" {
		return MotionHitboxShape{}, false
	}
	samples := motion.GetSamples()
	if len(samples) == 0 {
		return MotionHitboxShape{}, false
	}

	tStart := hitboxMotionT(profile, previousElapsed)
	tEnd := hitboxMotionT(profile, elapsed)
	if tStart > tEnd {
		tStart, tEnd = tEnd, tStart
	}
	sampleStartIndex, sampleEndIndex := hitboxMotionSampleIndexRange(samples, tStart, tEnd)

	startSample := interpolateHitboxMotionSample(profile, samples, tStart, motion.GetInterpolation())
	endSample := interpolateHitboxMotionSample(profile, samples, tEnd, motion.GetInterpolation())
	var shape RuntimeShape
	switch motion.GetSweepShape() {
	case "arc_slice":
		shape = arcSliceShapeFromMotionSamples(profile, basis, previousBasis, startSample, endSample)
	case "box_strip":
		shape = boxStripShapeFromMotionSamples(profile, basis, previousBasis, startSample, endSample)
	case "capsule_strip":
		shape = capsuleStripShapeFromMotionSamples(profile, basis, previousBasis, startSample, endSample)
	default:
		return MotionHitboxShape{}, false
	}
	return MotionHitboxShape{
		Shape:                  shape,
		MotionProfileID:        motion.GetId(),
		DamageGroupID:          motion.GetDamageGroupId(),
		TStart:                 tStart,
		TEnd:                   tEnd,
		MotionSampleStartIndex: sampleStartIndex,
		MotionSampleEndIndex:   sampleEndIndex,
	}, true
}

func hitboxMotionT(profile *apeironv1.SkillHitboxProfile, elapsed time.Duration) float64 {
	start := ms(profile.GetHitboxStartMs())
	end := ms(profile.GetHitboxEndMs())
	if elapsed <= start {
		return 0
	}
	if end <= start {
		return 1
	}
	return clamp01(float64(elapsed-start) / float64(end-start))
}

func interpolateHitboxMotionSample(profile *apeironv1.SkillHitboxProfile, samples []*apeironv1.SkillHitboxMotionSample, t float64, interpolation string) motionSample {
	if len(samples) == 0 {
		return motionSampleFromProfile(profile)
	}

	var first *apeironv1.SkillHitboxMotionSample
	var last *apeironv1.SkillHitboxMotionSample
	var lower *apeironv1.SkillHitboxMotionSample
	var upper *apeironv1.SkillHitboxMotionSample
	for _, sample := range samples {
		if sample == nil {
			continue
		}
		if first == nil || sample.GetT() < first.GetT() {
			first = sample
		}
		if last == nil || sample.GetT() > last.GetT() {
			last = sample
		}
		if sample.GetT() <= t && (lower == nil || sample.GetT() > lower.GetT()) {
			lower = sample
		}
		if sample.GetT() >= t && (upper == nil || sample.GetT() < upper.GetT()) {
			upper = sample
		}
	}
	if lower == nil {
		lower = first
	}
	if upper == nil {
		upper = last
	}
	if lower == nil || upper == nil {
		return motionSampleFromProfile(profile)
	}
	if lower == upper || interpolation == "step" {
		return motionSampleFromProto(profile, lower)
	}

	span := upper.GetT() - lower.GetT()
	alpha := 0.0
	if span > domainmath.Epsilon {
		alpha = clamp01((t - lower.GetT()) / span)
	}
	a := motionSampleFromProto(profile, lower)
	b := motionSampleFromProto(profile, upper)
	return lerpMotionSample(a, b, alpha)
}

func hitboxMotionSampleIndexRange(samples []*apeironv1.SkillHitboxMotionSample, tStart float64, tEnd float64) (int32, int32) {
	if len(samples) == 0 {
		return 0, 0
	}
	startLower, startUpper := hitboxMotionSampleBounds(samples, tStart)
	endLower, endUpper := hitboxMotionSampleBounds(samples, tEnd)
	minIndex := minMotionSampleIndex(startLower, startUpper, endLower, endUpper)
	maxIndex := maxMotionSampleIndex(startLower, startUpper, endLower, endUpper)
	return minIndex, maxIndex
}

func hitboxMotionSampleBounds(samples []*apeironv1.SkillHitboxMotionSample, t float64) (int32, int32) {
	firstOrdinal := -1
	lastOrdinal := -1
	lowerOrdinal := -1
	upperOrdinal := -1
	var first *apeironv1.SkillHitboxMotionSample
	var last *apeironv1.SkillHitboxMotionSample
	var lower *apeironv1.SkillHitboxMotionSample
	var upper *apeironv1.SkillHitboxMotionSample
	for ordinal, sample := range samples {
		if sample == nil {
			continue
		}
		if first == nil || sample.GetT() < first.GetT() {
			first = sample
			firstOrdinal = ordinal
		}
		if last == nil || sample.GetT() > last.GetT() {
			last = sample
			lastOrdinal = ordinal
		}
		if sample.GetT() <= t && (lower == nil || sample.GetT() > lower.GetT()) {
			lower = sample
			lowerOrdinal = ordinal
		}
		if sample.GetT() >= t && (upper == nil || sample.GetT() < upper.GetT()) {
			upper = sample
			upperOrdinal = ordinal
		}
	}
	if lower == nil {
		lower = first
		lowerOrdinal = firstOrdinal
	}
	if upper == nil {
		upper = last
		upperOrdinal = lastOrdinal
	}
	return motionSampleIndex(lower, lowerOrdinal), motionSampleIndex(upper, upperOrdinal)
}

func motionSampleIndex(sample *apeironv1.SkillHitboxMotionSample, ordinal int) int32 {
	if sample == nil {
		return 0
	}
	if sample.GetSampleIndex() != 0 || ordinal <= 0 {
		return sample.GetSampleIndex()
	}
	return int32(ordinal)
}

func minMotionSampleIndex(values ...int32) int32 {
	if len(values) == 0 {
		return 0
	}
	min := values[0]
	for _, value := range values[1:] {
		if value < min {
			min = value
		}
	}
	return min
}

func maxMotionSampleIndex(values ...int32) int32 {
	if len(values) == 0 {
		return 0
	}
	max := values[0]
	for _, value := range values[1:] {
		if value > max {
			max = value
		}
	}
	return max
}

type motionSample struct {
	OffsetX       float64
	OffsetY       float64
	OffsetZ       float64
	Length        float64
	Radius        float64
	SizeX         float64
	SizeY         float64
	SizeZ         float64
	StartAngleDeg float64
	EndAngleDeg   float64
}

func motionSampleFromProfile(profile *apeironv1.SkillHitboxProfile) motionSample {
	return motionSample{
		OffsetX:       profile.GetOffsetX(),
		OffsetY:       profile.GetOffsetY(),
		OffsetZ:       profile.GetOffsetZ(),
		Length:        firstPositive(profile.GetLength(), profile.GetSizeX(), profile.GetRadius()),
		Radius:        firstPositive(profile.GetRadius(), 0),
		SizeX:         firstPositive(profile.GetSizeX(), profile.GetLength(), profile.GetRadius()*2, 1),
		SizeY:         firstPositive(profile.GetSizeY(), profile.GetRadius()*2, 1),
		SizeZ:         firstPositive(profile.GetSizeZ(), profile.GetRadius()*2, 180),
		StartAngleDeg: profile.GetSizeX(),
		EndAngleDeg:   -profile.GetSizeY(),
	}
}

func motionSampleFromProto(profile *apeironv1.SkillHitboxProfile, sample *apeironv1.SkillHitboxMotionSample) motionSample {
	if sample == nil {
		return motionSampleFromProfile(profile)
	}
	return motionSample{
		OffsetX:       sample.GetOffsetX(),
		OffsetY:       sample.GetOffsetY(),
		OffsetZ:       sample.GetOffsetZ(),
		Length:        firstPositive(sample.GetLength(), profile.GetLength(), profile.GetSizeX(), profile.GetRadius()),
		Radius:        firstPositive(sample.GetRadius(), profile.GetRadius(), 0),
		SizeX:         firstPositive(sample.GetSizeX(), profile.GetSizeX(), sample.GetLength(), profile.GetLength(), profile.GetRadius()*2, 1),
		SizeY:         firstPositive(sample.GetSizeY(), profile.GetSizeY(), sample.GetRadius()*2, profile.GetRadius()*2, 1),
		SizeZ:         firstPositive(sample.GetSizeZ(), profile.GetSizeZ(), profile.GetRadius()*2, 180),
		StartAngleDeg: sample.GetStartAngleDeg(),
		EndAngleDeg:   sample.GetEndAngleDeg(),
	}
}

func lerpMotionSample(a motionSample, b motionSample, alpha float64) motionSample {
	return motionSample{
		OffsetX:       lerp(a.OffsetX, b.OffsetX, alpha),
		OffsetY:       lerp(a.OffsetY, b.OffsetY, alpha),
		OffsetZ:       lerp(a.OffsetZ, b.OffsetZ, alpha),
		Length:        lerp(a.Length, b.Length, alpha),
		Radius:        lerp(a.Radius, b.Radius, alpha),
		SizeX:         lerp(a.SizeX, b.SizeX, alpha),
		SizeY:         lerp(a.SizeY, b.SizeY, alpha),
		SizeZ:         lerp(a.SizeZ, b.SizeZ, alpha),
		StartAngleDeg: lerp(a.StartAngleDeg, b.StartAngleDeg, alpha),
		EndAngleDeg:   lerp(a.EndAngleDeg, b.EndAngleDeg, alpha),
	}
}

func arcSliceShapeFromMotionSamples(profile *apeironv1.SkillHitboxProfile, basis Basis, previousBasis Basis, start motionSample, end motionSample) RuntimeShape {
	minDeg := gomath.Min(gomath.Min(start.StartAngleDeg, start.EndAngleDeg), gomath.Min(end.StartAngleDeg, end.EndAngleDeg))
	maxDeg := gomath.Max(gomath.Max(start.StartAngleDeg, start.EndAngleDeg), gomath.Max(end.StartAngleDeg, end.EndAngleDeg))
	length := gomath.Max(start.Length, end.Length)
	radius := gomath.Max(start.Radius, end.Radius)
	height := gomath.Max(start.SizeZ, end.SizeZ)
	startCenter := previousBasis.Offset(start.OffsetX, start.OffsetY, start.OffsetZ)
	endCenter := basis.Offset(end.OffsetX, end.OffsetY, end.OffsetZ)
	center := midpoint(startCenter, endCenter)
	if height <= 0 {
		height = firstPositive(profile.GetSizeZ(), profile.GetRadius()*2, 180)
	}
	return ArcSliceShape{
		Center:       center,
		Length:       firstPositive(length, profile.GetLength(), profile.GetSizeX(), profile.GetRadius()),
		MinDeg:       minDeg,
		MaxDeg:       maxDeg,
		Height:       height,
		QueryPadding: firstPositive(radius, profile.GetRadius(), 0),
		Basis:        basis,
	}
}

func boxStripShapeFromMotionSamples(profile *apeironv1.SkillHitboxProfile, basis Basis, previousBasis Basis, start motionSample, end motionSample) RuntimeShape {
	startCenter := previousBasis.Offset(start.OffsetX, start.OffsetY, start.OffsetZ)
	endCenter := basis.Offset(end.OffsetX, end.OffsetY, end.OffsetZ)
	center := midpoint(startCenter, endCenter)
	delta := endCenter.Sub(startCenter)
	size := domainmath.V3(
		firstPositive(gomath.Abs(delta.Dot(basis.Forward))+gomath.Max(start.SizeX, end.SizeX), profile.GetSizeX(), profile.GetLength(), 1),
		firstPositive(gomath.Abs(delta.Dot(basis.Right))+gomath.Max(start.SizeY, end.SizeY), profile.GetSizeY(), profile.GetRadius()*2, 1),
		firstPositive(gomath.Abs(delta.Dot(basis.Up))+gomath.Max(start.SizeZ, end.SizeZ), profile.GetSizeZ(), profile.GetRadius()*2, 1),
	)
	return BoxStripShape{Center: center, Size: size, Basis: basis}
}

func capsuleStripShapeFromMotionSamples(profile *apeironv1.SkillHitboxProfile, basis Basis, previousBasis Basis, start motionSample, end motionSample) RuntimeShape {
	startTail := previousBasis.Offset(start.OffsetX, start.OffsetY, start.OffsetZ)
	startTip := startTail.Add(previousBasis.Forward.Mul(firstPositive(start.Length, profile.GetLength(), profile.GetSizeX(), 1)))
	endTail := basis.Offset(end.OffsetX, end.OffsetY, end.OffsetZ)
	endTip := endTail.Add(basis.Forward.Mul(firstPositive(end.Length, profile.GetLength(), profile.GetSizeX(), 1)))

	a, b := furthestAlongBasis(basis, startTail, startTip, endTail, endTip)
	radius := firstPositive(gomath.Max(start.Radius, end.Radius), profile.GetRadius(), profile.GetSizeY()*0.5, 1)
	return CapsuleStripShape{
		Segment: domainmath.NewSegment(a, b),
		Radius:  radius,
	}
}

type ArcSliceShape struct {
	Center       domainmath.Position
	Length       float64
	MinDeg       float64
	MaxDeg       float64
	Height       float64
	QueryPadding float64
	Basis        Basis
}

func (s ArcSliceShape) Type() ShapeType { return ShapeArcSlice }

func (s ArcSliceShape) Bounds() domainmath.AABB {
	extent := firstPositive(s.Length+s.QueryPadding, s.Length, 1)
	height := firstPositive(s.Height, s.QueryPadding*2, 180)
	return domainmath.AABBFromCenterSize(s.Center, domainmath.V3(extent*2, extent*2, height))
}

func (s ArcSliceShape) Contains(object spatial.SpatialObject) bool {
	if !s.Bounds().Intersects(object.Bounds) {
		return false
	}
	for _, point := range aabbSamplePoints(object.Bounds, object.Position) {
		if s.containsPoint(point) {
			return true
		}
	}
	return s.containsPoint(object.Bounds.ClosestPoint(s.Center))
}

func (s ArcSliceShape) ImpactPoint(object spatial.SpatialObject) domainmath.Position {
	return object.Bounds.ClosestPoint(s.Center)
}

func (s ArcSliceShape) containsPoint(point domainmath.Position) bool {
	local := s.Basis.Local(point, s.Center)
	if firstPositive(s.Height, 180) > 0 && gomath.Abs(local.Z) > firstPositive(s.Height, 180)*0.5 {
		return false
	}
	if local.X < -domainmath.Epsilon || local.X > s.Length+s.QueryPadding {
		return false
	}
	horizontalDistance := gomath.Sqrt(local.X*local.X + local.Y*local.Y)
	if horizontalDistance <= domainmath.Epsilon {
		return s.MinDeg <= 0 && s.MaxDeg >= 0
	}
	signedDeg := domainmath.RadToDeg(gomath.Atan2(local.Y, local.X))
	return signedDeg >= s.MinDeg && signedDeg <= s.MaxDeg
}

type BoxStripShape struct {
	Center domainmath.Position
	Size   domainmath.Vec3
	Basis  Basis
}

func (s BoxStripShape) Type() ShapeType { return ShapeBoxStrip }

func (s BoxStripShape) Bounds() domainmath.AABB {
	extent := gomath.Max(s.Size.X, gomath.Max(s.Size.Y, s.Size.Z))
	return domainmath.AABBFromCenterSize(s.Center, domainmath.V3(extent, extent, extent))
}

func (s BoxStripShape) Contains(object spatial.SpatialObject) bool {
	local := s.Basis.Local(object.Bounds.Center(), s.Center)
	objectSize := object.Bounds.Size()
	horizontalRadius := gomath.Max(objectSize.X, objectSize.Y) * 0.5
	verticalHalf := objectSize.Z * 0.5
	return gomath.Abs(local.X) <= s.Size.X*0.5+horizontalRadius &&
		gomath.Abs(local.Y) <= s.Size.Y*0.5+horizontalRadius &&
		gomath.Abs(local.Z) <= s.Size.Z*0.5+verticalHalf
}

func (s BoxStripShape) ImpactPoint(object spatial.SpatialObject) domainmath.Position {
	return object.Bounds.ClosestPoint(s.Center)
}

type CapsuleStripShape struct {
	Segment domainmath.Segment
	Radius  float64
}

func (s CapsuleStripShape) Type() ShapeType { return ShapeCapsuleStrip }

func (s CapsuleStripShape) Bounds() domainmath.AABB {
	return s.Segment.Bounds().Expand(s.Radius)
}

func (s CapsuleStripShape) Contains(object spatial.SpatialObject) bool {
	closest := domainmath.ClosestPointOnSegment(object.Position, s.Segment.A, s.Segment.B)
	return domainmath.NewSphere(closest, s.Radius).IntersectsAABB(object.Bounds)
}

func (s CapsuleStripShape) ImpactPoint(object spatial.SpatialObject) domainmath.Position {
	closest := domainmath.ClosestPointOnSegment(object.Position, s.Segment.A, s.Segment.B)
	return object.Bounds.ClosestPoint(closest)
}

func midpoint(a domainmath.Position, b domainmath.Position) domainmath.Position {
	return domainmath.V3((a.X+b.X)*0.5, (a.Y+b.Y)*0.5, (a.Z+b.Z)*0.5)
}

func furthestAlongBasis(basis Basis, points ...domainmath.Position) (domainmath.Position, domainmath.Position) {
	if len(points) == 0 {
		return basis.Origin, basis.Origin
	}
	minPoint := points[0]
	maxPoint := points[0]
	minProjection := points[0].Sub(basis.Origin).Dot(basis.Forward)
	maxProjection := minProjection
	for _, point := range points[1:] {
		projection := point.Sub(basis.Origin).Dot(basis.Forward)
		if projection < minProjection {
			minProjection = projection
			minPoint = point
		}
		if projection > maxProjection {
			maxProjection = projection
			maxPoint = point
		}
	}
	return minPoint, maxPoint
}

func lerp(a float64, b float64, alpha float64) float64 {
	return a + (b-a)*alpha
}

func clamp01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
