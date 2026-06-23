package ai

import (
	"math"
	"strings"

	domainmath "server-apeiron/internal/domain/math"
)

func classifyTactic(policy Policy, rangeCM float64, pressure float64) (string, string) {
	chaseRange := firstPositive(policy.ChaseRangeCM, policy.ApproachMaxDistanceCM, policy.DesiredRangeCM)
	retreatRange := firstPositive(policy.RetreatRangeCM, policy.BiteRangeCM)
	if chaseRange > 0 && rangeCM > chaseRange {
		return "approach", "acquire"
	}
	if retreatRange > 0 && rangeCM < retreatRange {
		if pressure >= firstPositive(policy.MaulPressureThreshold, 0.65) {
			return "pressure", "counter"
		}
		return "pressure", "evade"
	}
	return "circle", "reposition"
}

func movementForAction(action string, policy Policy, toTarget, right domainmath.Vec3, orbitSide string) (domainmath.Vec3, float64) {
	switch action {
	case "chase":
		return toTarget, firstPositive(policy.ChaseSpeedCMS, policy.OrbitSpeedCMS)
	case "lunge":
		return toTarget, firstPositive(policy.LungeSpeedCMS, policy.ChaseSpeedCMS, policy.OrbitSpeedCMS)
	case "maul":
		return right, firstPositive(policy.MaulSpeedCMS, policy.OrbitSpeedCMS)
	case "retreat", "dodge":
		return toTarget.Scale(-1), firstPositive(policy.RetreatSpeedCMS, policy.OrbitSpeedCMS)
	default:
		side := 1.0
		if strings.EqualFold(orbitSide, "right") {
			side = -1
		}
		return right.Scale(side), orbitSpeed(policy)
	}
}

func movementForSetup(setup SkillSetupPolicy, policy Policy, toTarget, right domainmath.Vec3, orbitSide string) (domainmath.Vec3, float64) {
	switch strings.ToLower(strings.TrimSpace(setup.MovementTactic)) {
	case "run_chase_then_jump", "chase_then_jump":
		return toTarget, firstPositive(policy.ChaseSpeedCMS, policy.LungeSpeedCMS, policy.OrbitSpeedCMS)
	case "lateral_counter_dash":
		return rightForOrbitSide(right, orbitSide), firstPositive(policy.MaulSpeedCMS, policy.OrbitSpeedCMS)
	case "circle_then_curve_to_target", "orbit_run", "flank_then_commit":
		lateral := rightForOrbitSide(right, orbitSide)
		curveToward := toTarget.Scale(0.28)
		return lateral.Add(curveToward).Normalize(), firstPositive(policy.ChaseSpeedCMS, policy.LungeSpeedCMS, policy.OrbitSpeedCMS)
	default:
		return movementForAction(actionForSkill(setup.SkillID, policy), policy, toTarget, right, orbitSide)
	}
}

func rightForOrbitSide(right domainmath.Vec3, orbitSide string) domainmath.Vec3 {
	side := 1.0
	if strings.EqualFold(orbitSide, "right") {
		side = -1
	}
	return right.Scale(side)
}

func orbitSpeed(policy Policy) float64 {
	speed := firstPositive(policy.OrbitSpeedCMS, policy.ChaseSpeedCMS)
	scale := firstPositive(policy.OrbitSpeedScale, 1)
	return speed * scale
}

func targetVectors(creature, target domainmath.Position) (domainmath.Vec3, domainmath.Vec3, float64) {
	toTarget := domainmath.Direction(creature, target)
	if toTarget.IsZero() {
		toTarget = domainmath.V3(-1, 0, 0)
	}
	right := domainmath.V3(-toTarget.Y, toTarget.X, 0).Normalize()
	rangeCM := creature.Distance(target)
	return toTarget, right, rangeCM
}

func firstPositive(values ...float64) float64 {
	for _, value := range values {
		if value > 0 && !math.IsNaN(value) {
			return value
		}
	}
	return 0
}

func firstPositiveUint64(values ...uint64) uint64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}
