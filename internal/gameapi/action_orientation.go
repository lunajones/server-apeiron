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
	AttackYawLatched     bool
}

// creatureActionOrientationLatch is the per-action persistent attack-direction lock.
// Once an action commits (at the policy-defined latch point), attack_yaw stops tracking
// the moving target and freezes to the committed line. The hitbox, airborne root and
// presentation all read this frozen value, so the strike follows where the actor actually
// lunged instead of re-aiming at the target every tick (roadmap orientation rules 3-5).
// It is keyed by the owning action InstanceID so a new action resets it cleanly.
type creatureActionOrientationLatch struct {
	InstanceID   string
	SkillID      string
	LatchPolicy  string
	AttackYawDeg float64
	Latched      bool
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
	// Focus (head/attention) and pre-latch attack yaw ease toward their contract-defined
	// source at their own turn rates instead of snapping, so the head leads and the strike
	// winds up. Sources are data-driven so the same code serves player aim policies later.
	hasTarget := creature != nil && target != nil
	focusDesired, attackDesired := targetYaw, targetYaw
	if contract.Orientation != nil {
		focusDesired = orientationDesiredSourceYaw(contract.Orientation.GetFocusYawSource(), targetYaw, state.MovementYawDeg, targetYaw, hasTarget)
		attackDesired = orientationDesiredSourceYaw(contract.Orientation.GetAttackYawSource(), targetYaw, state.MovementYawDeg, targetYaw, hasTarget)
	}
	focusPrev, focusKnown, attackPrev, attackKnown := 0.0, false, 0.0, false
	if creature != nil {
		focusPrev, focusKnown = creature.orientationFocusYaw, creature.orientationFocusYawKnown
		attackPrev, attackKnown = creature.orientationAttackYaw, creature.orientationAttackYawKnown
	}
	state.FocusYawDeg = approachPersistedOrientationYaw(focusKnown, focusPrev, focusDesired, orientationFocusTurnRate(contract, decision.TurnRateDegPerSec))
	state.AttackYawDeg = approachPersistedOrientationYaw(attackKnown, attackPrev, attackDesired, orientationAttackTurnRate(contract, decision.TurnRateDegPerSec))
	state.Phase = "tactical_setup"
	if creatureActionTransitionActive(creature) {
		state.Phase = "landing_inertia"
		if creature.creatureActionTransition != nil {
			if creature.creatureActionTransition.ExitDirection != (vector{}) {
				state.MovementYawDeg = vectorYaw(creature.creatureActionTransition.ExitDirection)
			}
			state.BodyYawDeg = approachCreatureYaw(entityYaw(creature), state.MovementYawDeg, orientationBodyTurnRate(contract, decision.TurnRateDegPerSec))
			// Landing inertia preserves the latched attack line so the strike direction
			// stays stable through the inertia tail instead of snapping to the exit move.
			if latch := creature.actionOrientationLatch; latch != nil && latch.Latched {
				state.AttackYawDeg = normalizeYaw(latch.AttackYawDeg)
				state.AttackYawLatched = true
			} else {
				state.AttackYawDeg = state.MovementYawDeg
			}
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
	// Once the action commits, attack yaw freezes to the latched committed line while
	// focus_yaw keeps tracking the target above — the two are now genuinely separate.
	if creature != nil && creature.actionInstance != nil {
		if latch := creature.actionOrientationLatch; latch != nil && latch.Latched {
			state.AttackYawDeg = normalizeYaw(latch.AttackYawDeg)
			state.AttackYawLatched = true
		}
	}
	bodyTarget := state.MovementYawDeg
	bodySource := ""
	if contract.Orientation != nil {
		bodySource = strings.ToLower(strings.TrimSpace(contract.Orientation.GetBodyYawSource()))
	}
	switch bodySource {
	case "aim_direction", "target", "attack_yaw", "aim":
		// Committed-facing actions (player shield rush/bash, heavy) keep the body on the
		// attack line the whole time.
		bodyTarget = state.AttackYawDeg
	case "movement_direction", "movement", "velocity":
		bodyTarget = state.MovementYawDeg
	default:
		// movement_direction_until_commit / none / unset: follow movement during setup,
		// snap the body onto the attack line once the action commits (pre_commit/airborne),
		// or whenever side-on movement is disallowed.
		if !orientationAllowsSideOn(contract) || state.Phase == "pre_commit" || state.Phase == "airborne" {
			bodyTarget = state.AttackYawDeg
		}
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
	if !math.IsNaN(orientation.FocusYawDeg) {
		creature.orientationFocusYaw = normalizeYaw(orientation.FocusYawDeg)
		creature.orientationFocusYawKnown = true
	}
	if !math.IsNaN(orientation.AttackYawDeg) {
		creature.orientationAttackYaw = normalizeYaw(orientation.AttackYawDeg)
		creature.orientationAttackYawKnown = true
	}
}

// approachPersistedOrientationYaw eases a persisted yaw toward target at turnRate. On first
// observation (not yet known) it snaps, so a freshly seen actor faces correctly without an
// artificial opening sweep; turnRate <= 0 also snaps.
func approachPersistedOrientationYaw(known bool, current, target, turnRate float64) float64 {
	if !known || turnRate <= 0 {
		return normalizeYaw(target)
	}
	return approachCreatureYaw(current, target, turnRate)
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

// orientationDesiredSourceYaw maps an action_orientation_policy *_yaw_source to a concrete
// desired yaw. Creatures have no camera/aim, so aim/camera/target/commit-snapshot sources all
// resolve to the target bearing; movement sources resolve to the movement direction. "none",
// empty, or unknown returns the fallback unchanged so the yaw holds its current intent.
// (For players this is where aim_direction would resolve to the camera/aim yaw instead.)
func orientationDesiredSourceYaw(source string, targetYaw, movementYaw, fallback float64, hasTarget bool) float64 {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "target", "aim_direction", "focus_or_camera", "commit_target_snapshot", "commit_aim_snapshot", "aim", "camera":
		if hasTarget {
			return targetYaw
		}
		return fallback
	case "movement_direction", "movement_direction_until_commit", "movement", "velocity":
		return movementYaw
	default:
		return fallback
	}
}

func orientationFocusTurnRate(contract SkillRuntimeContract, fallback float64) float64 {
	if contract.Orientation != nil && contract.Orientation.GetFocusTurnRateDegS() > 0 {
		return contract.Orientation.GetFocusTurnRateDegS()
	}
	if fallback > 0 {
		return fallback
	}
	return 720
}

func orientationAttackTurnRate(contract SkillRuntimeContract, fallback float64) float64 {
	if contract.Orientation != nil && contract.Orientation.GetAttackTurnRateDegS() > 0 {
		return contract.Orientation.GetAttackTurnRateDegS()
	}
	if fallback > 0 {
		return fallback
	}
	return 540
}

func orientationPolicyHasLungeCommit(contract SkillRuntimeContract) bool {
	if contract.Orientation == nil {
		return false
	}
	id := strings.ToLower(strings.TrimSpace(contract.Orientation.GetId()))
	return strings.Contains(id, "lunge") && strings.Contains(id, "commit")
}

// updateCreatureActionOrientationLatchLocked maintains the per-action attack-yaw latch.
// It runs each tick of an owned creature action, before the hitbox schedule is enqueued,
// so the committed attack direction is available to both damage resolution and presentation.
func (r *Runtime) updateCreatureActionOrientationLatchLocked(creature *entityState, target *entityState, contract SkillRuntimeContract, instance *actionruntime.Instance, now time.Time) {
	if creature == nil || instance == nil {
		return
	}
	// Only actions carrying an orientation policy participate; others keep target tracking.
	if contract.Orientation == nil {
		creature.actionOrientationLatch = nil
		return
	}
	latchPolicy := strings.ToLower(strings.TrimSpace(contract.Orientation.GetAttackYawLatchPolicy()))
	// A new action instance owns a fresh latch (resets any prior committed direction).
	if creature.actionOrientationLatch == nil || creature.actionOrientationLatch.InstanceID != instance.InstanceID {
		creature.actionOrientationLatch = &creatureActionOrientationLatch{
			InstanceID:  instance.InstanceID,
			SkillID:     instance.SkillID.String(),
			LatchPolicy: latchPolicy,
		}
	}
	latch := creature.actionOrientationLatch
	if latch.Latched {
		return
	}
	if latchPolicy == "" || latchPolicy == "none" {
		return
	}
	if !creatureActionAttackYawLatchReached(contract, instance, now) {
		return
	}
	latch.AttackYawDeg = creatureActionCommittedAttackYaw(creature, target)
	latch.Latched = true
}

// creatureActionAttackYawLatchReached reports whether the action has reached its policy
// latch point. latch_at_takeoff fires at airborne start (after the pre-commit alignment
// window); latch_at_active_start fires once the action leaves windup.
func creatureActionAttackYawLatchReached(contract SkillRuntimeContract, instance *actionruntime.Instance, now time.Time) bool {
	if instance == nil || contract.Orientation == nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(contract.Orientation.GetAttackYawLatchPolicy())) {
	case "latch_at_takeoff":
		envelope := creatureActionMovementEnvelopeAt(*instance, contract, now)
		if envelope.AirborneStartsAt.IsZero() {
			return !now.Before(envelope.MovementStartsAt)
		}
		return !now.Before(envelope.AirborneStartsAt)
	case "latch_at_active_start":
		phase := instance.PhaseAt(now)
		return phase != actionruntime.PhaseAccepted && phase != actionruntime.PhaseWindup
	default:
		return false
	}
}

// creatureActionCommittedAttackYaw captures the committed attack direction. It prefers the
// physical owned-root direction so the hitbox sweep matches actual travel; otherwise it
// snapshots the current target bearing at the latch point.
func creatureActionCommittedAttackYaw(creature *entityState, target *entityState) float64 {
	if creature != nil && creature.actionMotion != nil && creature.actionMotion.Direction != (vector{}) {
		return normalizeYaw(vectorYaw(creature.actionMotion.Direction))
	}
	if creature != nil && target != nil {
		toTarget := normalize(vector{x: target.position.x - creature.position.x, y: target.position.y - creature.position.y})
		if toTarget != (vector{}) {
			return normalizeYaw(vectorYaw(toTarget))
		}
	}
	return entityYaw(creature)
}
