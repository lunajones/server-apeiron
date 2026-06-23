package ai

import "strings"

func selectBinding(policy Policy, memory Memory, input Input, tacticalState string, decisionPhase string, rangeCM float64) (SkillBinding, float64, string, bool) {
	var best SkillBinding
	bestScore := -1.0
	for _, binding := range policy.Bindings {
		if !binding.Enabled || binding.SkillID == "" {
			continue
		}
		if !skillAvailable(binding.SkillID, input.UnavailableSkill) {
			continue
		}
		if !skillAffordable(binding.SkillID, input.ResourceCurrent, input.SkillCosts) {
			continue
		}
		if binding.RequiresLineOfSight && !input.LineOfSight {
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
		score *= repeatSkillScoreMultiplier(policy, memory, input.Tick, binding.SkillID)
		if score > bestScore {
			best = binding
			bestScore = score
		}
	}
	if bestScore < 0 {
		return SkillBinding{}, 0, "no_binding_ready", false
	}
	return best, bestScore, "skill_behavior_binding:" + best.ID, true
}

func skillAvailable(skillID string, unavailable map[string]string) bool {
	if skillID == "" {
		return false
	}
	if unavailable == nil {
		return true
	}
	_, blocked := unavailable[skillID]
	return !blocked
}

func skillAffordable(skillID string, resourceCurrent float64, skillCosts map[string]float64) bool {
	if skillID == "" || skillCosts == nil {
		return true
	}
	cost := skillCosts[skillID]
	if cost <= 0 {
		return true
	}
	return resourceCurrent >= cost
}

func repeatSkillScoreMultiplier(policy Policy, memory Memory, tick uint64, skillID string) float64 {
	if skillID == "" || memory.LastSelectedSkill == "" || memory.LastSelectedSkill != skillID {
		return 1
	}
	window := policy.RepeatSkillPenaltyWindowTicks
	if window == 0 {
		return 1
	}
	if tick > memory.LastSelectedSkillTick && tick-memory.LastSelectedSkillTick > window {
		return 1
	}
	multiplier := policy.RepeatSkillPenaltyMultiplier
	if multiplier <= 0 || multiplier >= 1 {
		return 1
	}
	return multiplier
}

func skillCost(skillID string, skillCosts map[string]float64) float64 {
	if skillID == "" || skillCosts == nil {
		return 0
	}
	return skillCosts[skillID]
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
