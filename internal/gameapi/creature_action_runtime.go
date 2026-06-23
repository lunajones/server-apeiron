package gameapi

import (
	"fmt"
	"strings"
	"time"

	gamev1 "server-apeiron/gen/apeiron/game/v1"
	creatureai "server-apeiron/internal/ai"
	"server-apeiron/internal/combat/actionruntime"
	"server-apeiron/internal/domain/ids"
)

type creatureActionRuntimeUpdate struct {
	Started bool
	Active  bool
	Phase   actionruntime.Phase
}

func (r *Runtime) applyCreatureActionRuntimeLocked(creature *entityState, target *entityState, decision creatureai.Decision, contract SkillRuntimeContract, start vector, now time.Time) creatureActionRuntimeUpdate {
	if creature == nil || !creatureai.PublishesSkill(decision.Action) || decision.SelectedSkill == "" {
		r.clearCreatureActionRuntimeLocked(creature, now)
		return creatureActionRuntimeUpdate{Phase: actionruntime.PhaseComplete}
	}
	skillID := decision.SelectedSkill
	instance := creature.actionInstance
	started := false
	if r.shouldStartCreatureActionInstanceLocked(creature, skillID, now) {
		next := r.newCreatureActionInstance(creature, skillID, contract, start, now)
		creature.actionInstance = &next
		instance = &next
		started = true
		r.spendCreatureSkillStaminaLocked(creature, skillID, contract)
		r.startCreatureSkillCooldownLocked(creature, skillID, contract, now)
	}
	if instance == nil {
		r.clearCreatureActionRuntimeLocked(creature, now)
		return creatureActionRuntimeUpdate{Phase: actionruntime.PhaseComplete}
	}
	phase := instance.PhaseAt(now)
	cooldownEnd := r.creatureSkillCooldownEndLocked(creature, skillID)
	creature.skillRuntime = &gamev1.SkillRuntimeState{
		CurrentSkillId:   skillID,
		State:            string(phase),
		StartedAtMs:      instance.StartedAt.UnixMilli(),
		CooldownEndMs:    cooldownEnd.UnixMilli(),
		LastResolvedAtMs: now.UnixMilli(),
	}
	if phase == actionruntime.PhaseComplete {
		creature.skillState = "idle"
		creature.combatState = "ready"
		creature.actionMotion = nil
		return creatureActionRuntimeUpdate{Started: started, Phase: phase}
	}
	creature.skillState = string(phase)
	creature.combatState = "committed"
	if target != nil {
		r.resolveCreatureSkillImpactLocked(creature, target, contract, now)
	}
	return creatureActionRuntimeUpdate{Started: started, Active: true, Phase: phase}
}

func (r *Runtime) shouldStartCreatureActionInstanceLocked(creature *entityState, skillID string, now time.Time) bool {
	if creature == nil || skillID == "" || creature.actionInstance == nil || creature.skillRuntime == nil {
		return true
	}
	if creature.skillRuntime.GetCurrentSkillId() != skillID || creature.skillRuntime.GetStartedAtMs() <= 0 {
		return true
	}
	if creature.actionInstance.SkillID.String() != skillID {
		return true
	}
	return creature.actionInstance.PhaseAt(now) == actionruntime.PhaseComplete
}

func (r *Runtime) clearCreatureActionRuntimeLocked(creature *entityState, now time.Time) {
	if creature == nil {
		return
	}
	creature.skillRuntime = &gamev1.SkillRuntimeState{State: "idle", LastResolvedAtMs: now.UnixMilli()}
	creature.skillState = "idle"
	creature.combatState = "ready"
	creature.actionInstance = nil
	creature.actionMotion = nil
}

func (r *Runtime) newCreatureActionInstance(creature *entityState, skillID string, contract SkillRuntimeContract, start vector, now time.Time) actionruntime.Instance {
	timing := actionTimingFromSkillContract(contract)
	commandID := creatureActionCommandID(creature, skillID, r.tick)
	return actionruntime.NewInstance(actionruntime.NewInstanceSpec{
		InstanceID:           actionruntime.NewInstanceID(ids.RuntimeEntityID(creature.id), skillID, commandID, r.tick, r.tick),
		EntityID:             ids.RuntimeEntityID(creature.id),
		ActorKind:            actionruntime.ActorKindCreature,
		ActionKind:           creatureActionKindForSkill(skillID),
		SkillID:              ids.SkillID(skillID),
		CommandID:            commandID,
		CommandSequence:      r.tick,
		ServerActionSequence: r.tick,
		StartedAt:            now,
		Timing:               timing,
		Cooldown:             timing.Cooldown,
		MovementContract:     movementActionContractForRuntime(contract.MovementAction),
		HasMovementContract:  contract.MovementAction.ID != "",
		ActionStartPosition:  toDomainVector(start),
		MovementLockedUntil:  now.Add(timing.ActionLock),
		GlobalLockedUntil:    now.Add(timing.Cooldown),
		RecoveryEndsAt:       now.Add(timing.Windup + timing.Active + timing.Recovery),
	})
}

func actionTimingFromSkillContract(contract SkillRuntimeContract) actionruntime.Timing {
	return actionruntime.Timing{
		Windup:     durationFromMS(contract.WindupMS),
		Active:     durationFromMS(contract.ActiveMS),
		Recovery:   durationFromMS(contract.RecoveryMS),
		Cooldown:   durationFromMS(contract.CooldownMS),
		ActionLock: durationFromMS(contract.WindupMS + contract.ActiveMS + contract.RecoveryMS),
	}
}

func creatureActionKindForSkill(skillID string) actionruntime.ActionKind {
	switch strings.TrimSpace(skillID) {
	case "":
		return actionruntime.ActionKindActiveSkill
	default:
		return actionruntime.ActionKindActiveSkill
	}
}

func creatureActionCommandID(creature *entityState, skillID string, tick uint64) string {
	entityID := uint64(0)
	if creature != nil {
		entityID = creature.id
	}
	return fmt.Sprintf("ai:%d:%s:%d", entityID, skillID, tick)
}

func (r *Runtime) creatureSkillCooldownEndLocked(creature *entityState, skillID string) time.Time {
	if creature == nil || skillID == "" || creature.creatureCooldownUntil == nil {
		return time.Time{}
	}
	return creature.creatureCooldownUntil[skillID]
}

func creatureActionLocomotionPhase(creature *entityState, now time.Time) string {
	if creature == nil || creature.actionInstance == nil {
		return "active"
	}
	phase := creature.actionInstance.PhaseAt(now)
	if phase == actionruntime.PhaseComplete {
		return string(actionruntime.PhaseRecovery)
	}
	return string(phase)
}

func creatureActionMotionComplete(creature *entityState, now time.Time) bool {
	if creature == nil || creature.actionInstance == nil {
		return true
	}
	return creature.actionInstance.PhaseAt(now) == actionruntime.PhaseComplete
}
