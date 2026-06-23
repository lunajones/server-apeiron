package ai

import (
	"fmt"
	"strings"
)

func ValidatePolicy(policy Policy) []string {
	var issues []string
	if strings.TrimSpace(policy.ContractID) == "" {
		issues = append(issues, "contract id")
	}
	if strings.TrimSpace(policy.DodgeSkillID) == "" {
		issues = append(issues, "dodge skill id")
	}
	issues = append(issues, requirePositive("desired range", policy.DesiredRangeCM)...)
	issues = append(issues, requirePositive("chase range", policy.ChaseRangeCM)...)
	issues = append(issues, requirePositive("retreat range", policy.RetreatRangeCM)...)
	issues = append(issues, requirePositive("bite range", policy.BiteRangeCM)...)
	issues = append(issues, requirePositive("lunge min range", policy.LungeMinRangeCM)...)
	issues = append(issues, requirePositive("lunge max range", policy.LungeMaxRangeCM)...)
	issues = append(issues, requirePositive("maul pressure threshold", policy.MaulPressureThreshold)...)
	issues = append(issues, requirePositive("orbit speed", policy.OrbitSpeedCMS)...)
	issues = append(issues, requirePositive("chase speed", policy.ChaseSpeedCMS)...)
	issues = append(issues, requirePositive("lunge speed", policy.LungeSpeedCMS)...)
	issues = append(issues, requirePositive("maul speed", policy.MaulSpeedCMS)...)
	issues = append(issues, requirePositive("retreat speed", policy.RetreatSpeedCMS)...)
	issues = append(issues, requirePositive("orbit speed scale", policy.OrbitSpeedScale)...)
	issues = append(issues, requirePositive("maul counter chance", policy.MaulCounterChance)...)
	issues = append(issues, requirePositive("dodge retreat multiplier", policy.DodgeRetreatMultiplier)...)
	issues = append(issues, requirePositive("global dodge multiplier", policy.GlobalDodgeMultiplier)...)
	issues = append(issues, requirePositive("commit threat weight", policy.CommitThreatWeight)...)
	issues = append(issues, requirePositive("closing threat weight", policy.ClosingThreatWeight)...)
	issues = append(issues, requirePositive("defensive bite weight", policy.DefensiveBiteWeight)...)
	issues = append(issues, requirePositive("fleeing lunge weight", policy.FleeingLungeWeight)...)
	issues = append(issues, requirePositive("low resource risk floor", policy.LowResourceRiskFloor)...)
	issues = append(issues, requirePositive("dodge committed threat multiplier", policy.DodgeCommittedThreatMultiplier)...)
	issues = append(issues, requirePositive("vulnerable bite multiplier", policy.VulnerableBiteMultiplier)...)
	issues = append(issues, requirePositive("vulnerable maul multiplier", policy.VulnerableMaulMultiplier)...)
	issues = append(issues, requirePositive("tactical destination distance", policy.TacticalDestinationDistanceCM)...)
	issues = append(issues, requirePositive("evasion lateral bias", policy.EvasionLateralBias)...)
	issues = append(issues, requirePositive("evasion backstep bias", policy.EvasionBackstepBias)...)
	issues = append(issues, requirePositive("evasion pressure threshold", policy.EvasionPressureThreshold)...)
	if policy.LungeMinRangeCM > 0 && policy.LungeMaxRangeCM > 0 && policy.LungeMinRangeCM > policy.LungeMaxRangeCM {
		issues = append(issues, "lunge min range exceeds max range")
	}
	if policy.RetreatRangeCM > 0 && policy.ChaseRangeCM > 0 && policy.RetreatRangeCM >= policy.ChaseRangeCM {
		issues = append(issues, "retreat range must be lower than chase range")
	}
	if policy.MinOrbitDurationTicks == 0 {
		issues = append(issues, "min orbit duration ticks")
	}
	if policy.SideSwitchCooldownTicks == 0 {
		issues = append(issues, "side switch cooldown ticks")
	}
	if policy.SideFlipChanceMultiplier < 0 || policy.SideFlipChanceMultiplier > 1 {
		issues = append(issues, "side flip chance multiplier out of range")
	}
	issues = append(issues, validatePolicySetupPolicies(policy.SetupPolicies)...)
	issues = append(issues, validatePolicyBindings(policy)...)
	return issues
}

func requirePositive(name string, value float64) []string {
	if value <= 0 {
		return []string{name}
	}
	return nil
}

func validatePolicySetupPolicies(setups map[string]SkillSetupPolicy) []string {
	if len(setups) == 0 {
		return []string{"skill setup policies"}
	}
	var issues []string
	for id, setup := range setups {
		label := id
		if label == "" {
			label = setup.ID
		}
		if !setup.Enabled {
			continue
		}
		if strings.TrimSpace(setup.ID) == "" {
			issues = append(issues, "setup policy id")
		}
		if strings.TrimSpace(setup.SkillID) == "" {
			issues = append(issues, fmt.Sprintf("setup %s skill id", label))
		}
		if strings.TrimSpace(setup.MovementTactic) == "" {
			issues = append(issues, fmt.Sprintf("setup %s movement tactic", label))
		}
		if setup.MaxSetupTicks == 0 {
			issues = append(issues, fmt.Sprintf("setup %s max setup ticks", label))
		}
		if setup.MinSetupTicks > 0 && setup.MaxSetupTicks > 0 && setup.MinSetupTicks > setup.MaxSetupTicks {
			issues = append(issues, fmt.Sprintf("setup %s min setup exceeds max setup", label))
		}
		if setup.PreferredMinRangeCM > 0 && setup.PreferredMaxRangeCM > 0 && setup.PreferredMinRangeCM > setup.PreferredMaxRangeCM {
			issues = append(issues, fmt.Sprintf("setup %s preferred min exceeds max", label))
		}
		if setup.CommitDistanceCM <= 0 {
			issues = append(issues, fmt.Sprintf("setup %s commit distance", label))
		}
	}
	return issues
}

func validatePolicyBindings(policy Policy) []string {
	if len(policy.Bindings) == 0 {
		return []string{"skill behavior bindings"}
	}
	setupByID := policy.SetupPolicies
	var issues []string
	enabled := 0
	for _, binding := range policy.Bindings {
		if !binding.Enabled {
			continue
		}
		enabled++
		label := binding.ID
		if label == "" {
			label = binding.SkillID
		}
		if strings.TrimSpace(binding.ID) == "" {
			issues = append(issues, "binding id")
		}
		if strings.TrimSpace(binding.SkillID) == "" {
			issues = append(issues, fmt.Sprintf("binding %s skill id", label))
		}
		if strings.TrimSpace(binding.TacticalState) == "" {
			issues = append(issues, fmt.Sprintf("binding %s tactical state", label))
		}
		if strings.TrimSpace(binding.DecisionPhase) == "" {
			issues = append(issues, fmt.Sprintf("binding %s decision phase", label))
		}
		if binding.Priority <= 0 {
			issues = append(issues, fmt.Sprintf("binding %s priority", label))
		}
		if binding.UsageWeight <= 0 {
			issues = append(issues, fmt.Sprintf("binding %s usage weight", label))
		}
		if binding.MinRangeCM > 0 && binding.MaxRangeCM > 0 && binding.MinRangeCM > binding.MaxRangeCM {
			issues = append(issues, fmt.Sprintf("binding %s min range exceeds max range", label))
		}
		if binding.SetupPolicyID != "" {
			setup, ok := setupByID[binding.SetupPolicyID]
			if !ok || !setup.Enabled {
				issues = append(issues, fmt.Sprintf("binding %s setup policy missing", label))
			} else if setup.SkillID != "" && setup.SkillID != binding.SkillID {
				issues = append(issues, fmt.Sprintf("binding %s setup skill mismatch", label))
			}
		}
	}
	if enabled == 0 {
		issues = append(issues, "enabled skill behavior bindings")
	}
	return issues
}
