package gameapi

import (
	"time"

	gamev1 "server-apeiron/gen/apeiron/game/v1"
	"server-apeiron/internal/combat/actionruntime"
)

const (
	skillRuntimeStateIdle        = "idle"
	skillRuntimeStateInterrupted = "interrupted"
)

func (r *Runtime) completeCreatureActionRuntimeLocked(creature *entityState, now time.Time) {
	if creature == nil {
		return
	}
	creature.actionInstance = nil
	creature.actionMotion = nil
	r.publishEntityTerminalSkillRuntimeLocked(creature, skillRuntimeStateIdle, now)
	creature.skillState = "idle"
	creature.combatState = "ready"
}

func (r *Runtime) interruptEntityActionRuntimeLocked(entity *entityState, now time.Time, preserveMotionSource string) bool {
	if entity == nil {
		return false
	}
	cancelled := r.cancelEntityActionImpactScheduleLocked(entity)
	entity.actionInstance = nil
	if entity.actionMotion != nil && entity.actionMotion.MotionSource != preserveMotionSource {
		entity.actionMotion = nil
	}
	r.publishEntityTerminalSkillRuntimeLocked(entity, skillRuntimeStateInterrupted, now)
	return cancelled
}

func (r *Runtime) cancelEntityActionImpactScheduleLocked(entity *entityState) bool {
	if entity == nil || entity.actionInstance == nil {
		return false
	}
	return r.cancelSkillImpactScheduleLocked(
		entity,
		entity.actionInstance.SkillID.String(),
		entity.actionInstance.InstanceID,
		entity.actionInstance.StartedAt,
	)
}

func (r *Runtime) publishEntityTerminalSkillRuntimeLocked(entity *entityState, state string, now time.Time) {
	if entity == nil {
		return
	}
	lastResolved := now.UnixMilli()
	if entity.skillRuntime == nil {
		entity.skillRuntime = &gamev1.SkillRuntimeState{}
	}
	entity.skillRuntime.State = state
	entity.skillRuntime.LastResolvedAtMs = lastResolved
	if state == skillRuntimeStateIdle {
		entity.skillRuntime.CurrentSkillId = ""
		entity.skillRuntime.StartedAtMs = 0
	}
}

func entityActionRuntimeActiveAt(entity *entityState, now time.Time) bool {
	if entity == nil || entity.actionInstance == nil {
		return false
	}
	return entity.actionInstance.PhaseAt(now) != actionruntime.PhaseComplete
}
