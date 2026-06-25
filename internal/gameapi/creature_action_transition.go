package gameapi

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	gamev1 "server-apeiron/gen/apeiron/game/v1"
	"server-apeiron/internal/combat/actionruntime"
	"server-apeiron/internal/movement"
)

const creatureActionTransitionOwner = "creature_action_transition"

type creatureActionTransitionState struct {
	SkillID             string
	ActionContractID    string
	ActionType          string
	Kind                string
	StartPosition       vector
	StartedAt           time.Time
	EndsAt              time.Time
	LastAdvancedAt      time.Time
	Endpoint            vector
	ExitDirection       vector
	ExitSpeedCMPerSec   float64
	Sequence            uint64
	SetupPolicyID       string
	PreviousTactic      string
	ContactPolicy       string
	Contract            MovementActionRuntimeContract
	DistanceAtHandoffCM float64
	TotalDistanceCM     float64
}

func (r *Runtime) beginCreatureActionTransitionLocked(entity *entityState, motion *actionMotionState, now time.Time, endpoint vector, exitVelocity vector, distanceAtHandoffCM float64) bool {
	if entity == nil || motion == nil || entity.entityType != "creature" || !strings.EqualFold(motion.MotionSource, "skill_root") {
		return false
	}
	duration := r.creatureActionTransitionDuration(motion)
	if duration <= 0 {
		return false
	}
	if strings.EqualFold(motion.Contract.ActionType, "leap") || motion.UseVerticalRoot {
		endpoint = r.entityGroundRootPosition(entity, endpoint)
		exitVelocity.z = 0
	}
	direction := normalize(exitVelocity)
	if direction == (vector{}) {
		direction = normalize(motion.Direction)
	}
	if direction == (vector{}) {
		direction = yawVector(entity.yaw)
	}
	exitSpeed := length(exitVelocity)
	kind := creatureActionTransitionKind(motion.Contract)
	totalDistanceCM := distance(entity.position, endpoint)
	entity.creatureActionTransition = &creatureActionTransitionState{
		SkillID:             motion.SkillID,
		ActionContractID:    motion.Contract.ID,
		ActionType:          motion.Contract.ActionType,
		Kind:                kind,
		StartedAt:           now,
		EndsAt:              now.Add(duration),
		LastAdvancedAt:      now,
		StartPosition:       entity.position,
		Endpoint:            endpoint,
		ExitDirection:       direction,
		ExitSpeedCMPerSec:   exitSpeed,
		Sequence:            motion.Sequence,
		SetupPolicyID:       entity.creatureActiveSetupPolicyID,
		PreviousTactic:      entity.movementState,
		ContactPolicy:       motion.ContactPolicy,
		Contract:            motion.Contract,
		DistanceAtHandoffCM: distanceAtHandoffCM,
		TotalDistanceCM:     totalDistanceCM,
	}
	entity.actionInstance = nil
	entity.creatureActiveSetupPolicyID = ""
	entity.velocity = scale(direction, exitSpeed)
	entity.movementState = kind
	entity.skillState = "recovery"
	entity.combatState = "committed"
	r.publishCreatureActionTransitionLocomotionLocked(entity, now)
	r.publishCreatureActionTransitionSkillRuntimeLocked(entity, now)
	r.logCreatureActionTransitionLocked("begin", entity, map[string]string{
		"skill":       motion.SkillID,
		"contract":    motion.Contract.ID,
		"kind":        kind,
		"duration_ms": strconv.FormatInt(duration.Milliseconds(), 10),
		"exit_speed":  strconv.FormatFloat(exitSpeed, 'f', 1, 64),
	})
	return true
}

func (r *Runtime) refreshCreatureActionTransitionLocked(entity *entityState, now time.Time) bool {
	if entity == nil || entity.creatureActionTransition == nil {
		return false
	}
	transition := entity.creatureActionTransition
	if !now.Before(transition.EndsAt) {
		r.completeCreatureActionTransitionLocked(entity, now)
		return false
	}
	if !transition.LastAdvancedAt.IsZero() && !now.After(transition.LastAdvancedAt) {
		r.publishCreatureActionTransitionLocomotionLocked(entity, now)
		return true
	}
	elapsed := now.Sub(transition.StartedAt)
	total := transition.EndsAt.Sub(transition.StartedAt)
	phaseT := 1.0
	if total > 0 {
		phaseT = elapsed.Seconds() / total.Seconds()
	}
	if phaseT < 0 {
		phaseT = 0
	}
	if phaseT > 1 {
		phaseT = 1
	}
	speed := transition.ExitSpeedCMPerSec * (1 - phaseT)
	if speed < 0 {
		speed = 0
	}
	motion := movement.ResolveConstantStep(movement.ConstantStepInput{
		Position:         toDomainVector(entity.position),
		Direction:        toDomainVector(transition.ExitDirection),
		SpeedCMPerSecond: speed,
		TickRate:         tickRate,
	})
	projected := fromDomainVector(motion.Projected)
	projected.z = transition.Endpoint.z
	distanceToEndpointCM := distance(entity.position, transition.Endpoint)
	if distanceToEndpointCM <= 0 {
		entity.position = r.entityGroundRootPosition(entity, transition.Endpoint)
		entity.velocity = vector{}
	} else if stepDistanceCM := distance(entity.position, projected); stepDistanceCM >= distanceToEndpointCM {
		entity.position = r.entityGroundRootPosition(entity, transition.Endpoint)
		entity.velocity = vector{}
	} else {
		entity.position = projected
		entity.velocity = fromDomainVector(motion.Velocity)
	}
	entity.velocity.z = 0
	if transition.ExitDirection != (vector{}) {
		entity.yaw = vectorYaw(transition.ExitDirection)
	}
	entity.movementState = transition.Kind
	entity.skillState = "recovery"
	entity.combatState = "committed"
	transition.LastAdvancedAt = now
	r.publishCreatureActionTransitionLocomotionLocked(entity, now)
	r.publishCreatureActionTransitionSkillRuntimeLocked(entity, now)
	return true
}

func (r *Runtime) completeCreatureActionTransitionLocked(entity *entityState, now time.Time) {
	if entity == nil || entity.creatureActionTransition == nil {
		return
	}
	transition := entity.creatureActionTransition
	entity.creatureActionTransition = nil
	entity.position = r.entityGroundRootPosition(entity, transition.Endpoint)
	entity.velocity = vector{}
	entity.movementState = "grounded"
	entity.skillState = "idle"
	entity.combatState = "ready"
	entity.actionInstance = nil
	entity.actionMotion = nil
	entity.actionOrientationLatch = nil
	entity.creatureActiveSetupPolicyID = ""
	r.publishEntityTerminalSkillRuntimeLocked(entity, skillRuntimeStateIdle, now)
	if entity.locomotion != nil && (entity.locomotion.GetAction() == transition.SkillID || entity.locomotion.GetAction() == transition.Kind) {
		entity.locomotion.Phase = "complete"
		entity.locomotion.MovementMode = "grounded"
		entity.locomotion.LandingHandoffActive = false
		entity.locomotion.LandingExitDirection = nil
		entity.locomotion.LandingExitSpeed = 0
		entity.locomotion.TargetSpeed = 0
		entity.locomotion.EffectiveSpeed = 0
		entity.locomotion.ActionProjectedPosition = toProto(entity.position)
		entity.locomotion.LastUpdatedTick = r.tick
	}
	r.logCreatureActionTransitionLocked("complete", entity, map[string]string{
		"skill":    transition.SkillID,
		"contract": transition.ActionContractID,
		"kind":     transition.Kind,
	})
}

func (r *Runtime) publishCreatureActionTransitionLocomotionLocked(entity *entityState, now time.Time) {
	if entity == nil || entity.creatureActionTransition == nil {
		return
	}
	transition := entity.creatureActionTransition
	remaining := transition.EndsAt.Sub(now)
	if remaining < 0 {
		remaining = 0
	}
	elapsed := now.Sub(transition.StartedAt)
	if elapsed < 0 {
		elapsed = 0
	}
	phase := "exit_handoff"
	if strings.EqualFold(transition.ActionType, "leap") {
		phase = "landing_handoff"
	}
	distanceTraveled := distance(transition.StartPosition, entity.position)
	if transition.EndsAt.Before(now) {
		distanceTraveled = distance(transition.StartPosition, transition.Endpoint)
	}
	loco := locomotionFromContractWithOverrides(transition.Contract, phase, transition.Endpoint, entity.position, r.tick, transition.Sequence, length(entity.velocity), distanceTraveled)
	loco.Action = transition.SkillID
	loco.AbilityKey = transition.SkillID
	loco.MovementMode = "grounded_handoff"
	loco.Phase = phase
	loco.PhaseElapsedMs = int32(elapsed.Milliseconds())
	loco.PhaseRemainingMs = int32(remaining.Milliseconds())
	loco.LandingHandoffActive = true
	loco.LandingExitDirection = toProto(transition.ExitDirection)
	loco.LandingExitSpeed = transition.ExitSpeedCMPerSec
	loco.ActionStartPosition = toProto(transition.StartPosition)
	loco.ActionProjectedPosition = toProto(entity.position)
	loco.ActionDistanceTraveled = transition.DistanceAtHandoffCM + distanceTraveled
	loco.TargetSpeed = length(entity.velocity)
	loco.EffectiveSpeed = length(entity.velocity)
	loco.LastUpdatedTick = r.tick
	entity.locomotion = loco
}

func (r *Runtime) publishCreatureActionTransitionSkillRuntimeLocked(entity *entityState, now time.Time) {
	if entity == nil || entity.creatureActionTransition == nil {
		return
	}
	transition := entity.creatureActionTransition
	if entity.skillRuntime == nil {
		entity.skillRuntime = &gamev1.SkillRuntimeState{}
	}
	entity.skillRuntime.CurrentSkillId = transition.SkillID
	entity.skillRuntime.State = string(actionruntime.PhaseRecovery)
	entity.skillRuntime.LastResolvedAtMs = now.UnixMilli()
	if entity.skillRuntime.StartedAtMs <= 0 && !transition.StartedAt.IsZero() {
		entity.skillRuntime.StartedAtMs = transition.StartedAt.UnixMilli()
	}
}

func (r *Runtime) creatureActionTransitionDuration(motion *actionMotionState) time.Duration {
	if motion == nil {
		return 0
	}
	switch strings.ToLower(strings.TrimSpace(motion.Contract.ActionType)) {
	case "dodge":
		if motion.Contract.RecoveryMS > 0 {
			return durationFromMS(motion.Contract.RecoveryMS)
		}
		if r.contracts.MovementProfile != nil && r.contracts.MovementProfile.GetDodgeCarryHandoffMs() > 0 {
			return durationFromMS(r.contracts.MovementProfile.GetDodgeCarryHandoffMs())
		}
	case "leap":
		if motion.Contract.RecoveryMS > 0 {
			return durationFromMS(motion.Contract.RecoveryMS)
		}
		if r.contracts.MovementProfile != nil && r.contracts.MovementProfile.GetLeapGroundedCarryHandoffMs() > 0 {
			return durationFromMS(r.contracts.MovementProfile.GetLeapGroundedCarryHandoffMs())
		}
	default:
		if motion.Contract.RecoveryMS > 0 {
			return durationFromMS(motion.Contract.RecoveryMS)
		}
	}
	return 0
}

func creatureActionTransitionKind(contract MovementActionRuntimeContract) string {
	switch strings.ToLower(strings.TrimSpace(contract.ActionType)) {
	case "dodge":
		return "creature_dodge_exit_transition"
	case "leap":
		return "creature_leap_landing_transition"
	default:
		return "creature_skill_exit_transition"
	}
}

func creatureActionTransitionActive(entity *entityState) bool {
	return entity != nil && entity.creatureActionTransition != nil
}

func (r *Runtime) logCreatureActionTransitionLocked(event string, entity *entityState, fields map[string]string) {
	if !creatureActionTransitionDebugEnabled() || entity == nil {
		return
	}
	parts := []string{
		"ApeironCreatureActionTransition",
		"event=" + event,
		"entity=" + strconv.FormatUint(entity.id, 10),
		"pos=" + fmt.Sprintf("(%.1f,%.1f,%.1f)", entity.position.x, entity.position.y, entity.position.z),
	}
	if entity.creatureActionTransition != nil {
		parts = append(parts,
			"owner="+creatureActionTransitionOwner,
			"skill="+entity.creatureActionTransition.SkillID,
			"kind="+entity.creatureActionTransition.Kind,
		)
	}
	for key, value := range fields {
		parts = append(parts, key+"="+value)
	}
	fmt.Println(strings.Join(parts, " "))
}

func creatureActionTransitionDebugEnabled() bool {
	value := strings.TrimSpace(os.Getenv("APEIRON_CREATURE_ACTION_TRANSITION_DEBUG"))
	return strings.EqualFold(value, "1") || strings.EqualFold(value, "true") || strings.EqualFold(value, "yes")
}
