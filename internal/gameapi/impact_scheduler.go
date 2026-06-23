package gameapi

import (
	"fmt"
	"time"
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
	RequireTime bool
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
	}
}
