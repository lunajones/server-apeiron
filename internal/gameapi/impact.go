package gameapi

import (
	"math"
	"sort"

	dbv1 "db-apeiron/gen/apeiron/v1"
)

const (
	defaultImpactTargetLimit       int32   = 1
	skillRangeMetersToCMThreshold  float64 = 25
	centimetersPerMeter            float64 = 100
	defaultRecoveredImpactHalfLane float64 = 45
)

type runtimeSkillImpact struct {
	SourceID       uint64
	TargetID       uint64
	SkillID        string
	DamageApplied  float64
	PostureApplied float64
	Blocked        bool
}

type runtimeSkillImpactCandidate struct {
	target   *entityState
	distCM   float64
	blocked  bool
	profile  *dbv1.SkillHitboxProfile
	priority int32
}

func (r *Runtime) applySkillImpact(source *entityState, skill SkillRuntimeContract, start, end, dir vector) []runtimeSkillImpact {
	if r == nil || source == nil || (skill.Damage <= 0 && skill.PostureDamage <= 0) {
		return nil
	}
	dir = normalize(dir)
	if dir == (vector{}) {
		dir = normalize(vector{x: end.x - start.x, y: end.y - start.y})
	}
	if dir == (vector{}) {
		dir = yawVector(source.yaw)
	}

	candidates := make([]runtimeSkillImpactCandidate, 0, len(r.entities))
	for _, target := range r.entities {
		if !runtimeSkillCanTarget(source, target) {
			continue
		}
		profile, ok := skillRuntimeHitboxContains(skill, start, end, dir, target.position)
		if !ok {
			continue
		}
		hitTravelDir := normalize(vector{x: target.position.x - start.x, y: target.position.y - start.y})
		if hitTravelDir == (vector{}) {
			hitTravelDir = dir
		}
		blocked := skill.Blockable && resolveDirectionalBlock(
			target.combatState == "blocking",
			toDomainVector(hitTravelDir),
			toDomainVector(yawVector(target.yaw)),
			0,
		)
		candidates = append(candidates, runtimeSkillImpactCandidate{
			target:   target,
			distCM:   distance(start, target.position),
			blocked:  blocked,
			profile:  profile,
			priority: profile.GetPriority(),
		})
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].priority != candidates[j].priority {
			return candidates[i].priority > candidates[j].priority
		}
		return candidates[i].distCM < candidates[j].distCM
	})

	limit := skillRuntimeMaxTargets(skill)
	impacts := make([]runtimeSkillImpact, 0, minInt(len(candidates), int(limit)))
	for _, candidate := range candidates {
		if int32(len(impacts)) >= limit {
			break
		}
		target := candidate.target
		impact := runtimeSkillImpact{
			SourceID:       source.id,
			TargetID:       target.id,
			SkillID:        skill.SkillID,
			PostureApplied: skill.PostureDamage,
			Blocked:        candidate.blocked,
		}
		if candidate.blocked {
			target.posture = clampMin(target.posture-skill.PostureDamage, 0)
		} else {
			target.health = clampMin(target.health-skill.Damage, 0)
			target.posture = clampMin(target.posture-skill.PostureDamage, 0)
			impact.DamageApplied = skill.Damage
		}
		impacts = append(impacts, impact)
	}
	return impacts
}

func runtimeSkillCanTarget(source *entityState, target *entityState) bool {
	if source == nil || target == nil || source.id == target.id || target.health <= 0 {
		return false
	}
	switch source.entityType {
	case "player":
		return target.entityType == "creature"
	case "creature":
		return target.entityType == "player"
	default:
		return false
	}
}

func skillRuntimeHitboxContains(skill SkillRuntimeContract, start, end, dir vector, target vector) (*dbv1.SkillHitboxProfile, bool) {
	for _, profile := range skill.Hitboxes {
		if profile == nil {
			continue
		}
		if skillHitboxProfileContains(skill, profile, start, end, dir, target) {
			return profile, true
		}
	}
	return nil, false
}

func skillHitboxProfileContains(skill SkillRuntimeContract, profile *dbv1.SkillHitboxProfile, start, end, dir vector, target vector) bool {
	dir = normalize(dir)
	if dir == (vector{}) {
		return false
	}
	reach := skillHitboxForwardReachCM(skill, profile)
	if reach <= 0 {
		return false
	}

	angleMin, angleMax, hasAngle := skillHitboxAngleWindow(profile)
	if hasAngle {
		rel := vector{x: target.x - start.x, y: target.y - start.y}
		if length(rel) > reach {
			return false
		}
		angle := signedAngleDeg(dir, rel)
		return angle >= angleMin && angle <= angleMax
	}

	origin := closestPointOnSegment(start, end, target)
	rel := vector{x: target.x - origin.x, y: target.y - origin.y}
	forward := rel.x*dir.x + rel.y*dir.y
	if forward < 0 || forward > reach {
		return false
	}
	lateral := math.Abs(rel.x*(-dir.y) + rel.y*dir.x)
	return lateral <= skillHitboxLaneHalfWidthCM(profile)
}

func skillHitboxForwardReachCM(skill SkillRuntimeContract, profile *dbv1.SkillHitboxProfile) float64 {
	reach := maxFloat64(profile.GetLength(), profile.GetRadius(), skillRangeToCM(skill.Range))
	if motion := profile.GetMotionProfile(); motion != nil {
		for _, sample := range motion.GetSamples() {
			if sample == nil {
				continue
			}
			reach = maxFloat64(reach, sample.GetOffsetX()+sample.GetLength(), sample.GetLength())
		}
	}
	return reach
}

func skillHitboxLaneHalfWidthCM(profile *dbv1.SkillHitboxProfile) float64 {
	width := maxFloat64(profile.GetRadius(), profile.GetSizeX()/2, profile.GetSizeY()/2)
	if motion := profile.GetMotionProfile(); motion != nil {
		for _, sample := range motion.GetSamples() {
			if sample == nil {
				continue
			}
			width = maxFloat64(width, sample.GetRadius(), sample.GetSizeX()/2, sample.GetSizeY()/2)
		}
	}
	if width <= 0 {
		return defaultRecoveredImpactHalfLane
	}
	return width
}

func skillHitboxAngleWindow(profile *dbv1.SkillHitboxProfile) (float64, float64, bool) {
	if motion := profile.GetMotionProfile(); motion != nil {
		found := false
		minAngle := 0.0
		maxAngle := 0.0
		for _, sample := range motion.GetSamples() {
			if sample == nil || sample.GetStartAngleDeg() == sample.GetEndAngleDeg() {
				continue
			}
			lo := math.Min(sample.GetStartAngleDeg(), sample.GetEndAngleDeg())
			hi := math.Max(sample.GetStartAngleDeg(), sample.GetEndAngleDeg())
			if !found || lo < minAngle {
				minAngle = lo
			}
			if !found || hi > maxAngle {
				maxAngle = hi
			}
			found = true
		}
		if found {
			return minAngle, maxAngle, true
		}
	}
	if angle := profile.GetAngle(); angle > 0 {
		half := angle / 2
		return -half, half, true
	}
	return 0, 0, false
}

func skillRuntimeMaxTargets(skill SkillRuntimeContract) int32 {
	limit := skill.MaxTargets
	for _, profile := range skill.Hitboxes {
		if profile != nil && profile.GetMaxTargets() > 0 && (limit <= 0 || profile.GetMaxTargets() < limit) {
			limit = profile.GetMaxTargets()
		}
	}
	if limit <= 0 {
		return defaultImpactTargetLimit
	}
	return limit
}

func skillRangeToCM(value float64) float64 {
	if value <= 0 {
		return 0
	}
	if value < skillRangeMetersToCMThreshold {
		return value * centimetersPerMeter
	}
	return value
}

func closestPointOnSegment(a, b, p vector) vector {
	ab := vector{x: b.x - a.x, y: b.y - a.y}
	abLenSq := ab.x*ab.x + ab.y*ab.y
	if abLenSq <= 0.0001 {
		return a
	}
	ap := vector{x: p.x - a.x, y: p.y - a.y}
	t := (ap.x*ab.x + ap.y*ab.y) / abLenSq
	t = math.Max(0, math.Min(1, t))
	return vector{x: a.x + ab.x*t, y: a.y + ab.y*t, z: a.z + (b.z-a.z)*t}
}

func signedAngleDeg(forward, target vector) float64 {
	forward = normalize(forward)
	target = normalize(target)
	if forward == (vector{}) || target == (vector{}) {
		return 0
	}
	cross := forward.x*target.y - forward.y*target.x
	dot := forward.x*target.x + forward.y*target.y
	return math.Atan2(cross, dot) * 180 / math.Pi
}

func maxFloat64(values ...float64) float64 {
	max := 0.0
	for _, value := range values {
		if value > max {
			max = value
		}
	}
	return max
}

func clampMin(value, min float64) float64 {
	if value < min {
		return min
	}
	return value
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
