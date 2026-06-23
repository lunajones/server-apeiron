package gameapi

import (
	"math"
	"strings"

	dbv1 "db-apeiron/gen/apeiron/v1"
)

const impactTemporalEpsilon = 0.0001

type temporalImpactSample struct {
	OffsetX       float64
	OffsetY       float64
	Length        float64
	Radius        float64
	SizeX         float64
	SizeY         float64
	StartAngleDeg float64
	EndAngleDeg   float64
}

func skillHitboxProfileContainsAt(skill SkillRuntimeContract, profile *dbv1.SkillHitboxProfile, start, end, dir vector, target vector, elapsedMS float64) bool {
	motion := profile.GetMotionProfile()
	if motion == nil || !motion.GetEnabled() || len(motion.GetSamples()) == 0 {
		return skillStaticHitboxProfileContains(skill, profile, start, end, dir, target)
	}
	return skillTemporalMotionHitboxContains(skill, profile, motion, start, dir, target, elapsedMS)
}

func skillImpactEvaluationElapsedMS(skill SkillRuntimeContract) float64 {
	best := -1.0
	for _, profile := range skill.Hitboxes {
		if profile == nil {
			continue
		}
		startMS := float64(profile.GetHitboxStartMs())
		if best < 0 || startMS < best {
			best = startMS
		}
	}
	if best >= 0 {
		return best
	}
	if skill.WindupMS > 0 {
		return float64(skill.WindupMS)
	}
	return 0
}

func skillTemporalMotionHitboxContains(skill SkillRuntimeContract, profile *dbv1.SkillHitboxProfile, motion *dbv1.SkillHitboxMotionProfile, start vector, dir vector, target vector, elapsedMS float64) bool {
	dir = normalize(dir)
	if dir == (vector{}) {
		return false
	}
	t, ok := skillTemporalHitboxT(profile, elapsedMS)
	if !ok {
		return false
	}
	sample, ok := skillTemporalImpactSampleAt(profile, motion.GetSamples(), t, motion.GetInterpolation())
	if !ok {
		return false
	}

	right := vector{x: -dir.y, y: dir.x}
	origin := vector{
		x: start.x + dir.x*sample.OffsetX + right.x*sample.OffsetY,
		y: start.y + dir.y*sample.OffsetX + right.y*sample.OffsetY,
		z: start.z,
	}

	switch strings.ToLower(strings.TrimSpace(motion.GetSweepShape())) {
	case "arc_slice", "arc", "asymmetric_arc":
		return skillTemporalArcContains(skill, profile, sample, origin, dir, target)
	case "box_strip", "box", "rectangle", "rect":
		return skillTemporalBoxContains(skill, profile, sample, origin, dir, target)
	case "capsule_strip", "capsule", "lane", "":
		return skillTemporalCapsuleContains(skill, profile, sample, origin, dir, target)
	default:
		return skillTemporalCapsuleContains(skill, profile, sample, origin, dir, target)
	}
}

func skillTemporalHitboxT(profile *dbv1.SkillHitboxProfile, elapsedMS float64) (float64, bool) {
	startMS := float64(profile.GetHitboxStartMs())
	endMS := float64(profile.GetHitboxEndMs())
	if endMS <= startMS {
		return 0, true
	}
	if elapsedMS+impactTemporalEpsilon < startMS || elapsedMS-impactTemporalEpsilon > endMS {
		return 0, false
	}
	return clamp01Float64((elapsedMS - startMS) / (endMS - startMS)), true
}

func skillTemporalImpactSampleAt(profile *dbv1.SkillHitboxProfile, samples []*dbv1.SkillHitboxMotionSample, t float64, interpolation string) (temporalImpactSample, bool) {
	if len(samples) == 0 {
		return skillTemporalImpactSampleFromProfile(profile), true
	}

	var first *dbv1.SkillHitboxMotionSample
	var last *dbv1.SkillHitboxMotionSample
	var lower *dbv1.SkillHitboxMotionSample
	var upper *dbv1.SkillHitboxMotionSample
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
		return temporalImpactSample{}, false
	}
	if lower == upper || strings.EqualFold(interpolation, "step") {
		return skillTemporalImpactSampleFromProto(profile, lower), true
	}

	span := upper.GetT() - lower.GetT()
	alpha := 0.0
	if span > impactTemporalEpsilon {
		alpha = clamp01Float64((t - lower.GetT()) / span)
	}
	return lerpTemporalImpactSample(
		skillTemporalImpactSampleFromProto(profile, lower),
		skillTemporalImpactSampleFromProto(profile, upper),
		alpha,
	), true
}

func skillTemporalCapsuleContains(skill SkillRuntimeContract, profile *dbv1.SkillHitboxProfile, sample temporalImpactSample, origin vector, dir vector, target vector) bool {
	lengthCM := firstPositiveFloat64(sample.Length, profile.GetLength(), skillRangeToCM(skill.Range))
	radiusCM := firstPositiveFloat64(sample.Radius, sample.SizeY/2, profile.GetRadius(), profile.GetSizeY()/2)
	if lengthCM <= 0 || radiusCM <= 0 {
		return false
	}
	end := vector{x: origin.x + dir.x*lengthCM, y: origin.y + dir.y*lengthCM, z: origin.z}
	closest := closestPointOnSegment(origin, end, target)
	return distance2D(closest, target) <= radiusCM
}

func skillTemporalBoxContains(skill SkillRuntimeContract, profile *dbv1.SkillHitboxProfile, sample temporalImpactSample, origin vector, dir vector, target vector) bool {
	lengthCM := firstPositiveFloat64(sample.SizeX, sample.Length, profile.GetSizeX(), profile.GetLength(), skillRangeToCM(skill.Range))
	halfWidthCM := firstPositiveFloat64(sample.SizeY/2, sample.Radius, profile.GetSizeY()/2, profile.GetRadius())
	if lengthCM <= 0 || halfWidthCM <= 0 {
		return false
	}
	rel := vector{x: target.x - origin.x, y: target.y - origin.y}
	forward := rel.x*dir.x + rel.y*dir.y
	if forward < -impactTemporalEpsilon || forward > lengthCM+impactTemporalEpsilon {
		return false
	}
	lateral := math.Abs(rel.x*(-dir.y) + rel.y*dir.x)
	return lateral <= halfWidthCM+impactTemporalEpsilon
}

func skillTemporalArcContains(skill SkillRuntimeContract, profile *dbv1.SkillHitboxProfile, sample temporalImpactSample, origin vector, dir vector, target vector) bool {
	reachCM := firstPositiveFloat64(sample.Length, sample.Radius, profile.GetLength(), profile.GetRadius(), skillRangeToCM(skill.Range))
	if reachCM <= 0 {
		return false
	}
	rel := vector{x: target.x - origin.x, y: target.y - origin.y}
	if length(rel) > reachCM+impactTemporalEpsilon {
		return false
	}
	minAngle, maxAngle, ok := skillTemporalAngleWindow(profile, sample)
	if !ok {
		return true
	}
	angle := signedAngleDeg(dir, rel)
	return angle >= minAngle-impactTemporalEpsilon && angle <= maxAngle+impactTemporalEpsilon
}

func skillTemporalAngleWindow(profile *dbv1.SkillHitboxProfile, sample temporalImpactSample) (float64, float64, bool) {
	if sample.StartAngleDeg != sample.EndAngleDeg {
		return math.Min(sample.StartAngleDeg, sample.EndAngleDeg), math.Max(sample.StartAngleDeg, sample.EndAngleDeg), true
	}
	if angle := profile.GetAngle(); angle > 0 {
		half := angle / 2
		return -half, half, true
	}
	return 0, 0, false
}

func skillTemporalImpactSampleFromProfile(profile *dbv1.SkillHitboxProfile) temporalImpactSample {
	return temporalImpactSample{
		OffsetX:       profile.GetOffsetX(),
		OffsetY:       profile.GetOffsetY(),
		Length:        firstPositiveFloat64(profile.GetLength(), profile.GetRadius()),
		Radius:        firstPositiveFloat64(profile.GetRadius(), profile.GetSizeY()/2),
		SizeX:         firstPositiveFloat64(profile.GetSizeX(), profile.GetLength(), profile.GetRadius()),
		SizeY:         firstPositiveFloat64(profile.GetSizeY(), profile.GetRadius()*2),
		StartAngleDeg: -profile.GetAngle() / 2,
		EndAngleDeg:   profile.GetAngle() / 2,
	}
}

func skillTemporalImpactSampleFromProto(profile *dbv1.SkillHitboxProfile, sample *dbv1.SkillHitboxMotionSample) temporalImpactSample {
	if sample == nil {
		return skillTemporalImpactSampleFromProfile(profile)
	}
	return temporalImpactSample{
		OffsetX:       sample.GetOffsetX(),
		OffsetY:       sample.GetOffsetY(),
		Length:        firstPositiveFloat64(sample.GetLength(), profile.GetLength(), profile.GetRadius()),
		Radius:        firstPositiveFloat64(sample.GetRadius(), profile.GetRadius(), sample.GetSizeY()/2),
		SizeX:         firstPositiveFloat64(sample.GetSizeX(), sample.GetLength(), profile.GetSizeX(), profile.GetLength()),
		SizeY:         firstPositiveFloat64(sample.GetSizeY(), sample.GetRadius()*2, profile.GetSizeY(), profile.GetRadius()*2),
		StartAngleDeg: sample.GetStartAngleDeg(),
		EndAngleDeg:   sample.GetEndAngleDeg(),
	}
}

func lerpTemporalImpactSample(a temporalImpactSample, b temporalImpactSample, alpha float64) temporalImpactSample {
	return temporalImpactSample{
		OffsetX:       lerpFloat64(a.OffsetX, b.OffsetX, alpha),
		OffsetY:       lerpFloat64(a.OffsetY, b.OffsetY, alpha),
		Length:        lerpFloat64(a.Length, b.Length, alpha),
		Radius:        lerpFloat64(a.Radius, b.Radius, alpha),
		SizeX:         lerpFloat64(a.SizeX, b.SizeX, alpha),
		SizeY:         lerpFloat64(a.SizeY, b.SizeY, alpha),
		StartAngleDeg: lerpFloat64(a.StartAngleDeg, b.StartAngleDeg, alpha),
		EndAngleDeg:   lerpFloat64(a.EndAngleDeg, b.EndAngleDeg, alpha),
	}
}

func firstPositiveFloat64(values ...float64) float64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func lerpFloat64(a, b, alpha float64) float64 {
	return a + (b-a)*clamp01Float64(alpha)
}

func clamp01Float64(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func distance2D(a, b vector) float64 {
	dx := a.x - b.x
	dy := a.y - b.y
	return math.Sqrt(dx*dx + dy*dy)
}
