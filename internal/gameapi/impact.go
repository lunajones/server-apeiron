package gameapi

import (
	"math"
	"sort"
	"time"

	dbv1 "db-apeiron/gen/apeiron/v1"
)

const (
	defaultImpactTargetLimit      int32   = 1
	skillRangeMetersToCMThreshold float64 = 25
	centimetersPerMeter           float64 = 100
)

type runtimeSkillImpact struct {
	SourceID               uint64
	TargetID               uint64
	SkillID                string
	ImpactType             string
	ImpactResponseProfile  string
	StatusApplied          []string
	ControlType            string
	ControlReleasePolicy   string
	ControlDistanceCM      float64
	ControlSpeedCMS        float64
	ControlDirectionPolicy string
	DamageApplied          float64
	PostureApplied         float64
	Blocked                bool
	Parried                bool
	Evaded                 bool
	Reason                 string
	TargetPipelineState    string
	TargetIFrame           bool
}

type runtimeSkillImpactCandidate struct {
	target   *entityState
	distCM   float64
	profile  *dbv1.SkillHitboxProfile
	priority int32
}

func (r *Runtime) applySkillImpact(source *entityState, skill SkillRuntimeContract, start, end, dir vector) []runtimeSkillImpact {
	return r.applySkillImpactAt(source, skill, start, end, dir, skillImpactEvaluationElapsedMS(skill))
}

func (r *Runtime) applySkillImpactAt(source *entityState, skill SkillRuntimeContract, start, end, dir vector, elapsedMS float64) []runtimeSkillImpact {
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
		profile, ok := skillRuntimeHitboxContainsAt(skill, start, end, dir, target.position, elapsedMS)
		if !ok {
			continue
		}
		candidates = append(candidates, runtimeSkillImpactCandidate{
			target:   target,
			distCM:   distance(start, target.position),
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
		impact, ok := r.resolveRuntimeSkillImpact(source, target, skill, candidate.profile, start, dir)
		if !ok {
			continue
		}
		target.health = clampMin(target.health-impact.DamageApplied, 0)
		target.posture = clampMin(target.posture-impact.PostureApplied, 0)
		r.respawnPlayerAfterFatalDamageLocked(target)
		impacts = append(impacts, impact)
	}
	return impacts
}

func (r *Runtime) respawnPlayerAfterFatalDamageLocked(target *entityState) {
	if r == nil || target == nil || target.entityType != "player" || target.health > 0 {
		return
	}
	if target.maxHealth > 0 {
		target.health = target.maxHealth
	} else {
		target.health = 100
		target.maxHealth = 100
	}
	if target.maxPosture > 0 {
		target.posture = target.maxPosture
	}
	if target.maxStamina > 0 {
		target.stamina = target.maxStamina
	}
	target.velocity = vector{}
	target.movementState = "grounded"
	target.combatState = "ready"
	target.skillState = "idle"
	target.skillRuntime = nil
	target.actionInstance = nil
	target.actionMotion = nil
	target.actionHandoffUntil = time.Time{}
	target.actionHandoffAction = ""
	target.actionLockedUntil = time.Time{}
	target.actionLockReason = ""
	target.locomotion = r.locomotion("grounded", "respawn", "", "complete", target.position, target.position, 0)
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
	return skillRuntimeHitboxContainsAt(skill, start, end, dir, target, skillImpactEvaluationElapsedMS(skill))
}

func skillRuntimeHitboxContainsAt(skill SkillRuntimeContract, start, end, dir vector, target vector, elapsedMS float64) (*dbv1.SkillHitboxProfile, bool) {
	for _, profile := range skill.Hitboxes {
		if profile == nil {
			continue
		}
		if skillHitboxProfileContainsAt(skill, profile, start, end, dir, target, elapsedMS) {
			return profile, true
		}
	}
	return nil, false
}

func skillHitboxProfileContains(skill SkillRuntimeContract, profile *dbv1.SkillHitboxProfile, start, end, dir vector, target vector) bool {
	return skillHitboxProfileContainsAt(skill, profile, start, end, dir, target, skillImpactEvaluationElapsedMS(skill))
}

func skillStaticHitboxProfileContains(skill SkillRuntimeContract, profile *dbv1.SkillHitboxProfile, start, end, dir vector, target vector) bool {
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
	return maxFloat64(profile.GetLength(), profile.GetRadius(), skillRangeToCM(skill.Range))
}

func skillHitboxLaneHalfWidthCM(profile *dbv1.SkillHitboxProfile) float64 {
	return maxFloat64(profile.GetRadius(), profile.GetSizeX()/2, profile.GetSizeY()/2)
}

func skillHitboxAngleWindow(profile *dbv1.SkillHitboxProfile) (float64, float64, bool) {
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
