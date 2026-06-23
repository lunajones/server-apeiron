package gameapi

import "fmt"

const (
	contractSurfaceFinalAuthority        = "final_authority"
	contractSurfaceCompatRuntimeRequired = "compat_runtime_required"
	contractSurfaceSchemaCompatOnly      = "schema_compat_only"
	contractSurfaceDeadCandidate         = "dead_candidate"
)

type RuntimeContractSurface struct {
	Name                    string
	Status                  string
	NormalRuntimeAuthority  bool
	ExposedCompatibilityAPI bool
	RuntimeConsumer         string
	CanonicalReplacement    string
	Decision                string
}

func runtimeContractSurfaces() []RuntimeContractSurface {
	return []RuntimeContractSurface{
		{
			Name:                   "movement_action_contract",
			Status:                 contractSurfaceFinalAuthority,
			NormalRuntimeAuthority: true,
			RuntimeConsumer:        "ProfileDataService.GetMovementActionContract -> LoadRuntimeContractsFromDB",
			Decision:               "canonical action movement source for normal, dodge, leap, turn, player skills, and creature skills",
		},
		{
			Name:                   "skill_movement_action_binding",
			Status:                 contractSurfaceFinalAuthority,
			NormalRuntimeAuthority: true,
			RuntimeConsumer:        "SkillDataService.GetSkillMovementActionBinding -> LoadRuntimeContractsFromDB",
			Decision:               "canonical skill-to-movement-action binding; owns starts_at_phase, handoff, normal input, target, and contact policies",
		},
		{
			Name:                   "runtime_movement_reconciliation_profile",
			Status:                 contractSurfaceFinalAuthority,
			NormalRuntimeAuthority: true,
			RuntimeConsumer:        "ProfileDataService.GetRuntimeMovementReconciliationProfile -> LoadRuntimeContractsFromDB",
			Decision:               "canonical rich reconciliation profile shared with Unreal; missing values fail strict coverage",
		},
		{
			Name:                    "skill_movement_effect/GetSkillMovementEffect",
			Status:                  contractSurfaceCompatRuntimeRequired,
			NormalRuntimeAuthority:  false,
			ExposedCompatibilityAPI: true,
			RuntimeConsumer:         "none in server normal runtime loader",
			CanonicalReplacement:    "skill_movement_action_binding + movement_action_contract",
			Decision:                "keep endpoint coherent for older compatibility clients/tools only; never use it to decide player or creature skill root motion",
		},
		{
			Name:                   "temporal_melee_hitbox_motion_profiles",
			Status:                 contractSurfaceFinalAuthority,
			NormalRuntimeAuthority: true,
			RuntimeConsumer:        "SkillDataService.GetSkillHitboxProfiles -> pending temporal impact runner",
			Decision:               "canonical melee damage volume source for player and creature skills",
		},
		{
			Name:                   "skill_hitbox_motion_profile.hitbox_profile_id",
			Status:                 contractSurfaceSchemaCompatOnly,
			NormalRuntimeAuthority: false,
			CanonicalReplacement:   "skill_hitbox_profile.motion_profile_id + skill_hitbox_motion_sample.motion_profile_id",
			Decision:               "schema compatibility tolerance only; runtime must use the canonical motion profile relation",
		},
		{
			Name:                   "skill_hitbox_motion_sample.id_text_compat_shape",
			Status:                 contractSurfaceSchemaCompatOnly,
			NormalRuntimeAuthority: false,
			CanonicalReplacement:   "motion_profile_id + sample_index",
			Decision:               "migration normalization bridge only; not a gameplay key",
		},
		{
			Name:                   "creature_behavior_runtime_contract",
			Status:                 contractSurfaceFinalAuthority,
			NormalRuntimeAuthority: true,
			RuntimeConsumer:        "ProfileDataService.GetCreatureBehaviorRuntimeContract -> wolfBrainPolicy",
			Decision:               "canonical creature brain contract for wolf policy, stamina, repetition pressure, and policy links",
		},
		{
			Name:                   "creature_behavior_runtime_contract.display_name/combat_role_id",
			Status:                 contractSurfaceSchemaCompatOnly,
			NormalRuntimeAuthority: false,
			CanonicalReplacement:   "creature_template_id + behavior policy ids",
			Decision:               "old catalog/UI requirements only; AI runtime must not use them",
		},
		{
			Name:                   "creature_skill_setup_policy.setup_tactic",
			Status:                 contractSurfaceSchemaCompatOnly,
			NormalRuntimeAuthority: false,
			CanonicalReplacement:   "setup_type + movement_tactic",
			Decision:               "stale setup column tolerance only; creature brain reads setup_type and movement_tactic",
		},
		{
			Name:                   "creature_orbit_policy.compat_radius_columns",
			Status:                 contractSurfaceSchemaCompatOnly,
			NormalRuntimeAuthority: false,
			CanonicalReplacement:   "behavior range policy + orbit locomotion policy",
			Decision:               "nullable compatibility columns only; do not revive as orbit authority",
		},
		{
			Name:                   "033_schema_compatibility_numeric_defaults",
			Status:                 contractSurfaceSchemaCompatOnly,
			NormalRuntimeAuthority: false,
			CanonicalReplacement:   "bootstrap seeds and strict DB contract coverage",
			Decision:               "old-DB boot bridge only; tuned movement values must come from final contract rows",
		},
		{
			Name:                   "original_modern_migration_numbering",
			Status:                 contractSurfaceDeadCandidate,
			NormalRuntimeAuthority: false,
			CanonicalReplacement:   "current compact canonical migrations plus compatibility map",
			Decision:               "filename numbering alone is not runtime authority; do not recreate duplicate migrations just for old numbers",
		},
	}
}

func compatRuntimeSurfaceBlockers() []string {
	var blockers []string
	for _, surface := range runtimeContractSurfaces() {
		if surface.NormalRuntimeAuthority && surface.Status != contractSurfaceFinalAuthority {
			blockers = append(blockers, fmt.Sprintf("%s is %s but marked normal runtime authority", surface.Name, surface.Status))
		}
		if surface.Status == contractSurfaceFinalAuthority && surface.CanonicalReplacement != "" {
			blockers = append(blockers, fmt.Sprintf("%s is final authority but still points to replacement %s", surface.Name, surface.CanonicalReplacement))
		}
	}
	return blockers
}

func compatRuntimeSurfaceStatusValues() map[string]string {
	out := map[string]string{}
	for _, surface := range runtimeContractSurfaces() {
		status := surface.Status
		if surface.NormalRuntimeAuthority {
			status += ":runtime_authority"
		}
		if surface.ExposedCompatibilityAPI {
			status += ":compat_api"
		}
		out["contracts.surface."+surface.Name] = status
	}
	return out
}
