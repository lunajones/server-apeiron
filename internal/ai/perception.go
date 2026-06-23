package ai

import (
	"math"
	"strings"

	domainmath "server-apeiron/internal/domain/math"
)

const (
	defaultCommitThreatWeight   = 0.28
	defaultClosingThreatWeight  = 0.18
	defaultDefensiveBiteWeight  = 0.14
	defaultFleeingLungeWeight   = 0.20
	defaultLowResourceRiskFloor = 0.16
)

type Perception struct {
	TargetVelocityCMPerSec domainmath.Vec3
	TargetMovementState    string
	TargetCombatState      string
	TargetSkillState       string
	TargetActionActive     bool
	TargetBlocking         bool
	TargetParrying         bool
	TargetIFrame           bool
	TargetResourceCurrent  float64
	TargetResourceMax      float64
	TargetPostureCurrent   float64
	TargetPostureMax       float64
}

type ThreatAssessment struct {
	Pressure             float64
	TargetClosing        bool
	TargetFleeing        bool
	TargetDefensive      bool
	TargetCommitted      bool
	TargetVulnerable     bool
	TargetHasIFrame      bool
	PreferredCounter     string
	SkillScoreMultiplier map[string]float64
	Reason               string
}

func AssessThreat(policy Policy, input Input, toTarget domainmath.Vec3, rangeCM float64) ThreatAssessment {
	perception := input.Perception
	pressure := clamp01(input.Pressure)
	if pressure == 0 && policy.MaulPressureThreshold > 0 {
		pressure = clamp01(policy.MaulPressureThreshold * 0.5)
	}

	targetSpeed := perception.TargetVelocityCMPerSec.Length()
	targetDir := perception.TargetVelocityCMPerSec.Normalize()
	closing := false
	fleeing := false
	if targetSpeed > 0 && !toTarget.IsZero() {
		closingDot := targetDir.Dot(toTarget.Scale(-1))
		closing = closingDot > 0.35
		fleeing = closingDot < -0.35
	}

	committed := perception.TargetActionActive || actionStateIsCommitted(perception.TargetSkillState) || actionStateIsCommitted(perception.TargetCombatState)
	defensive := perception.TargetBlocking || perception.TargetParrying || actionStateIsDefensive(perception.TargetCombatState)
	if committed {
		pressure += defaultCommitThreatWeight
	}
	if closing {
		pressure += defaultClosingThreatWeight
	}
	if defensive {
		pressure += defaultDefensiveBiteWeight
	}
	if targetResourceLow(perception.TargetResourceCurrent, perception.TargetResourceMax) {
		pressure += defaultLowResourceRiskFloor
	}

	vulnerable := !perception.TargetIFrame && !perception.TargetParrying && targetPostureLow(perception.TargetPostureCurrent, perception.TargetPostureMax)
	multipliers := map[string]float64{}
	if policy.MaulCounterUnderPressure && committed && !perception.TargetIFrame {
		multipliers["maul"] = 1 + firstPositive(policy.MaulCounterChance, 0.22)
	}
	if policy.DodgeUnderPressure && committed {
		multipliers[policy.DodgeSkillID] = firstPositive(policy.GlobalDodgeMultiplier, 1) * 1.12
	}
	if defensive && !perception.TargetIFrame {
		multipliers["bite"] = 1.0 + defaultDefensiveBiteWeight
	}
	if fleeing && rangeCM >= firstPositive(policy.LungeMinRangeCM, policy.DesiredRangeCM) {
		multipliers["lunge"] = 1.0 + defaultFleeingLungeWeight
	}
	if vulnerable {
		multipliers["bite"] = math.Max(firstPositive(multipliers["bite"], 1), 1.16)
		multipliers["maul"] = math.Max(firstPositive(multipliers["maul"], 1), 1.10)
	}

	counter := ""
	if policy.MaulCounterUnderPressure && pressure >= firstPositive(policy.MaulPressureThreshold, 0.65) && committed && !perception.TargetIFrame {
		counter = "maul"
	} else if policy.DodgeUnderPressure && pressure >= firstPositive(policy.MaulPressureThreshold, 0.65) && committed {
		counter = policy.DodgeSkillID
	}

	reasonParts := make([]string, 0, 5)
	if committed {
		reasonParts = append(reasonParts, "target_committed")
	}
	if closing {
		reasonParts = append(reasonParts, "target_closing")
	}
	if fleeing {
		reasonParts = append(reasonParts, "target_fleeing")
	}
	if defensive {
		reasonParts = append(reasonParts, "target_defensive")
	}
	if vulnerable {
		reasonParts = append(reasonParts, "target_vulnerable")
	}
	reason := strings.Join(reasonParts, "+")
	if reason == "" {
		reason = "baseline"
	}

	return ThreatAssessment{
		Pressure:             clamp01(pressure),
		TargetClosing:        closing,
		TargetFleeing:        fleeing,
		TargetDefensive:      defensive,
		TargetCommitted:      committed,
		TargetVulnerable:     vulnerable,
		TargetHasIFrame:      perception.TargetIFrame,
		PreferredCounter:     counter,
		SkillScoreMultiplier: multipliers,
		Reason:               reason,
	}
}

func skillThreatMultiplier(threat ThreatAssessment, skillID string) float64 {
	if skillID == "" || threat.SkillScoreMultiplier == nil {
		return 1
	}
	return firstPositive(threat.SkillScoreMultiplier[skillID], 1)
}

func actionStateIsCommitted(state string) bool {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "active", "committed", "cast", "casting", "windup", "recovery":
		return true
	default:
		return false
	}
}

func actionStateIsDefensive(state string) bool {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "blocking", "block", "guard", "parry", "parry_active", "perfect_block":
		return true
	default:
		return false
	}
}

func targetResourceLow(current, max float64) bool {
	return max > 0 && current >= 0 && current/max <= 0.25
}

func targetPostureLow(current, max float64) bool {
	return max > 0 && current >= 0 && current/max <= 0.35
}

func clamp01(value float64) float64 {
	if math.IsNaN(value) || value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
