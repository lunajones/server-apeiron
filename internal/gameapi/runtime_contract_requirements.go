package gameapi

import "fmt"

const (
	runtimeRequirementBaseMovementAction = "base_movement_action"
	runtimeRequirementSkill              = "skill"
	runtimeRequirementCombatCoreProfile  = "combat_core_profile"
	runtimeRequirementDefenseContract    = "defense_contract"
	runtimeRequirementWeaponKit          = "weapon_kit"
	runtimeRequirementWolfBrainPolicy    = "wolf_brain_policy"
	runtimeRequirementMovementProfile    = "movement_profile"
)

type RuntimeContractRequirement struct {
	Category    string
	Key         string
	ContractID  string
	Description string
}

func runtimeContractRequirements() []RuntimeContractRequirement {
	return []RuntimeContractRequirement{
		{
			Category:    runtimeRequirementMovementProfile,
			Key:         "runtime_movement",
			ContractID:  runtimeMovementReconciliationProfileID,
			Description: "rich movement/reconciliation profile consumed by server and Unreal",
		},
		{
			Category:    runtimeRequirementBaseMovementAction,
			Key:         "move",
			ContractID:  "grounded_move_v1",
			Description: "normal grounded locomotion",
		},
		{
			Category:    runtimeRequirementBaseMovementAction,
			Key:         "turn",
			ContractID:  "turn_v1_rate_limited_contextual",
			Description: "camera/control yaw action",
		},
		{
			Category:    runtimeRequirementBaseMovementAction,
			Key:         "dodge",
			ContractID:  "dodge_v1_full_iframe",
			Description: "protected full-iframe dodge baseline",
		},
		{
			Category:    runtimeRequirementBaseMovementAction,
			Key:         "jump",
			ContractID:  "jump_v1_authoritative_grounded_handoff",
			Description: "protected leap/jump baseline",
		},
		{Category: runtimeRequirementSkill, Key: "player_basic_attack_1", Description: "Bulwark M1 combo stage 1"},
		{Category: runtimeRequirementSkill, Key: "player_basic_attack_2", Description: "Bulwark M1 combo stage 2"},
		{Category: runtimeRequirementSkill, Key: "player_basic_attack_3", Description: "Bulwark M1 combo stage 3 shield drive"},
		{Category: runtimeRequirementSkill, Key: "player_shield_bash", Description: "Bulwark R Shield Bash"},
		{Category: runtimeRequirementSkill, Key: "player_shield_rush", Description: "Bulwark F Shield Rush"},
		{Category: runtimeRequirementSkill, Key: "bite", Description: "wolf close-range pressure bite"},
		{Category: runtimeRequirementSkill, Key: "lunge", Description: "wolf airborne passthrough lunge"},
		{Category: runtimeRequirementSkill, Key: "wolf_dodge", Description: "wolf evasion skill"},
		{Category: runtimeRequirementSkill, Key: "maul", Description: "wolf pressure counter maul"},
		{
			Category:    runtimeRequirementCombatCoreProfile,
			Key:         "player",
			ContractID:  playerCombatCoreProfileID,
			Description: "player sword-and-shield combat core",
		},
		{
			Category:    runtimeRequirementCombatCoreProfile,
			Key:         "creature",
			ContractID:  creatureCombatCoreProfileID,
			Description: "steppe wolf combat core",
		},
		{
			Category:    runtimeRequirementDefenseContract,
			Key:         "player_guard",
			ContractID:  playerGuardDefenseContractID,
			Description: "player shield guard defense contract",
		},
		{
			Category:    runtimeRequirementDefenseContract,
			Key:         "creature_guard",
			ContractID:  creatureGuardDefenseContractID,
			Description: "wolf hit-vs-guard interaction contract",
		},
		{
			Category:    runtimeRequirementWeaponKit,
			Key:         "sword_shield",
			ContractID:  "weaponkit_sword_shield",
			Description: "current player sword/shield combat mode slots",
		},
		{
			Category:    runtimeRequirementWolfBrainPolicy,
			Key:         "steppe_wolf",
			ContractID:  wolfRuntimeContractID,
			Description: "wolf creature brain runtime contract",
		},
	}
}

func runtimeRequirementsByCategory(category string) []RuntimeContractRequirement {
	requirements := runtimeContractRequirements()
	out := make([]RuntimeContractRequirement, 0, len(requirements))
	for _, requirement := range requirements {
		if requirement.Category == category {
			out = append(out, requirement)
		}
	}
	return out
}

func requirementStatusValues(contracts RuntimeContracts) map[string]string {
	out := map[string]string{}
	for _, requirement := range runtimeContractRequirements() {
		out["contracts.required."+requirement.Category+"."+requirement.Key] = contracts.requirementStatus(requirement)
	}
	return out
}

func (c RuntimeContracts) requirementStatus(requirement RuntimeContractRequirement) string {
	switch requirement.Category {
	case runtimeRequirementMovementProfile:
		if c.MovementProfile == nil || c.MovementProfile.GetProfileId() == "" {
			return "missing:" + requirement.ContractID
		}
		if c.MovementProfile.GetProfileId() != requirement.ContractID {
			return fmt.Sprintf("mismatch:%s!=%s", c.MovementProfile.GetProfileId(), requirement.ContractID)
		}
		return "ready:" + c.MovementProfile.GetProfileId()
	case runtimeRequirementBaseMovementAction:
		action := c.ActionContracts[requirement.Key]
		if action.ID == "" {
			return "missing:" + requirement.ContractID
		}
		if action.ID != requirement.ContractID {
			return fmt.Sprintf("mismatch:%s!=%s", action.ID, requirement.ContractID)
		}
		return "ready:" + action.ID
	case runtimeRequirementSkill:
		skill := c.SkillContracts[requirement.Key]
		if skill.SkillID == "" {
			return "missing"
		}
		if !skill.Enabled {
			return "disabled"
		}
		if skill.MovementAction.ID == "" {
			return "missing_movement_action"
		}
		return "ready:" + skill.MovementAction.ID
	case runtimeRequirementCombatCoreProfile:
		profile := c.CombatCore.Profiles[requirement.ContractID]
		if profile == nil {
			return "missing:" + requirement.ContractID
		}
		return "ready:" + requirement.ContractID
	case runtimeRequirementDefenseContract:
		contract := c.Defense.Contracts[requirement.ContractID]
		if contract == nil {
			return "missing:" + requirement.ContractID
		}
		return "ready:" + requirement.ContractID
	case runtimeRequirementWeaponKit:
		if len(c.CombatModes) == 0 {
			return "missing:" + requirement.ContractID
		}
		return "ready:" + requirement.ContractID
	case runtimeRequirementWolfBrainPolicy:
		if c.WolfPolicy.ContractID == "" {
			return "missing:" + requirement.ContractID
		}
		if c.WolfPolicy.ContractID != requirement.ContractID {
			return fmt.Sprintf("mismatch:%s!=%s", c.WolfPolicy.ContractID, requirement.ContractID)
		}
		return "ready:" + c.WolfPolicy.ContractID
	default:
		return "unknown_category"
	}
}
