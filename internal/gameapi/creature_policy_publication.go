package gameapi

import (
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

func resolveGroundedCreatureDecisionMotion(creature *entityState, decision creatureai.Decision) creatureDecisionMotion {
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
	velocity := fromDomainVector(motion.Velocity)
	velocity.z = 0
	motion.Projected = toDomainVector(projected)
	motion.Velocity = toDomainVector(velocity)
	motion.Direction = flattenDomainDirection(motion.Direction)
	return creatureDecisionMotion{Start: start, Motion: motion}
}

func applyCreatureDecisionMotion(creature *entityState, target *entityState, decision creatureai.Decision, resolved creatureDecisionMotion) {
	if creature == nil {
		return
	}
	creature.position = fromDomainVector(resolved.Motion.Projected)
	creature.velocity = fromDomainVector(resolved.Motion.Velocity)
	if target != nil {
		creature.yaw = vectorYaw(normalize(vector{x: target.position.x - creature.position.x, y: target.position.y - creature.position.y}))
	}
	creature.movementState = decision.Action
	if creatureai.PublishesSkill(decision.Action) {
		creature.skillState = decision.Action
	} else {
		creature.skillState = "idle"
	}
}

func (r *Runtime) publishWolfLocomotionLocked(wolf *entityState, decision creatureai.Decision, contract SkillRuntimeContract, resolved creatureDecisionMotion, now time.Time) {
	if wolf == nil {
		return
	}
	locoContract := r.contracts.contractForAbility(decision.SelectedSkill)
	if creatureai.PublishesSkill(decision.Action) && contract.MovementAction.ID != "" {
		locoContract = contract.MovementAction
	}
	wolf.locomotion = locomotionFromContractWithOverrides(locoContract, "active", resolved.Start, wolf.position, r.tick, 0, resolved.Motion.SpeedCMPerSecond, resolved.Motion.DistanceCM)
	wolf.locomotion.MovementMode = "grounded"
	wolf.locomotion.Action = decision.Action
	wolf.locomotion.AbilityKey = decision.SelectedSkill
	if creatureai.PublishesSkill(decision.Action) {
		wolf.locomotion.Phase = creatureActionLocomotionPhase(wolf, now)
		applyActionInstanceLocomotionTiming(wolf.locomotion, wolf.actionInstance, now)
	}
}

func (r *Runtime) publishWolfAIStateLocked(wolf *entityState, decision creatureai.Decision, policy WolfRuntimePolicy, contract SkillRuntimeContract, rangeCM float64, lungeMinRangeCM float64, lungeMaxRangeCM float64) {
	if wolf == nil {
		return
	}
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
		SkillMovementTakeoffMs:                140,
		SkillMovementLandingLockMs:            120,
		SkillWindupMs:                         contract.WindupMS,
		SkillActiveStartMs:                    contract.WindupMS,
		SkillActiveEndMs:                      contract.WindupMS + contract.ActiveMS,
		SkillRecoveryMs:                       contract.RecoveryMS,
		SkillActionLockMs:                     contract.WindupMS + contract.ActiveMS + contract.RecoveryMS,
		SkillMovementType:                     contract.MovementAction.ActionType,
		SkillMovementStartMs:                  contract.WindupMS,
		SkillMovementDurationMs:               contract.MovementAction.DurationMS,
		SkillMovementDistanceCm:               contract.MovementAction.DistanceCM,
		SkillMovementDesiredLandingDistanceCm: lungeMaxRangeCM,
		SkillMovementMinLandingDistanceCm:     lungeMinRangeCM,
		SkillMovementStopAtContactRatio:       1,
	}
}

func flattenDomainDirection(direction domainmath.Vec3) domainmath.Vec3 {
	direction.Z = 0
	return direction.Normalize()
}
