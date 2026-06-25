package gameapi

import (
	"math"
	"strings"
	"time"

	creatureai "server-apeiron/internal/ai"
	"server-apeiron/internal/combat/actionruntime"
)

type actionOrientationRuntimeState struct {
	Phase                string
	OrientationPolicyID  string
	EnvelopePolicyID     string
	BodyYawDeg           float64
	FocusYawDeg          float64
	AttackYawDeg         float64
	MovementYawDeg       float64
	PreCommitMS          int32
	AirborneMS           int32
	LandingInertiaMS     int32
	AttackYawLatchPolicy string
}

func resolveCreatureActionOrientation(creature *entityState, target *entityState, decision creatureai.Decision, contract SkillRuntimeContract, envelope creatureActionMovementEnvelope, now time.Time) actionOrientationRuntimeState {
	state := actionOrientationRuntimeState{
		BodyYawDeg:     entityYaw(creature),
		FocusYawDeg:    entityYaw(creature),
		AttackYawDeg:   entityYaw(creature),
		MovementYawDeg: entityYaw(creature),
	}
	if contract.Orientation != nil {
		state.OrientationPolicyID = contract.Orientation.GetId()
		state.AttackYawLatchPolicy = contract.Orientation.GetAttackYawLatchPolicy()
	}
	if contract.Envelope != nil {
		state.EnvelopePolicyID = contract.Envelope.GetId()
		state.PreCommitMS = contract.Envelope.GetPreCommitMs()
		state.AirborneMS = contract.Envelope.GetAirborneMs()
		state.LandingInertiaMS = contract.Envelope.GetLandingInertiaMs()
	}
	movementDir := fromDomainVector(flattenDomainDirection(decision.Direction))
	if movementDir == (vector{}) && creature != nil {
		movementDir = normalize(creature.velocity)
	}
	if movementDir == (vector{}) && creature != nil && target != nil {
		movementDir = normalize(vector{x: target.position.x - creature.position.x, y: target.position.y - creature.position.y})
	}
	if movementDir == (vector{}) {
		movementDir = yawVector(state.BodyYawDeg)
	}
	state.MovementYawDeg = vectorYaw(movementDir)
	targetYaw := state.BodyYawDeg
	if creature != nil && target != nil {
		toTarget := normalize(vector{x: target.position.x - creature.position.x, y: target.position.y - creature.position.y})
		if toTarget != (vector{}) {
			targetYaw = vectorYaw(toTarget)
		}
	}
	state.FocusYawDeg = targetYaw
	state.AttackYawDeg = targetYaw
	state.Phase = "tactical_setup"
	if creatureActionTransitionActive(creature) {
		state.Phase = "landing_inertia"
		if creature.creatureActionTransition != nil {
			if creature.creatureActionTransition.ExitDirection != (vector{}) {
				state.MovementYawDeg = vectorYaw(creature.creatureActionTransition.ExitDirection)
			}
			state.BodyYawDeg = approachCreatureYaw(entityYaw(creature), state.MovementYawDeg, orientationBodyTurnRate(contract, decision.TurnRateDegPerSec))
			state.AttackYawDeg = state.MovementYawDeg
		}
		return state
	}
	if creature != nil && creature.actionInstance != nil && creatureai.PublishesSkill(decision.Action) {
		switch {
		case envelope.PreCommitActive:
			state.Phase = "pre_commit"
		case envelope.AirborneActive:
			state.Phase = "airborne"
		case envelope.LandingInertiaActive:
			state.Phase = "landing_inertia"
		default:
			phase := creature.actionInstance.PhaseAt(now)
			if phase == actionruntime.PhaseWindup {
				state.Phase = "tactical_setup"
			} else if phase == actionruntime.PhaseRecovery {
				state.Phase = "reentry"
			}
		}
	}
	bodyTarget := state.MovementYawDeg
	if !orientationAllowsSideOn(contract) || state.Phase == "pre_commit" || state.Phase == "airborne" {
		bodyTarget = state.AttackYawDeg
	}
	state.BodyYawDeg = approachCreatureYaw(entityYaw(creature), bodyTarget, orientationBodyTurnRate(contract, decision.TurnRateDegPerSec))
	return state
}

func applyCreatureOrientationState(creature *entityState, orientation actionOrientationRuntimeState) {
	if creature == nil {
		return
	}
	if !math.IsNaN(orientation.BodyYawDeg) {
		creature.yaw = normalizeYaw(orientation.BodyYawDeg)
	}
}

func entityYaw(entity *entityState) float64 {
	if entity == nil {
		return 0
	}
	return normalizeYaw(entity.yaw)
}

func orientationAllowsSideOn(contract SkillRuntimeContract) bool {
	return contract.Orientation != nil && contract.Orientation.GetAllowBodySideOnMovement()
}

func orientationBodyTurnRate(contract SkillRuntimeContract, fallback float64) float64 {
	if contract.Orientation != nil && contract.Orientation.GetBodyTurnRateDegS() > 0 {
		return contract.Orientation.GetBodyTurnRateDegS()
	}
	if fallback > 0 {
		return fallback
	}
	return 360
}

func orientationPolicyHasLungeCommit(contract SkillRuntimeContract) bool {
	if contract.Orientation == nil {
		return false
	}
	id := strings.ToLower(strings.TrimSpace(contract.Orientation.GetId()))
	return strings.Contains(id, "lunge") && strings.Contains(id, "commit")
}
