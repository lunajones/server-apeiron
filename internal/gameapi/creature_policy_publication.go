package gameapi

import (
	"math"
	"strings"
	"time"

	gamev1 "server-apeiron/gen/apeiron/game/v1"
	creatureai "server-apeiron/internal/ai"
	domainmath "server-apeiron/internal/domain/math"
	"server-apeiron/internal/movement"
)

type creatureDecisionMotion struct {
	Start  vector
	Motion movement.MotionResult
}

func resolveGroundedCreatureDecisionMotion(creature *entityState, target *entityState, decision creatureai.Decision) creatureDecisionMotion {
	start := vector{}
	if creature != nil {
		start = creature.position
	}
	motion := movement.ResolveConstantStep(movement.ConstantStepInput{
		Position:         toDomainVector(start),
		Direction:        flattenDomainDirection(decision.Direction),
		SpeedCMPerSecond: decision.SpeedCMPerSec,
		TickRate:         tickRate,
	})
	projected := fromDomainVector(motion.Projected)
	projected.z = start.z
	projected = clampCreatureTacticalProjectionToBodyContact(start, projected, creature, target)
	velocity := fromDomainVector(motion.Velocity)
	velocity = scale(vector{x: projected.x - start.x, y: projected.y - start.y, z: projected.z - start.z}, tickRate)
	velocity.z = 0
	motion.Projected = toDomainVector(projected)
	motion.Velocity = toDomainVector(velocity)
	motion.Direction = flattenDomainDirection(motion.Direction)
	motion.DistanceCM = distance(start, projected)
	motion.SpeedCMPerSecond = length(velocity)
	return creatureDecisionMotion{Start: start, Motion: motion}
}

func clampCreatureTacticalProjectionToBodyContact(start vector, projected vector, creature *entityState, target *entityState) vector {
	if creature == nil || target == nil {
		return projected
	}
	minSeparation := runtimeEntityRadiusCM(creature) + runtimeEntityRadiusCM(target)
	if minSeparation <= 0 || distance(projected, target.position) >= minSeparation {
		return projected
	}
	away := normalize(vector{x: projected.x - target.position.x, y: projected.y - target.position.y})
	if away == (vector{}) {
		away = normalize(vector{x: start.x - target.position.x, y: start.y - target.position.y})
	}
	if away == (vector{}) {
		away = yawVector(target.yaw + 180)
	}
	out := add(target.position, scale(away, minSeparation))
	out.z = start.z
	return out
}

func applyCreatureDecisionMotion(creature *entityState, target *entityState, decision creatureai.Decision, resolved creatureDecisionMotion) {
	if creature == nil {
		return
	}
	if creatureActionTransitionActive(creature) {
		return
	}
	creature.position = fromDomainVector(resolved.Motion.Projected)
	creature.velocity = fromDomainVector(resolved.Motion.Velocity)
	if target != nil {
		targetYaw := vectorYaw(normalize(vector{x: target.position.x - creature.position.x, y: target.position.y - creature.position.y}))
		if creatureDecisionUsesSideOnBody(decision) {
			if moveDir := fromDomainVector(flattenDomainDirection(decision.Direction)); moveDir != (vector{}) {
				targetYaw = vectorYaw(moveDir)
			}
		}
		creature.yaw = approachCreatureYaw(creature.yaw, targetYaw, decision.TurnRateDegPerSec)
	}
	creature.movementState = decision.Action
	if creatureai.PublishesSkill(decision.Action) {
		creature.skillState = decision.Action
	} else {
		creature.skillState = "idle"
	}
}

func creatureDecisionUsesSideOnBody(decision creatureai.Decision) bool {
	tactic := strings.ToLower(strings.TrimSpace(decision.MovementTactic))
	if tactic == "" {
		tactic = strings.ToLower(strings.TrimSpace(decision.Action))
	}
	return strings.Contains(tactic, "circle") ||
		strings.Contains(tactic, "flank") ||
		strings.Contains(tactic, "orbit") ||
		strings.Contains(tactic, "setup")
}

func approachCreatureYaw(current, target, turnRateDegPerSec float64) float64 {
	if turnRateDegPerSec <= 0 {
		return target
	}
	maxStep := turnRateDegPerSec / tickRate
	delta := normalizeYawDelta(target - current)
	if math.Abs(delta) <= maxStep {
		return normalizeYaw(target)
	}
	if delta > 0 {
		return normalizeYaw(current + maxStep)
	}
	return normalizeYaw(current - maxStep)
}

func normalizeYawDelta(delta float64) float64 {
	for delta > 180 {
		delta -= 360
	}
	for delta < -180 {
		delta += 360
	}
	return delta
}

func normalizeYaw(yaw float64) float64 {
	for yaw >= 360 {
		yaw -= 360
	}
	for yaw < 0 {
		yaw += 360
	}
	return yaw
}

func (r *Runtime) publishWolfLocomotionLocked(wolf *entityState, decision creatureai.Decision, contract SkillRuntimeContract, actionUpdate creatureActionRuntimeUpdate, resolved creatureDecisionMotion, now time.Time) {
	if wolf == nil {
		return
	}
	if creatureActionTransitionActive(wolf) {
		r.publishCreatureActionTransitionLocomotionLocked(wolf, now)
		return
	}
	locoContract := r.contracts.contractForAbility("move")
	action := decision.MovementTactic
	abilityKey := "move"
	if action == "" {
		action = "move"
	}
	if creatureai.PublishesSkill(decision.Action) && actionUpdate.RootMotionApplied && contract.MovementAction.ID != "" {
		locoContract = contract.MovementAction
		action = decision.Action
		abilityKey = decision.SelectedSkill
	}
	wolf.locomotion = locomotionFromContractWithOverrides(locoContract, "active", resolved.Start, wolf.position, r.tick, 0, resolved.Motion.SpeedCMPerSecond, resolved.Motion.DistanceCM)
	wolf.locomotion.MovementMode = "grounded"
	wolf.locomotion.Action = action
	wolf.locomotion.AbilityKey = abilityKey
	if creatureai.PublishesSkill(decision.Action) && actionUpdate.RootMotionApplied {
		wolf.locomotion.Phase = creatureActionLocomotionPhase(wolf, now)
		applyActionInstanceLocomotionTiming(wolf.locomotion, wolf.actionInstance, now)
	}
}

func (r *Runtime) publishWolfAIStateLocked(wolf *entityState, target *entityState, decision creatureai.Decision, policy WolfRuntimePolicy, contract SkillRuntimeContract, actionUpdate creatureActionRuntimeUpdate, rangeCM float64, lungeMinRangeCM float64, lungeMaxRangeCM float64, now time.Time) {
	if wolf == nil {
		return
	}
	if transition := wolf.creatureActionTransition; transition != nil {
		orientation := resolveCreatureActionOrientation(wolf, target, decision, contract, creatureActionMovementEnvelope{}, now)
		applyCreatureOrientationState(wolf, orientation)
		wolf.creatureAI = &gamev1.CreatureAIState{
			MovementTactic:          transition.Kind,
			CombatTactic:            "action_transition",
			Commitment:              "recovering",
			CapabilityId:            policy.CapabilityID,
			ContractId:              policy.ContractID,
			ContractHash:            policy.ContractHash,
			LastReason:              creatureActionTransitionOwner,
			TacticalDestination:     toProto(transition.Endpoint),
			BehaviorFamily:          "beast_harasser",
			CombatRole:              "duelist",
			ActualRangeCm:           rangeCM,
			PathState:               creatureActionTransitionOwner,
			LosState:                "clear",
			SelectedSkillId:         transition.SkillID,
			ProfileSource:           r.contracts.Source,
			SkillRecoveryMs:         int32(transition.EndsAt.Sub(transition.StartedAt).Milliseconds()),
			SkillActionLockMs:       int32(transition.EndsAt.Sub(transition.StartedAt).Milliseconds()),
			SkillMovementType:       transition.ActionType,
			SkillMovementDistanceCm: transition.DistanceAtHandoffCM,
			OrientationPhase:        orientation.Phase,
			OrientationPolicyId:     orientation.OrientationPolicyID,
			EnvelopePolicyId:        orientation.EnvelopePolicyID,
			BodyYawDeg:              orientation.BodyYawDeg,
			FocusYawDeg:             orientation.FocusYawDeg,
			AttackYawDeg:            orientation.AttackYawDeg,
			MovementYawDeg:          orientation.MovementYawDeg,
			PreCommitMs:             orientation.PreCommitMS,
			AirborneMs:              orientation.AirborneMS,
			LandingInertiaMs:        orientation.LandingInertiaMS,
			AttackYawLatchPolicy:    orientation.AttackYawLatchPolicy,
		}
		return
	}
	movementPresentation := creatureSkillMovementPresentation{}
	skillMovementType := ""
	envelope := creatureActionMovementEnvelope{}
	if actionUpdate.RootMotionApplied && contract.MovementAction.ID != "" {
		if wolf.actionInstance != nil {
			envelope = creatureActionMovementEnvelopeAt(*wolf.actionInstance, contract, now)
		}
		movementPresentation = creatureSkillMovementPresentationFromContract(contract)
		skillMovementType = contract.MovementAction.ActionType
	}
	orientation := resolveCreatureActionOrientation(wolf, target, decision, contract, envelope, now)
	applyCreatureOrientationState(wolf, orientation)
	wolf.creatureAI = &gamev1.CreatureAIState{
		MovementTactic:                        decision.MovementTactic,
		CombatTactic:                          decision.CombatTactic,
		Commitment:                            decision.Commitment,
		CapabilityId:                          policy.CapabilityID,
		ContractId:                            policy.ContractID,
		ContractHash:                          policy.ContractHash,
		OrbitSide:                             decision.OrbitSide,
		LastReason:                            decision.Reason,
		TacticalDestination:                   toProto(fromDomainVector(decision.Destination)),
		BehaviorFamily:                        "beast_harasser",
		CombatRole:                            "duelist",
		DecisionScore:                         decision.Score,
		DesiredRangeCm:                        policy.DesiredRangeCM,
		ActualRangeCm:                         rangeCM,
		PathState:                             "direct",
		LosState:                              "clear",
		SelectedSkillId:                       decision.SelectedSkill,
		ProfileSource:                         r.contracts.Source,
		SkillMovementArcHeightCm:              policy.LungeArcHeightCM,
		SkillMovementArcCurve:                 "low_fast",
		SkillMovementTakeoffMs:                movementPresentation.TakeoffMS,
		SkillMovementLandingLockMs:            movementPresentation.LandingLockMS,
		SkillWindupMs:                         contract.WindupMS,
		SkillActiveStartMs:                    contract.WindupMS,
		SkillActiveEndMs:                      contract.WindupMS + contract.ActiveMS,
		SkillRecoveryMs:                       contract.RecoveryMS,
		SkillActionLockMs:                     contract.WindupMS + contract.ActiveMS + contract.RecoveryMS,
		SkillMovementType:                     skillMovementType,
		SkillMovementStartMs:                  movementPresentation.MovementStartMS,
		SkillMovementDurationMs:               movementPresentation.MovementDuration,
		SkillMovementDistanceCm:               movementPresentation.MovementDistance,
		SkillMovementDesiredLandingDistanceCm: lungeMaxRangeCM,
		SkillMovementMinLandingDistanceCm:     lungeMinRangeCM,
		SkillMovementStopAtContactRatio:       movementPresentation.StopAtContactRate,
		OrientationPhase:                      orientation.Phase,
		OrientationPolicyId:                   orientation.OrientationPolicyID,
		EnvelopePolicyId:                      orientation.EnvelopePolicyID,
		BodyYawDeg:                            orientation.BodyYawDeg,
		FocusYawDeg:                           orientation.FocusYawDeg,
		AttackYawDeg:                          orientation.AttackYawDeg,
		MovementYawDeg:                        orientation.MovementYawDeg,
		PreCommitMs:                           orientation.PreCommitMS,
		AirborneMs:                            orientation.AirborneMS,
		LandingInertiaMs:                      orientation.LandingInertiaMS,
		AttackYawLatchPolicy:                  orientation.AttackYawLatchPolicy,
	}
}

func flattenDomainDirection(direction domainmath.Vec3) domainmath.Vec3 {
	direction.Z = 0
	return direction.Normalize()
}
