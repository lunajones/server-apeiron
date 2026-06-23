package gameapi

import (
	"fmt"
	"time"

	"server-apeiron/internal/movement"
)

type skillImpactSchedule struct {
	InstanceID  string
	StartedAt   time.Time
	Source      *entityState
	Skill       SkillRuntimeContract
	Start       vector
	End         vector
	Direction   vector
	ElapsedMS   float64
	PreviousMS  float64
	RequireTime bool
	TrackSource bool
}

func (r *Runtime) enqueueSkillImpactScheduleLocked(schedule skillImpactSchedule) bool {
	if r == nil || schedule.Source == nil || schedule.Skill.SkillID == "" {
		return false
	}
	key := skillImpactScheduleKey(schedule.Source, schedule.Skill.SkillID, schedule.InstanceID, schedule.StartedAt)
	if key == "" {
		return false
	}
	if r.impacts == nil {
		r.impacts = make(map[string]skillImpactSchedule)
	}
	if schedule.Source.resolvedSkillImpacts != nil {
		if _, resolved := schedule.Source.resolvedSkillImpacts[key]; resolved {
			return false
		}
	}
	if _, exists := r.impacts[key]; exists {
		return false
	}
	r.impacts[key] = schedule
	return true
}

func (r *Runtime) runPendingSkillImpactSchedulesLocked(now time.Time) []runtimeSkillImpact {
	if r == nil || len(r.impacts) == 0 {
		return nil
	}
	impacts := make([]runtimeSkillImpact, 0, len(r.impacts))
	for key, schedule := range r.impacts {
		if schedule.Source == nil || schedule.Source.health <= 0 {
			delete(r.impacts, key)
			continue
		}
		previousMS := schedule.ElapsedMS
		schedule.ElapsedMS = skillImpactScheduleElapsedMS(schedule, now)
		schedule.PreviousMS = previousMS
		if schedule.TrackSource {
			schedule.Start, schedule.End, schedule.Direction = skillImpactScheduleTrace(schedule)
		}
		evaluationMS, crossedWindow := skillImpactScheduleEvaluationElapsedMS(schedule)
		if !crossedWindow {
			if skillImpactScheduleExpired(schedule) {
				delete(r.impacts, key)
			} else {
				r.impacts[key] = schedule
			}
			continue
		}
		schedule.ElapsedMS = evaluationMS
		resolved := r.resolveSkillImpactScheduleLocked(schedule)
		schedule.ElapsedMS = skillImpactScheduleElapsedMS(schedule, now)
		if len(resolved) > 0 {
			impacts = append(impacts, resolved...)
			delete(r.impacts, key)
			continue
		}
		if skillImpactScheduleExpired(schedule) {
			delete(r.impacts, key)
			continue
		}
		r.impacts[key] = schedule
	}
	return impacts
}

func (r *Runtime) resolveSkillImpactScheduleLocked(schedule skillImpactSchedule) []runtimeSkillImpact {
	if r == nil || schedule.Source == nil || schedule.Skill.SkillID == "" {
		return nil
	}
	if schedule.RequireTime && !skillHasTemporalImpactWindowAt(schedule.Skill, schedule.ElapsedMS) {
		return nil
	}
	key := skillImpactScheduleKey(schedule.Source, schedule.Skill.SkillID, schedule.InstanceID, schedule.StartedAt)
	if key == "" {
		return nil
	}
	if schedule.Source.resolvedSkillImpacts != nil {
		if _, resolved := schedule.Source.resolvedSkillImpacts[key]; resolved {
			return nil
		}
	}
	impacts := r.applySkillImpactAt(schedule.Source, schedule.Skill, schedule.Start, schedule.End, schedule.Direction, schedule.ElapsedMS)
	if len(impacts) == 0 {
		return nil
	}
	if schedule.Source.resolvedSkillImpacts == nil {
		schedule.Source.resolvedSkillImpacts = map[string]struct{}{}
	}
	schedule.Source.resolvedSkillImpacts[key] = struct{}{}
	return impacts
}

func skillImpactScheduleKey(source *entityState, skillID string, instanceID string, startedAt time.Time) string {
	if source == nil || skillID == "" {
		return ""
	}
	if instanceID != "" {
		return fmt.Sprintf("%d:%s:%s", source.id, skillID, instanceID)
	}
	if !startedAt.IsZero() {
		return fmt.Sprintf("%d:%s:%d", source.id, skillID, startedAt.UnixMilli())
	}
	return ""
}

func skillImpactScheduleFromActionInstance(source *entityState, skill SkillRuntimeContract, instanceID string, startedAt time.Time, start, end, dir vector, elapsedMS float64) skillImpactSchedule {
	return skillImpactSchedule{
		InstanceID:  instanceID,
		StartedAt:   startedAt,
		Source:      source,
		Skill:       skill,
		Start:       start,
		End:         end,
		Direction:   dir,
		ElapsedMS:   elapsedMS,
		RequireTime: true,
		TrackSource: true,
	}
}

func skillImpactScheduleElapsedMS(schedule skillImpactSchedule, now time.Time) float64 {
	if !schedule.StartedAt.IsZero() {
		elapsed := now.Sub(schedule.StartedAt).Seconds() * 1000
		if elapsed > 0 {
			return elapsed
		}
	}
	return schedule.ElapsedMS
}

func skillImpactScheduleExpired(schedule skillImpactSchedule) bool {
	if !schedule.RequireTime {
		return true
	}
	windowEnd, ok := skillLatestImpactWindowEndMS(schedule.Skill)
	if !ok {
		return true
	}
	return schedule.ElapsedMS-impactTemporalEpsilon > windowEnd
}

func skillImpactScheduleEvaluationElapsedMS(schedule skillImpactSchedule) (float64, bool) {
	if !schedule.RequireTime {
		return schedule.ElapsedMS, true
	}
	bestStart := 0.0
	bestEnd := 0.0
	found := false
	for _, profile := range schedule.Skill.Hitboxes {
		if profile == nil {
			continue
		}
		startMS := float64(profile.GetHitboxStartMs())
		endMS := float64(profile.GetHitboxEndMs())
		if endMS <= startMS {
			endMS = startMS
		}
		if schedule.ElapsedMS+impactTemporalEpsilon < startMS || schedule.PreviousMS-impactTemporalEpsilon > endMS {
			continue
		}
		if !found || startMS < bestStart {
			bestStart = startMS
			bestEnd = endMS
			found = true
		}
	}
	if !found {
		return schedule.ElapsedMS, false
	}
	if schedule.ElapsedMS < bestStart {
		return bestStart, true
	}
	if schedule.ElapsedMS > bestEnd {
		return bestEnd, true
	}
	return schedule.ElapsedMS, true
}

func skillLatestImpactWindowEndMS(skill SkillRuntimeContract) (float64, bool) {
	best := -1.0
	for _, profile := range skill.Hitboxes {
		if profile == nil {
			continue
		}
		endMS := float64(profile.GetHitboxEndMs())
		if endMS <= float64(profile.GetHitboxStartMs()) {
			endMS = float64(profile.GetHitboxStartMs())
		}
		if endMS > best {
			best = endMS
		}
	}
	return best, best >= 0
}

func skillImpactScheduleTrace(schedule skillImpactSchedule) (vector, vector, vector) {
	start := schedule.Start
	if schedule.Source != nil {
		start = schedule.Source.position
	}
	dir := normalize(schedule.Direction)
	if dir == (vector{}) && schedule.Source != nil {
		dir = yawVector(schedule.Source.yaw)
	}
	if dir == (vector{}) {
		dir = vector{x: 1}
	}
	reach := skillImpactTraceReachCM(schedule.Skill)
	if reach <= 0 && schedule.End != (vector{}) {
		reach = distance(start, schedule.End)
	}
	end := vector{x: start.x + dir.x*reach, y: start.y + dir.y*reach, z: start.z}
	return start, end, dir
}

func skillImpactTraceReachCM(skill SkillRuntimeContract) float64 {
	reach := skillRangeToCM(skill.Range)
	if reach <= 0 {
		reach = movement.ActionDistance(skill.MovementAction, 0)
	}
	if reach <= 0 {
		reach = maxSkillHitboxReachCM(skill)
	}
	return reach
}
