package ai

import "strings"

func selectBinding(policy Policy, tacticalState string, decisionPhase string, rangeCM float64, lineOfSight bool) (SkillBinding, bool) {
	var best SkillBinding
	bestScore := -1.0
	for _, binding := range policy.Bindings {
		if !binding.Enabled || binding.SkillID == "" {
			continue
		}
		if binding.RequiresLineOfSight && !lineOfSight {
			continue
		}
		if binding.MinRangeCM > 0 && rangeCM < binding.MinRangeCM {
			continue
		}
		if binding.MaxRangeCM > 0 && rangeCM > binding.MaxRangeCM {
			continue
		}
		if !matchesState(binding.TacticalState, tacticalState) {
			continue
		}
		if !matchesState(binding.DecisionPhase, decisionPhase) {
			continue
		}
		score := float64(binding.Priority) * firstPositive(binding.UsageWeight, 1)
		if score > bestScore {
			best = binding
			bestScore = score
		}
	}
	return best, bestScore >= 0
}

func matchesState(configured string, actual string) bool {
	configured = strings.TrimSpace(strings.ToLower(configured))
	actual = strings.TrimSpace(strings.ToLower(actual))
	return configured == "" || configured == "*" || configured == actual
}

func actionForSkill(skillID string, policy Policy) string {
	switch skillID {
	case "lunge":
		return "lunge"
	case "maul":
		return "maul"
	case policy.DodgeSkillID:
		return "retreat"
	case "bite":
		return "bite"
	default:
		if skillID == "" {
			return "orbit"
		}
		return "skill"
	}
}

func publishesSkill(action string) bool {
	switch action {
	case "lunge", "maul", "retreat", "dodge", "bite", "skill":
		return true
	default:
		return false
	}
}
