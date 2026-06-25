package gameapi

import (
	"fmt"
	"strings"
	"time"

	gamev1 "server-apeiron/gen/apeiron/game/v1"
	creatureai "server-apeiron/internal/ai"
	"server-apeiron/internal/combat/actionruntime"
	"server-apeiron/internal/domain/ids"
	"server-apeiron/internal/movement"
)

type creatureActionRuntimeUpdate struct {
	Started           bool
	Active            bool
	RootMotionApplied bool
	Phase             actionruntime.Phase
}

func (r *Runtime) applyCreatureActionRuntimeLocked(creature *entityState, target *entityState, decision creatureai.Decision, contract SkillRuntimeContract, start vector, now time.Time) creatureActionRuntimeUpdate {
	if creatureActionTransitionActive(creature) {
		r.refreshCreatureActionTransitionLocked(creature, now)
		return creatureActionRuntimeUpdate{Active: true, RootMotionApplied: true, Phase: actionruntime.PhaseRecovery}
	}
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
		creature.creatureActiveSetupPolicyID = decision.SetupPolicyID
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
		r.completeCreatureActionRuntimeLocked(creature, now)
		return creatureActionRuntimeUpdate{Started: started, Phase: phase}
	}
	creature.skillState = string(phase)
	creature.combatState = "committed"
	rootMotionApplied := r.applyCreatureSkillRootMotionLocked(creature, target, decision, contract, instance, now)
	// Refresh the attack-yaw latch before the hitbox is scheduled so a committed action
	// damages along its latched line, not the moving target's live bearing.
	r.updateCreatureActionOrientationLatchLocked(creature, target, contract, instance, now)
	if target != nil {
		r.enqueueCreatureSkillImpactLocked(creature, target, contract, now)
	}
	return creatureActionRuntimeUpdate{Started: started, Active: true, RootMotionApplied: rootMotionApplied, Phase: phase}
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
	if creatureActionTransitionActive(creature) {
		r.refreshCreatureActionTransitionLocked(creature, now)
		return
	}
	if entityActionRuntimeActiveAt(creature, now) {
		r.interruptEntityActionRuntimeLocked(creature, now, "")
	}
	r.completeCreatureActionRuntimeLocked(creature, now)
}

func (r *Runtime) newCreatureActionInstance(creature *entityState, skillID string, contract SkillRuntimeContract, start vector, now time.Time) actionruntime.Instance {
	timing := creatureActionTimingFromSkillContract(contract)
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

func creatureActionTimingFromSkillContract(contract SkillRuntimeContract) actionruntime.Timing {
	timing := actionTimingFromSkillContract(contract)
	movementDuration := movement.ActionDuration(contract.MovementAction)
	if movementDuration <= 0 {
		return timing
	}
	movementOffset := creatureSkillMovementStartOffset(timing, contract)
	actionDuration := timing.Windup + timing.Active + timing.Recovery
	requiredDuration := movementOffset + movementDuration
	if requiredDuration > actionDuration {
		timing.Recovery += requiredDuration - actionDuration
		timing.ActionLock = timing.Windup + timing.Active + timing.Recovery
	}
	return timing
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

func (r *Runtime) applyCreatureSkillRootMotionLocked(creature *entityState, target *entityState, decision creatureai.Decision, contract SkillRuntimeContract, instance *actionruntime.Instance, now time.Time) bool {
	if creature == nil || instance == nil || contract.MovementAction.ID == "" {
		return false
	}
	if movement.ActionDistance(contract.MovementAction, 0) <= 0 || movement.ActionDuration(contract.MovementAction) <= 0 {
		return false
	}
	envelope := creatureActionMovementEnvelopeAt(*instance, contract, now)
	if !envelope.RootMotionActive {
		return false
	}
	if creature.actionMotion == nil || creature.actionMotion.SkillID != decision.SelectedSkill || creature.actionMotion.CommandID != instance.InstanceID {
		r.startCreatureSkillRootMotionLocked(creature, target, decision, contract, *instance, envelope.MovementStartsAt)
	}
	if creature.actionMotion == nil {
		return false
	}
	r.advanceActionMotionLocked(creature, now)
	r.reaimCreatureLungeAtTakeoffLocked(creature, target, contract, instance, now)
	return true
}

// reaimCreatureLungeAtTakeoffLocked commits a takeoff-latching action (lunge) to the current
// target at the moment it goes airborne, then leaves it fixed. The remaining path is rotated
// around the creature's current position so travel stays continuous — only the heading snaps.
// This is what makes the pre-commit window meaningful: the body aligns during pre-commit, then
// the lunge line locks onto the target at takeoff instead of staying fixed to the pre-commit
// start aim. It fires once per action and only for latch_at_takeoff orientation policies.
func (r *Runtime) reaimCreatureLungeAtTakeoffLocked(creature *entityState, target *entityState, contract SkillRuntimeContract, instance *actionruntime.Instance, now time.Time) {
	if creature == nil || target == nil || instance == nil {
		return
	}
	motion := creature.actionMotion
	if motion == nil || motion.ReaimedAtTakeoff {
		return
	}
	if contract.Orientation == nil || !strings.EqualFold(strings.TrimSpace(contract.Orientation.GetAttackYawLatchPolicy()), "latch_at_takeoff") {
		return
	}
	envelope := creatureActionMovementEnvelopeAt(*instance, contract, now)
	if envelope.AirborneStartsAt.IsZero() || now.Before(envelope.AirborneStartsAt) {
		return
	}
	newDir := normalize(vector{x: target.position.x - creature.position.x, y: target.position.y - creature.position.y})
	if newDir == (vector{}) {
		return
	}
	start2D := vector{x: motion.StartPosition.x, y: motion.StartPosition.y}
	cur2D := vector{x: creature.position.x, y: creature.position.y}
	distSoFar := distance(start2D, cur2D)
	motion.StartPosition = vector{x: creature.position.x - newDir.x*distSoFar, y: creature.position.y - newDir.y*distSoFar, z: motion.StartPosition.z}
	motion.Direction = newDir
	motion.ProjectedPosition = vector{x: motion.StartPosition.x + newDir.x*motion.TotalDistanceCM, y: motion.StartPosition.y + newDir.y*motion.TotalDistanceCM, z: motion.ProjectedPosition.z}
	motion.ReaimedAtTakeoff = true
}

func (r *Runtime) startCreatureSkillRootMotionLocked(creature *entityState, target *entityState, decision creatureai.Decision, contract SkillRuntimeContract, instance actionruntime.Instance, rootStart time.Time) {
	if creature == nil {
		return
	}
	dir := creatureSkillRootDirection(creature, target, decision, contract)
	fullMotion := movement.ResolveActionMotion(movement.ActionMotionInput{
		Position:  toDomainVector(creature.position),
		Direction: toDomainVector(dir),
		Contract:  contract.MovementAction,
	})
	if fullMotion.Stopped || fullMotion.DistanceCM <= 0 {
		return
	}
	contact := creatureActionContactRuntimeFromContract(contract)
	var contactTargetID uint64
	if target != nil {
		contactTargetID = target.id
	}
	startPosition := creature.position
	if creatureSkillUsesVerticalRoot(contract) {
		startPosition = r.entityGroundRootPosition(creature, startPosition)
		creature.position = startPosition
	}
	creature.actionMotion = &actionMotionState{
		SkillID:           decision.SelectedSkill,
		CommandID:         instance.InstanceID,
		Sequence:          instance.CommandSequence,
		MotionSource:      "skill_root",
		StartedAt:         rootStart,
		StartPosition:     startPosition,
		ProjectedPosition: fromDomainVector(fullMotion.Projected),
		Direction:         dir,
		Contract:          contract.MovementAction,
		NormalInputPolicy: contract.NormalInputPolicy,
		TotalDistanceCM:   fullMotion.DistanceCM,
		ContactPolicy:     contact.Policy,
		ContactTargetID:   contactTargetID,
		AllowsPassthrough: contact.AllowsPassthrough,
		StopsAtContact:    contact.StopsAtContact,
		ContactStopCM:     contact.StopDistanceCM,
		UseVerticalRoot:   creatureSkillUsesVerticalRoot(contract),
	}
}

func creatureSkillUsesVerticalRoot(contract SkillRuntimeContract) bool {
	if !strings.EqualFold(strings.TrimSpace(contract.MovementAction.ActionType), "leap") {
		return false
	}
	return contract.MovementAction.AirborneDurationMS > 0 ||
		len(contract.MovementAction.VerticalCurveSamples) > 0 ||
		strings.TrimSpace(contract.MovementAction.VerticalMotionModel) != ""
}

func creatureSkillRootDirection(creature *entityState, target *entityState, decision creatureai.Decision, contract SkillRuntimeContract) vector {
	if creatureSkillRootPrefersDecisionDirection(contract) {
		dir := fromDomainVector(flattenDomainDirection(decision.Direction))
		if dir != (vector{}) {
			return normalize(dir)
		}
	}
	if target != nil && creature != nil {
		dir := normalize(vector{x: target.position.x - creature.position.x, y: target.position.y - creature.position.y})
		if dir != (vector{}) {
			return dir
		}
	}
	dir := fromDomainVector(flattenDomainDirection(decision.Direction))
	if dir != (vector{}) {
		return normalize(dir)
	}
	if creature != nil {
		return yawVector(creature.yaw)
	}
	return vector{x: 1}
}

func creatureSkillRootPrefersDecisionDirection(contract SkillRuntimeContract) bool {
	policy := strings.TrimSpace(contract.ContactPolicy)
	if policy == "" {
		policy = contract.MovementAction.ContactPolicy
	}
	policy = strings.ToLower(strings.TrimSpace(policy))
	return strings.Contains(policy, "lateral_counter")
}

func creatureSkillMovementStartAt(instance actionruntime.Instance, contract SkillRuntimeContract) time.Time {
	return instance.StartedAt.Add(creatureSkillMovementStartOffset(instance.Timing, contract))
}

func creatureSkillMovementStartOffset(timing actionruntime.Timing, contract SkillRuntimeContract) time.Duration {
	switch strings.ToLower(strings.TrimSpace(contract.StartsAtPhase)) {
	case "windup", "startup", "accepted", "start", "":
		return 0
	case "recovery":
		return timing.Windup + timing.Active
	case "active", "cast":
		return timing.Windup
	default:
		return timing.Windup
	}
}
