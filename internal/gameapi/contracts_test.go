package gameapi

import (
	"context"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	dbv1 "db-apeiron/gen/apeiron/v1"
	gamev1 "server-apeiron/gen/apeiron/game/v1"

	"google.golang.org/grpc"
)

func TestDevFixtureRuntimeContractsExposeRequiredSkillContracts(t *testing.T) {
	contracts := DevFixtureRuntimeContracts()
	if err := contracts.ValidateRequiredCoverage(false); err != nil {
		t.Fatalf("fixture contract coverage failed: %v", err)
	}
	for _, skillID := range requiredRuntimeSkillIDs() {
		skill := contracts.skillContract(skillID)
		if !skill.Enabled {
			t.Fatalf("%s contract is not enabled", skillID)
		}
		if skill.MovementActionContractID == "" {
			t.Fatalf("%s has no movement action contract id", skillID)
		}
		if skill.MovementAction.ReconciliationContractID == "" {
			t.Fatalf("%s has no reconciliation contract id", skillID)
		}
		if contracts.ActionContracts[skillID].ID == "" {
			t.Fatalf("%s is missing from action contract manifest", skillID)
		}
	}
}

func TestCurrentPlayerAndCreatureDamagingSkillsUseTemporalHitboxes(t *testing.T) {
	t.Parallel()

	contracts := DevFixtureRuntimeContracts()
	cases := []struct {
		skillID       string
		motionID      string
		sweepShape    string
		damageGroupID string
	}{
		{"player_basic_attack_1", "motion_player_basic_attack_1_forward_v1", "box_strip", "player_basic_attack_1_damage"},
		{"player_basic_attack_2", "motion_player_basic_attack_2_right_to_left_v1", "arc_slice", "player_basic_attack_2_damage"},
		{"player_basic_attack_3", "motion_player_basic_attack_3_shield_drive_v1", "capsule_strip", "player_basic_attack_3_damage"},
		{"player_shield_bash", "motion_player_shield_bash_front_push_v1", "capsule_strip", "player_shield_bash_front_push"},
		{"player_shield_rush", "motion_player_shield_rush_front_contact_v1", "box_strip", "player_shield_rush_front_contact"},
		{"bite", "motion_wolf_bite_melee_v1", "capsule_strip", "wolf_bite_damage"},
		{"lunge", "motion_wolf_lunge_cross_v1", "capsule_strip", "wolf_lunge_damage"},
		{"maul", "motion_wolf_maul_lateral_counter_v1", "arc_slice", "wolf_maul_damage"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.skillID, func(t *testing.T) {
			t.Parallel()

			skill := contracts.skillContract(tc.skillID)
			if skill.SkillID != tc.skillID {
				t.Fatalf("missing skill contract %q (got %q)", tc.skillID, skill.SkillID)
			}
			if skill.Damage <= 0 && skill.PostureDamage <= 0 {
				t.Fatalf("%s is not a damaging/posture skill in runtime contract", tc.skillID)
			}
			if len(skill.Hitboxes) != 1 {
				t.Fatalf("%s hitboxes = %d, want exactly one canonical temporal hitbox", tc.skillID, len(skill.Hitboxes))
			}

			profile := skill.Hitboxes[0]
			if profile.GetHitboxShape() != "temporal_sweep" {
				t.Fatalf("%s hitbox shape = %q, want temporal_sweep", tc.skillID, profile.GetHitboxShape())
			}
			if profile.GetDamageGroupId() != tc.damageGroupID {
				t.Fatalf("%s damage group = %q, want %q", tc.skillID, profile.GetDamageGroupId(), tc.damageGroupID)
			}

			motion := profile.GetMotionProfile()
			if motion == nil || !motion.GetEnabled() {
				t.Fatalf("%s temporal motion profile missing or disabled: %#v", tc.skillID, motion)
			}
			if motion.GetId() != tc.motionID {
				t.Fatalf("%s motion profile = %q, want %q", tc.skillID, motion.GetId(), tc.motionID)
			}
			if motion.GetMotionType() != "timeline_sweep" {
				t.Fatalf("%s motion type = %q, want timeline_sweep", tc.skillID, motion.GetMotionType())
			}
			if motion.GetTimeBasis() != "hitbox_window_normalized" {
				t.Fatalf("%s time basis = %q, want hitbox_window_normalized", tc.skillID, motion.GetTimeBasis())
			}
			if motion.GetSweepShape() != tc.sweepShape {
				t.Fatalf("%s sweep shape = %q, want %q", tc.skillID, motion.GetSweepShape(), tc.sweepShape)
			}
			if motion.GetDamageGroupId() != tc.damageGroupID {
				t.Fatalf("%s motion damage group = %q, want %q", tc.skillID, motion.GetDamageGroupId(), tc.damageGroupID)
			}
			if len(motion.GetSamples()) < 3 {
				t.Fatalf("%s temporal samples = %d, want at least 3", tc.skillID, len(motion.GetSamples()))
			}
		})
	}
}

func TestDevFixturePlayerShieldKitMatchesCanonicalMotionGeometry(t *testing.T) {
	contracts := DevFixtureRuntimeContracts()

	cases := []struct {
		skillID      string
		distanceCM   float64
		baseSpeedCMS float64
		sweepShape   string
		lengths      []float64
		offsetsX     []float64
		offsetsY     []float64
		radii        []float64
		sizeY        []float64
	}{
		{
			skillID:      "player_basic_attack_1",
			distanceCM:   84,
			baseSpeedCMS: 240,
			sweepShape:   "box_strip",
			lengths:      []float64{28, 46, 64},
			offsetsX:     []float64{0, 0, 0},
			offsetsY:     []float64{0, 0, 0},
			radii:        []float64{26, 26, 26},
			sizeY:        []float64{52, 52, 52},
		},
		{
			skillID:      "player_basic_attack_2",
			distanceCM:   42,
			baseSpeedCMS: 114,
			sweepShape:   "arc_slice",
			lengths:      []float64{125, 135, 125},
			offsetsX:     []float64{70, 80, 70},
			offsetsY:     []float64{-35, 0, 35},
			radii:        []float64{50, 52, 50},
			sizeY:        []float64{0, 0, 0},
		},
		{
			skillID:      "player_basic_attack_3",
			distanceCM:   252,
			baseSpeedCMS: 406.4,
			sweepShape:   "capsule_strip",
			lengths:      []float64{42, 140, 252},
			offsetsX:     []float64{0, 0, 0},
			offsetsY:     []float64{0, 0, 0},
			radii:        []float64{42, 42, 42},
			sizeY:        []float64{0, 0, 0},
		},
		{
			skillID:      "player_shield_bash",
			distanceCM:   95,
			baseSpeedCMS: 541,
			sweepShape:   "capsule_strip",
			lengths:      []float64{75, 120, 160},
			offsetsX:     []float64{45, 72, 92},
			offsetsY:     []float64{0, 0, 0},
			radii:        []float64{66, 66, 66},
			sizeY:        []float64{0, 0, 0},
		},
		{
			skillID:      "player_shield_rush",
			distanceCM:   864,
			baseSpeedCMS: 1033.2,
			sweepShape:   "box_strip",
			lengths:      []float64{34, 44, 54},
			offsetsX:     []float64{8, 10, 12},
			offsetsY:     []float64{0, 0, 0},
			radii:        []float64{112, 112, 112},
			sizeY:        []float64{224, 224, 224},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.skillID, func(t *testing.T) {
			skill := contracts.skillContract(tc.skillID)
			if math.Abs(skill.MovementAction.DistanceCM-tc.distanceCM) > 0.001 {
				t.Fatalf("%s distance = %v, want %v", tc.skillID, skill.MovementAction.DistanceCM, tc.distanceCM)
			}
			if math.Abs(skill.MovementAction.BaseSpeedCMS-tc.baseSpeedCMS) > 0.001 {
				t.Fatalf("%s base speed = %v, want %v", tc.skillID, skill.MovementAction.BaseSpeedCMS, tc.baseSpeedCMS)
			}
			if len(skill.Hitboxes) != 1 || skill.Hitboxes[0].GetMotionProfile() == nil {
				t.Fatalf("%s missing canonical motion profile", tc.skillID)
			}
			motion := skill.Hitboxes[0].GetMotionProfile()
			if motion.GetSweepShape() != tc.sweepShape {
				t.Fatalf("%s sweep shape = %q, want %q", tc.skillID, motion.GetSweepShape(), tc.sweepShape)
			}
			samples := motion.GetSamples()
			if len(samples) != len(tc.lengths) {
				t.Fatalf("%s sample count = %d, want %d", tc.skillID, len(samples), len(tc.lengths))
			}
			for i, sample := range samples {
				if math.Abs(sample.GetLength()-tc.lengths[i]) > 0.001 {
					t.Fatalf("%s sample %d length = %v, want %v", tc.skillID, i, sample.GetLength(), tc.lengths[i])
				}
				if math.Abs(sample.GetOffsetX()-tc.offsetsX[i]) > 0.001 {
					t.Fatalf("%s sample %d offset_x = %v, want %v", tc.skillID, i, sample.GetOffsetX(), tc.offsetsX[i])
				}
				if math.Abs(sample.GetOffsetY()-tc.offsetsY[i]) > 0.001 {
					t.Fatalf("%s sample %d offset_y = %v, want %v", tc.skillID, i, sample.GetOffsetY(), tc.offsetsY[i])
				}
				if math.Abs(sample.GetRadius()-tc.radii[i]) > 0.001 {
					t.Fatalf("%s sample %d radius = %v, want %v", tc.skillID, i, sample.GetRadius(), tc.radii[i])
				}
				if len(tc.sizeY) > i && math.Abs(sample.GetSizeY()-tc.sizeY[i]) > 0.001 {
					t.Fatalf("%s sample %d size_y = %v, want %v", tc.skillID, i, sample.GetSizeY(), tc.sizeY[i])
				}
			}
		})
	}
}

func TestRuntimeContractRequirementsDriveRequiredSkillAndMovementLists(t *testing.T) {
	requiredSkills := requiredRuntimeSkillIDs()
	for _, skillID := range []string{
		"player_basic_attack_1",
		"player_basic_attack_2",
		"player_basic_attack_3",
		"player_shield_bash",
		"player_shield_rush",
		"bite",
		"lunge",
		"wolf_dodge",
		"maul",
	} {
		if !stringSliceContains(requiredSkills, skillID) {
			t.Fatalf("runtime requirement manifest is missing skill %s: %#v", skillID, requiredSkills)
		}
	}

	requiredActions := requiredBaseMovementActions()
	for abilityKey, contractID := range map[string]string{
		"move":  "grounded_move_v1",
		"turn":  "turn_v1_rate_limited_contextual",
		"dodge": "dodge_v1_full_iframe",
		"jump":  "jump_v1_authoritative_grounded_handoff",
	} {
		var found bool
		for _, got := range requiredActions {
			if got.abilityKey == abilityKey && got.contractID == contractID {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("runtime requirement manifest missing base movement action %s -> %s: %#v", abilityKey, contractID, requiredActions)
		}
	}
}

func TestRuntimeRequirementStatusValuesExposeRequiredContractReadiness(t *testing.T) {
	contracts := LoadRuntimeContractsFromDB(context.Background(), fakeRuntimeContractSource{}, fakeRuntimeContractSource{}, fakeRuntimeContractSource{})
	status := requirementStatusValues(contracts)

	for _, key := range []string{
		"contracts.required.movement_profile.runtime_movement",
		"contracts.required.base_movement_action.dodge",
		"contracts.required.skill.player_shield_rush",
		"contracts.required.skill.lunge",
		"contracts.required.combat_core_profile.player",
		"contracts.required.defense_contract.player_guard",
		"contracts.required.weapon_kit.sword_shield",
		"contracts.required.wolf_brain_policy.steppe_wolf",
	} {
		if !strings.HasPrefix(status[key], "ready:") {
			t.Fatalf("requirement %s status = %q", key, status[key])
		}
	}

	delete(contracts.SkillContracts, "player_shield_rush")
	status = requirementStatusValues(contracts)
	if status["contracts.required.skill.player_shield_rush"] != "missing" {
		t.Fatalf("missing skill requirement status = %q", status["contracts.required.skill.player_shield_rush"])
	}
}

func TestStrictRuntimeCoverageRejectsDamagingSkillWithoutTemporalHitbox(t *testing.T) {
	contracts := DevFixtureRuntimeContracts()
	skill := contracts.SkillContracts["player_shield_rush"]
	skill.Hitboxes = nil
	contracts.SkillContracts["player_shield_rush"] = skill

	err := contracts.ValidateRequiredCoverage(true)
	if err == nil {
		t.Fatal("ValidateRequiredCoverage accepted damaging skill without temporal hitbox")
	}
	if !strings.Contains(err.Error(), "skill temporal hitbox player_shield_rush") {
		t.Fatalf("missing temporal hitbox blocker not reported: %v", err)
	}
}

func TestStrictRuntimeCoverageRejectsSkillWithoutMovementPhasePolicies(t *testing.T) {
	contracts := DevFixtureRuntimeContracts()
	skill := contracts.SkillContracts["lunge"]
	skill.StartsAtPhase = ""
	skill.HandoffPolicy = ""
	skill.NormalInputPolicy = ""
	contracts.SkillContracts["lunge"] = skill

	err := contracts.ValidateRequiredCoverage(true)
	if err == nil {
		t.Fatal("ValidateRequiredCoverage accepted skill without movement phase policies")
	}
	for _, want := range []string{
		"skill movement starts phase lunge",
		"skill movement handoff policy lunge",
		"skill movement normal input policy lunge",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("missing blocker %q in %v", want, err)
		}
	}
}

func TestStrictRuntimeCoverageRejectsPushContactSkillWithoutControlEffect(t *testing.T) {
	contracts := DevFixtureRuntimeContracts()
	skill := contracts.SkillContracts["player_shield_rush"]
	skill.ControlEffects = nil
	if skill.Impact != nil {
		copy := *skill.Impact
		copy.ControlEffects = nil
		skill.Impact = &copy
	}
	contracts.SkillContracts["player_shield_rush"] = skill

	err := contracts.ValidateRequiredCoverage(true)
	if err == nil {
		t.Fatal("ValidateRequiredCoverage accepted push contact skill without control effect")
	}
	if !strings.Contains(err.Error(), "skill impact control effect player_shield_rush") {
		t.Fatalf("missing push control blocker not reported: %v", err)
	}
}

func TestStrictRuntimeCoverageRejectsLateralCounterWithoutControlEffect(t *testing.T) {
	contracts := DevFixtureRuntimeContracts()
	skill := contracts.SkillContracts["maul"]
	skill.ControlEffects = nil
	if skill.Impact != nil {
		copy := *skill.Impact
		copy.ControlEffects = nil
		skill.Impact = &copy
	}
	contracts.SkillContracts["maul"] = skill

	err := contracts.ValidateRequiredCoverage(true)
	if err == nil {
		t.Fatal("ValidateRequiredCoverage accepted lateral counter skill without control effect")
	}
	if !strings.Contains(err.Error(), "skill impact control effect maul") {
		t.Fatalf("missing maul control blocker not reported: %v", err)
	}
}

func TestStrictRuntimeCoverageRejectsIncompleteControlEffectMotion(t *testing.T) {
	contracts := DevFixtureRuntimeContracts()
	skill := contracts.SkillContracts["player_shield_rush"]
	if len(skill.ControlEffects) == 0 {
		t.Fatal("fixture Shield Rush should have a control effect")
	}
	effect := *skill.ControlEffects[0]
	effect.ControlType = ""
	effect.ReleasePolicyId = ""
	effect.DirectionPolicy = ""
	effect.DurationMs = 0
	effect.DistanceCm = 0
	effect.SpeedCmS = 0
	skill.ControlEffects = []*dbv1.SkillControlEffect{&effect}
	if skill.Impact != nil {
		copy := *skill.Impact
		copy.ControlEffects = skill.ControlEffects
		skill.Impact = &copy
	}
	contracts.SkillContracts["player_shield_rush"] = skill

	err := contracts.ValidateRequiredCoverage(true)
	if err == nil {
		t.Fatal("ValidateRequiredCoverage accepted incomplete control effect motion")
	}
	for _, want := range []string{
		"skill control effect player_shield_rush/",
		"control type",
		"release policy",
		"direction policy",
		"duration",
		"distance",
		"speed",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("missing blocker %q in %v", want, err)
		}
	}
}

func TestStrictRuntimeCoverageRejectsTemporalMotionSampleWithoutGeometry(t *testing.T) {
	contracts := DevFixtureRuntimeContracts()
	skill := contracts.SkillContracts["player_basic_attack_1"]
	if len(skill.Hitboxes) == 0 || skill.Hitboxes[0].GetMotionProfile() == nil {
		t.Fatal("fixture basic attack should have temporal hitbox samples")
	}
	profile := *skill.Hitboxes[0]
	profile.Radius = 0
	profile.SizeY = 0
	motion := *profile.GetMotionProfile()
	samples := make([]*dbv1.SkillHitboxMotionSample, 0, len(motion.GetSamples()))
	for _, original := range motion.GetSamples() {
		if original == nil {
			continue
		}
		sample := *original
		sample.Radius = 0
		sample.SizeY = 0
		samples = append(samples, &sample)
	}
	motion.Samples = samples
	profile.MotionProfile = &motion
	skill.Hitboxes = []*dbv1.SkillHitboxProfile{&profile}
	contracts.SkillContracts["player_basic_attack_1"] = skill

	err := contracts.ValidateRequiredCoverage(true)
	if err == nil {
		t.Fatal("ValidateRequiredCoverage accepted temporal samples without width/radius")
	}
	if !strings.Contains(err.Error(), "skill motion sample player_basic_attack_1/") || !strings.Contains(err.Error(), "width") {
		t.Fatalf("missing temporal geometry blocker not reported: %v", err)
	}
}

func TestStrictRuntimeCoverageRejectsIncompleteMovementActionContract(t *testing.T) {
	contracts := DevFixtureRuntimeContracts()
	action := contracts.ActionContracts["player_shield_rush"]
	action.PhaseWindowPolicy = ""
	action.PredictionErrorPolicy = ""
	action.RootMotionOwner = ""
	action.ContactPolicy = ""
	contracts.ActionContracts["player_shield_rush"] = action
	skill := contracts.SkillContracts["player_shield_rush"]
	skill.MovementAction = action
	contracts.SkillContracts["player_shield_rush"] = skill

	err := contracts.ValidateRequiredCoverage(true)
	if err == nil {
		t.Fatal("ValidateRequiredCoverage accepted incomplete skill movement action contract")
	}
	for _, want := range []string{
		"skill movement player_shield_rush phase window policy",
		"skill movement player_shield_rush prediction error policy",
		"skill movement player_shield_rush root motion owner",
		"skill movement player_shield_rush contact policy",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("missing blocker %q in %v", want, err)
		}
	}
}

func TestStrictRuntimeCoverageRejectsSkillBindingActionMismatch(t *testing.T) {
	contracts := DevFixtureRuntimeContracts()
	skill := contracts.SkillContracts["player_shield_rush"]
	skill.MovementAction.ID = "some_other_contract"
	contracts.SkillContracts["player_shield_rush"] = skill

	err := contracts.ValidateRequiredCoverage(true)
	if err == nil {
		t.Fatal("ValidateRequiredCoverage accepted mismatched skill movement binding")
	}
	if !strings.Contains(err.Error(), "skill movement binding/action mismatch player_shield_rush") {
		t.Fatalf("binding/action mismatch not reported: %v", err)
	}
}

func TestStrictRuntimeCoverageRejectsSkillActionManifestMismatch(t *testing.T) {
	contracts := DevFixtureRuntimeContracts()
	action := contracts.ActionContracts["player_shield_rush"]
	action.ID = "some_other_contract"
	contracts.ActionContracts["player_shield_rush"] = action

	err := contracts.ValidateRequiredCoverage(true)
	if err == nil {
		t.Fatal("ValidateRequiredCoverage accepted mismatched skill action manifest")
	}
	if !strings.Contains(err.Error(), "skill action manifest mismatch player_shield_rush") {
		t.Fatalf("action manifest mismatch not reported: %v", err)
	}
}

func TestStrictRuntimeCoverageRejectsCombatModeSlotWithoutRuntimeSkill(t *testing.T) {
	contracts := DevFixtureRuntimeContracts()
	delete(contracts.SkillContracts, "player_shield_rush")

	err := contracts.ValidateRequiredCoverage(true)
	if err == nil {
		t.Fatal("ValidateRequiredCoverage accepted combat mode slot pointing at missing skill runtime")
	}
	if !strings.Contains(err.Error(), "combat mode slot mode_sword_shield_bulwark:3 references missing runtime skill player_shield_rush") {
		t.Fatalf("combat mode missing skill blocker not reported: %v", err)
	}
}

func TestStrictRuntimeCoverageAllowsNonDamagingMovementSkillWithoutHitbox(t *testing.T) {
	contracts := DevFixtureRuntimeContracts()
	skill := contracts.SkillContracts["wolf_dodge"]
	if skill.Damage != 0 || skill.PostureDamage != 0 {
		t.Fatalf("wolf_dodge fixture unexpectedly damages: %#v", skill)
	}
	skill.Hitboxes = nil
	contracts.SkillContracts["wolf_dodge"] = skill

	if err := contracts.ValidateRequiredCoverage(true); err != nil {
		t.Fatalf("ValidateRequiredCoverage rejected non-damaging movement skill without hitbox: %v", err)
	}
}

func TestDevFixtureRuntimeContractsExposeCreatureSkillContracts(t *testing.T) {
	contracts := DevFixtureRuntimeContracts()
	for _, skillID := range []string{"bite", "lunge", "wolf_dodge", "maul"} {
		skill := contracts.skillContract(skillID)
		if !skill.Enabled {
			t.Fatalf("%s contract is not enabled", skillID)
		}
		if skill.MovementActionContractID == "" {
			t.Fatalf("%s has no movement action contract id", skillID)
		}
		if skill.MovementAction.ReconciliationContractID == "" {
			t.Fatalf("%s has no reconciliation contract id", skillID)
		}
	}
	if !hasCreatureSkillBehaviorBinding(contracts.WolfPolicy.SkillBehaviorBindings, "lunge", "circle", "reposition") {
		t.Fatalf("fixture wolf lunge binding missing: %#v", contracts.WolfPolicy.SkillBehaviorBindings)
	}
	if !hasCreatureSkillBehaviorBinding(contracts.WolfPolicy.SkillBehaviorBindings, "maul", "pressure", "counter") {
		t.Fatalf("fixture wolf maul binding missing: %#v", contracts.WolfPolicy.SkillBehaviorBindings)
	}
}

func TestDevFixtureCombatModesKeepBulwarkAndVanguardSeparate(t *testing.T) {
	contracts := DevFixtureRuntimeContracts()
	if !hasCombatModeSlot(contracts.CombatModes, swordShieldBulwarkModeID, 0, "player_basic_attack_1") {
		t.Fatalf("fixture Bulwark M1 slot missing: %#v", contracts.CombatModes)
	}
	if !hasCombatModeSlot(contracts.CombatModes, swordShieldBulwarkModeID, 2, "player_shield_bash") {
		t.Fatalf("fixture Bulwark R slot missing: %#v", contracts.CombatModes)
	}
	if !hasCombatModeSlot(contracts.CombatModes, swordShieldBulwarkModeID, 3, "player_shield_rush") {
		t.Fatalf("fixture Bulwark F slot missing: %#v", contracts.CombatModes)
	}
	for _, slotIndex := range []uint32{0, 1, 2, 3, 4} {
		if !hasEmptyCombatModeSlot(contracts.CombatModes, swordShieldVanguardModeID, slotIndex) {
			t.Fatalf("fixture Vanguard slot %d should be empty/disabled: %#v", slotIndex, contracts.CombatModes)
		}
	}
}

func TestLoadRuntimeContractsFromDBUsesRequiredSkillBindings(t *testing.T) {
	source := fakeRuntimeContractSource{}
	contracts := LoadRuntimeContractsFromDB(context.Background(), source, source, source)
	if err := contracts.ValidateRequiredCoverage(true); err != nil {
		t.Fatalf("db contract coverage failed: %v", err)
	}
	if contracts.Source != runtimeContractSourceDB {
		t.Fatalf("source = %q, want %s", contracts.Source, runtimeContractSourceDB)
	}
	if contracts.MovementProfile.GetProfileId() != runtimeMovementReconciliationProfileID {
		t.Fatalf("movement reconciliation profile = %q", contracts.MovementProfile.GetProfileId())
	}

	for _, skillID := range requiredRuntimeSkillIDs() {
		skill := contracts.skillContract(skillID)
		expectedContractID := fakeSkillMovementActionContractID(skillID)
		if skill.MovementActionContractID != expectedContractID {
			t.Fatalf("%s movement binding = %q", skillID, skill.MovementActionContractID)
		}
		if contracts.ActionContracts[skillID].ID != expectedContractID {
			t.Fatalf("%s action contract id = %q", skillID, contracts.ActionContracts[skillID].ID)
		}
	}
	for _, action := range requiredBaseMovementActions() {
		if contracts.ActionContracts[action.abilityKey].ID != action.contractID {
			t.Fatalf("%s base action contract id = %q", action.abilityKey, contracts.ActionContracts[action.abilityKey].ID)
		}
	}
	if profile := contracts.CombatCore.Profiles[playerCombatCoreProfileID]; profile == nil || !profile.GetCanBlock() || profile.GetMaxPosture() != 100 {
		t.Fatalf("player combat core did not load from DB fake: %#v", profile)
	}
	if profile := contracts.CombatCore.Profiles[creatureCombatCoreProfileID]; profile == nil || profile.GetCanBlock() || profile.GetMaxPosture() != 65 {
		t.Fatalf("creature combat core did not load from DB fake: %#v", profile)
	}
	if contract := contracts.Defense.Contracts[playerGuardDefenseContractID]; contract == nil || contract.GetFrontalArcDeg() != 120 || contract.GetDefenseType() != "shield_block" {
		t.Fatalf("player guard defense contract did not load from DB fake: %#v", contract)
	}
	if contract := contracts.Defense.Contracts[creatureGuardDefenseContractID]; contract == nil || contract.GetDefenseType() != "incoming_melee" {
		t.Fatalf("creature guard defense contract did not load from DB fake: %#v", contract)
	}
	if !hasCombatModeSlot(contracts.CombatModes, "mode_sword_shield_bulwark", 3, "player_shield_rush") {
		t.Fatalf("DB combat mode slots did not load Bulwark F -> Shield Rush: %#v", contracts.CombatModes)
	}
	if !hasCombatModeSlot(contracts.CombatModes, "mode_sword_shield_bulwark", 0, "player_basic_attack_1") {
		t.Fatalf("DB combat mode slots did not load Bulwark M1 -> basic attack: %#v", contracts.CombatModes)
	}
	if effects := contracts.SkillContracts["player_shield_rush"].ControlEffects; len(effects) != 1 || effects[0].GetStatusEffectId() != "impact_shield_rush_carry_push" {
		t.Fatalf("DB skill impact control effects did not load for Shield Rush: %#v", effects)
	}
	if action := contracts.SkillContracts["player_shield_rush"].MovementAction; action.DistanceCM != 864 || action.DurationMS != 1100 || action.ActiveMS != 720 || action.RecoveryMS != 260 {
		t.Fatalf("Shield Rush movement envelope = distance %.1f duration %d active %d recovery %d, want canonical 864/1100/720/260", action.DistanceCM, action.DurationMS, action.ActiveMS, action.RecoveryMS)
	}
	if effects := contracts.SkillContracts["player_shield_rush"].ControlEffects; effects[0].GetDistanceCm() != 864 {
		t.Fatalf("Shield Rush control distance = %.1f, want canonical 864", effects[0].GetDistanceCm())
	}
	if impact := contracts.SkillContracts["player_shield_rush"].Impact; impact == nil || impact.GetImpactType() != "blunt" {
		t.Fatalf("DB skill impact profile did not load for Shield Rush: %#v", impact)
	}
	if !hasEmptyCombatModeSlot(contracts.CombatModes, "mode_sword_shield_vanguard", 0) {
		t.Fatalf("Vanguard M1 should be present but empty/disabled until implemented: %#v", contracts.CombatModes)
	}
	if hasCombatModeSlot(contracts.CombatModes, "mode_sword_shield_vanguard", 2, "player_shield_bash") {
		t.Fatalf("Vanguard must not inherit Bulwark R skill from fallback")
	}
	if hasCombatModeSlot(contracts.CombatModes, "mode_sword_shield_vanguard", 0, "player_basic_attack_1") {
		t.Fatalf("Vanguard must not expose M1 until sword-forward skills are implemented")
	}
	if contracts.WolfPolicy.TargetOpportunityPolicyID != "opportunity_wolf_harasser_v1" {
		t.Fatalf("wolf opportunity policy = %q", contracts.WolfPolicy.TargetOpportunityPolicyID)
	}
	if contracts.WolfPolicy.OrbitPolicyID != "orbit_wolf_harasser_combat_walk_v1" {
		t.Fatalf("wolf orbit policy = %q", contracts.WolfPolicy.OrbitPolicyID)
	}
	if contracts.WolfPolicy.OrbitLocomotionMode != "combat_walk" {
		t.Fatalf("wolf orbit locomotion mode = %q", contracts.WolfPolicy.OrbitLocomotionMode)
	}
	if contracts.WolfPolicy.MaxStamina != 100 || contracts.WolfPolicy.DodgeStaminaCostMultiplier != 0.5 || contracts.WolfPolicy.StaminaRegenPerSecond != 12 {
		t.Fatalf("wolf stamina policy = max %.1f dodge multiplier %.2f regen %.1f", contracts.WolfPolicy.MaxStamina, contracts.WolfPolicy.DodgeStaminaCostMultiplier, contracts.WolfPolicy.StaminaRegenPerSecond)
	}
	if contracts.WolfPolicy.RepeatSkillPenaltyWindowMS != 5200 || contracts.WolfPolicy.RepeatSkillPenaltyMultiplier != 0.35 {
		t.Fatalf("wolf repeat policy = window %d multiplier %.2f", contracts.WolfPolicy.RepeatSkillPenaltyWindowMS, contracts.WolfPolicy.RepeatSkillPenaltyMultiplier)
	}
	if contracts.WolfPolicy.DesiredRangeCM != 560 || contracts.WolfPolicy.OrbitSpeedCMS != 150 || contracts.WolfPolicy.ChaseSpeedCMS != 310 {
		t.Fatalf("wolf range/speed policy = desired %.0f orbit %.0f chase %.0f", contracts.WolfPolicy.DesiredRangeCM, contracts.WolfPolicy.OrbitSpeedCMS, contracts.WolfPolicy.ChaseSpeedCMS)
	}
	if contracts.WolfPolicy.DodgeCommittedThreatMultiplier != 1.12 || contracts.WolfPolicy.VulnerableBiteMultiplier != 1.16 || contracts.WolfPolicy.TacticalDestinationDistanceCM != 180 {
		t.Fatalf("wolf threat policy = dodge %.2f vulnerable bite %.2f destination %.0f", contracts.WolfPolicy.DodgeCommittedThreatMultiplier, contracts.WolfPolicy.VulnerableBiteMultiplier, contracts.WolfPolicy.TacticalDestinationDistanceCM)
	}
	if contracts.WolfPolicy.EvasionLateralBias != 0.72 || contracts.WolfPolicy.EvasionBackstepBias != 0.28 || contracts.WolfPolicy.EvasionPressureThreshold != 0.42 {
		t.Fatalf("wolf evasion policy = lateral %.2f back %.2f pressure %.2f", contracts.WolfPolicy.EvasionLateralBias, contracts.WolfPolicy.EvasionBackstepBias, contracts.WolfPolicy.EvasionPressureThreshold)
	}
	if contracts.WolfPolicy.LungeMinRangeCM != 180 || contracts.WolfPolicy.LungeMaxRangeCM != 700 {
		t.Fatalf("wolf lunge range = %.0f..%.0f", contracts.WolfPolicy.LungeMinRangeCM, contracts.WolfPolicy.LungeMaxRangeCM)
	}
	if !hasCreatureSkillBehaviorBinding(contracts.WolfPolicy.SkillBehaviorBindings, "lunge", "circle", "reposition") {
		t.Fatalf("wolf lunge circle/reposition binding missing: %#v", contracts.WolfPolicy.SkillBehaviorBindings)
	}
	if !hasCreatureSkillBehaviorBinding(contracts.WolfPolicy.SkillBehaviorBindings, "maul", "pressure", "counter") {
		t.Fatalf("wolf maul pressure/counter binding missing: %#v", contracts.WolfPolicy.SkillBehaviorBindings)
	}
	if !hasCreatureSkillSetupPolicy(contracts.WolfPolicy.SkillSetupPolicies, "wolf_lunge_flank_windup_v1", "lunge", "circle_then_curve_to_target") {
		t.Fatalf("wolf lunge setup policy missing: %#v", contracts.WolfPolicy.SkillSetupPolicies)
	}
	if !hasCreatureSkillSetupPolicy(contracts.WolfPolicy.SkillSetupPolicies, "wolf_maul_pressure_counter_v1", "maul", "lateral_counter_dash") {
		t.Fatalf("wolf maul setup policy missing: %#v", contracts.WolfPolicy.SkillSetupPolicies)
	}
}

func TestLoadRuntimeContractsFromDBDoesNotLeakFixtureCombatFallback(t *testing.T) {
	source := fakeRuntimeContractSource{
		missingCombatCoreProfiles: map[string]bool{playerCombatCoreProfileID: true},
		missingDefenseContracts:   map[string]bool{playerGuardDefenseContractID: true},
	}
	contracts := LoadRuntimeContractsFromDB(context.Background(), source, source, source)

	err := contracts.ValidateRequiredCoverage(true)
	if err == nil {
		t.Fatal("expected missing DB combat contracts to fail strict coverage")
	}
	if !strings.Contains(err.Error(), "missing combat core profile "+playerCombatCoreProfileID) {
		t.Fatalf("coverage error missing combat core failure: %v", err)
	}
	if !strings.Contains(err.Error(), "missing combat defense contract "+playerGuardDefenseContractID) {
		t.Fatalf("coverage error missing defense failure: %v", err)
	}
	if got := contracts.CombatCore.Profiles[playerCombatCoreProfileID]; got != nil {
		t.Fatalf("DB loader leaked fixture player combat core fallback: %#v", got)
	}
	if got := contracts.Defense.Contracts[playerGuardDefenseContractID]; got != nil {
		t.Fatalf("DB loader leaked fixture player guard fallback: %#v", got)
	}
}

func TestLoadRuntimeContractsFromDBRejectsMissingRequiredBinding(t *testing.T) {
	source := fakeRuntimeContractSource{missingSkills: map[string]bool{"maul": true}}
	contracts := LoadRuntimeContractsFromDB(context.Background(), source, source, source)

	err := contracts.ValidateRequiredCoverage(true)
	if err == nil {
		t.Fatal("expected missing DB runtime contract to fail strict coverage")
	}
	if !strings.Contains(err.Error(), "missing skill runtime maul") {
		t.Fatalf("coverage error = %v", err)
	}
}

func TestLoadRuntimeContractsFromDBDoesNotLeakFixtureAbilityFallback(t *testing.T) {
	source := fakeRuntimeContractSource{missingActions: map[string]bool{"dodge_v1_full_iframe": true}}
	contracts := LoadRuntimeContractsFromDB(context.Background(), source, source, source)

	err := contracts.ValidateRequiredCoverage(true)
	if err == nil {
		t.Fatal("expected missing DB movement action to fail strict coverage")
	}
	if !strings.Contains(err.Error(), "missing movement action dodge -> dodge_v1_full_iframe") {
		t.Fatalf("coverage error = %v", err)
	}
	if got := contracts.ActionContracts["dodge"]; got.ID != "" {
		t.Fatalf("DB loader leaked fixture dodge fallback: %#v", got)
	}
}

func TestLoadRuntimeContractsFromDBDoesNotLeakFixtureCombatModeFallback(t *testing.T) {
	source := fakeRuntimeContractSource{missingCombatModes: true}
	contracts := LoadRuntimeContractsFromDB(context.Background(), source, source, source)

	err := contracts.ValidateRequiredCoverage(true)
	if err == nil {
		t.Fatal("expected missing DB combat mode slots to fail strict coverage")
	}
	if !strings.Contains(err.Error(), "missing weapon combat mode slots weaponkit_sword_shield") {
		t.Fatalf("coverage error = %v", err)
	}
	if len(contracts.CombatModes) != 0 {
		t.Fatalf("DB loader leaked fixture combat modes: %#v", contracts.CombatModes)
	}
}

func TestDBRuntimeContractsDoNotInventMissingFallbacks(t *testing.T) {
	contracts := RuntimeContracts{Source: runtimeContractSourceDB}

	if got := contracts.contractForAbility("dodge"); got.ID != "" {
		t.Fatalf("strict DB contract invented ability fallback: %#v", got)
	}
	if got := contracts.skillContract("player_shield_rush"); got.MovementAction.ID != "" || got.Enabled {
		t.Fatalf("strict DB contract invented skill fallback: %#v", got)
	}
}

func TestRuntimeContractCoverageReportClassifiesMissingContracts(t *testing.T) {
	contracts := RuntimeContracts{Source: runtimeContractSourceDB}
	report := contracts.CoverageReport(true)
	if report.Ready {
		t.Fatal("empty DB contracts reported ready")
	}
	if !coverageReportHasBlocker(report, "runtime_movement_profile", "movement reconciliation profile") {
		t.Fatalf("movement profile blocker missing: %#v", report)
	}
	if !coverageReportHasBlocker(report, "skill_runtime_contracts", "skill runtime player_shield_rush") {
		t.Fatalf("skill blocker missing: %#v", report)
	}
	if !coverageReportHasBlocker(report, "combat_mode_slots", "sword shield combat mode slots") {
		t.Fatalf("combat mode blocker missing: %#v", report)
	}
}

func TestRuntimeContractCoverageReportAcceptsFixtureFixture(t *testing.T) {
	report := DevFixtureRuntimeContracts().CoverageReport(false)
	if !report.Ready {
		t.Fatalf("fixture fixture coverage report is not ready: %#v", report.Blockers())
	}
	for _, category := range []string{
		"runtime_movement_profile",
		"base_movement_actions",
		"skill_runtime_contracts",
		"wolf_brain_policy",
		"combat_core_profiles",
		"combat_defense_contracts",
		"combat_mode_slots",
		"compat_runtime_surfaces",
	} {
		if !coverageReportHasCategory(report, category) {
			t.Fatalf("coverage report missing category %q: %#v", category, report.Categories)
		}
	}
}

func TestRuntimeContractSourceDoesNotExposeLegacySkillMovementEffect(t *testing.T) {
	sourceType := reflect.TypeOf((*ContractSource)(nil)).Elem()
	if _, ok := sourceType.MethodByName("GetSkillMovementEffect"); ok {
		t.Fatal("normal runtime contract loader must not consume compatibility GetSkillMovementEffect")
	}
	for _, required := range []string{
		"GetSkillMovementActionBinding",
		"GetSkillActionTiming",
		"GetSkillHitboxProfiles",
		"GetWeaponCombatModeSlots",
	} {
		if _, ok := sourceType.MethodByName(required); !ok {
			t.Fatalf("normal runtime contract loader missing canonical method %s", required)
		}
	}
}

func TestLegacyRuntimeSurfaceClassificationKeepsCompatOutOfAuthority(t *testing.T) {
	var foundLegacy bool
	for _, surface := range runtimeContractSurfaces() {
		if surface.NormalRuntimeAuthority && surface.Status != contractSurfaceFinalAuthority {
			t.Fatalf("non-final surface became normal runtime authority: %#v", surface)
		}
		if surface.Name == "skill_movement_effect/GetSkillMovementEffect" {
			foundLegacy = true
			if surface.Status != contractSurfaceCompatRuntimeRequired {
				t.Fatalf("skill movement compatibility status = %q", surface.Status)
			}
			if surface.NormalRuntimeAuthority {
				t.Fatalf("skill movement compatibility endpoint must not be runtime authority: %#v", surface)
			}
			if surface.CanonicalReplacement != "skill_movement_action_binding + movement_action_contract" {
				t.Fatalf("skill movement compatibility canonical replacement = %q", surface.CanonicalReplacement)
			}
		}
	}
	if !foundLegacy {
		t.Fatal("skill movement compatibility endpoint is missing from the runtime surface audit")
	}
	if blockers := compatRuntimeSurfaceBlockers(); len(blockers) != 0 {
		t.Fatalf("compat surface blockers = %#v", blockers)
	}
}

func TestRuntimeReadinessReportsMissingContracts(t *testing.T) {
	runtime := NewRuntimeWithContracts(RuntimeContracts{Source: runtimeContractSourceDB})
	resp, err := runtime.Readiness(context.Background(), &gamev1.Empty{})
	if err != nil {
		t.Fatalf("Readiness failed: %v", err)
	}
	if resp.GetReady() {
		t.Fatal("runtime without DB contracts reported ready")
	}
	if len(resp.GetBlockers()) == 0 {
		t.Fatalf("readiness blockers missing: %#v", resp)
	}
}

func coverageReportHasCategory(report RuntimeContractCoverageReport, name string) bool {
	for _, category := range report.Categories {
		if category.Name == name {
			return true
		}
	}
	return false
}

func coverageReportHasBlocker(report RuntimeContractCoverageReport, categoryName string, blocker string) bool {
	for _, category := range report.Categories {
		if category.Name != categoryName {
			continue
		}
		for _, got := range category.Blockers {
			if got == blocker {
				return true
			}
		}
	}
	return false
}

func stringSliceContains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func TestRuntimeReadinessAcceptsFixtureFixtureForDevTests(t *testing.T) {
	runtime := NewRuntimeWithContracts(DevFixtureRuntimeContracts())
	resp, err := runtime.Readiness(context.Background(), &gamev1.Empty{})
	if err != nil {
		t.Fatalf("Readiness failed: %v", err)
	}
	if !resp.GetReady() {
		t.Fatalf("fixture fixture should be ready for dev/test runtime: %#v", resp.GetBlockers())
	}
}

func TestRuntimeStatsExposeContractCoverageByCategory(t *testing.T) {
	runtime := NewRuntimeWithContracts(RuntimeContracts{Source: runtimeContractSourceDB})
	resp, err := runtime.RuntimeStats(context.Background(), &gamev1.Empty{})
	if err != nil {
		t.Fatalf("RuntimeStats failed: %v", err)
	}
	if resp.GetPhaseStatus()["contract_source"] != runtimeContractSourceDB {
		t.Fatalf("contract source status missing: %#v", resp.GetPhaseStatus())
	}
	if resp.GetPhaseStatus()["contracts.runtime_movement_profile"] != "blocked" {
		t.Fatalf("movement coverage status = %#v", resp.GetPhaseStatus())
	}

	fixtureRuntime := NewRuntimeWithContracts(DevFixtureRuntimeContracts())
	fixtureResp, err := fixtureRuntime.RuntimeStats(context.Background(), &gamev1.Empty{})
	if err != nil {
		t.Fatalf("fixture RuntimeStats failed: %v", err)
	}
	if fixtureResp.GetPhaseStatus()["contracts.skill_runtime_contracts"] != "ready" {
		t.Fatalf("fixture skill contract coverage status = %#v", fixtureResp.GetPhaseStatus())
	}
	if fixtureResp.GetPhaseStatus()["contracts.compat_runtime_surfaces"] != "ready" {
		t.Fatalf("fixture compat surface coverage status = %#v", fixtureResp.GetPhaseStatus())
	}
	if got := fixtureResp.GetPhaseStatus()["contracts.surface.skill_movement_effect/GetSkillMovementEffect"]; !strings.Contains(got, "compat_runtime_required") || !strings.Contains(got, "compat_api") {
		t.Fatalf("skill movement compatibility surface status = %q", got)
	}
	if got := fixtureResp.GetPhaseStatus()["contracts.surface.movement_action_contract"]; !strings.Contains(got, "final_authority") || !strings.Contains(got, "runtime_authority") {
		t.Fatalf("movement action surface status = %q", got)
	}
	if got := fixtureResp.GetPhaseStatus()["contracts.required.skill.player_shield_rush"]; !strings.HasPrefix(got, "ready:") {
		t.Fatalf("required skill status = %q", got)
	}
}

func TestWolfMaulPublishesSkillMovementContractOnlyDuringRootMotion(t *testing.T) {
	runtime := NewRuntimeWithContracts(DevFixtureRuntimeContracts())
	player := runtime.ensurePlayerLocked("local_player")
	wolf := runtime.ensureWolfLocked(player)

	wolf.position = vector{x: player.position.x + 120, y: player.position.y, z: player.position.z}
	runtime.tick = 150
	runtime.updateWolfPolicyLocked(wolf, player)

	if wolf.creatureAI.GetSelectedSkillId() != "maul" {
		t.Fatalf("selected skill = %q, want maul", wolf.creatureAI.GetSelectedSkillId())
	}
	if wolf.creatureAI.GetSkillMovementType() != "" {
		t.Fatalf("maul windup leaked skill movement type = %q", wolf.creatureAI.GetSkillMovementType())
	}
	if wolf.actionInstance == nil || wolf.skillRuntime == nil {
		t.Fatal("maul did not start action runtime")
	}
	contract := runtime.contracts.skillContract("maul")
	activeElapsed := durationFromMS(contract.WindupMS) + 80*time.Millisecond
	startedAt := time.Now().Add(-activeElapsed)
	wolf.actionInstance.StartedAt = startedAt
	wolf.skillRuntime.StartedAtMs = startedAt.UnixMilli()
	wolf.actionMotion = nil

	runtime.tick = 151
	runtime.updateWolfPolicyLocked(wolf, player)

	if wolf.creatureAI.GetSkillMovementType() != "grounded_skill" {
		t.Fatalf("maul movement type = %q", wolf.creatureAI.GetSkillMovementType())
	}
	if wolf.creatureAI.GetSkillMovementDistanceCm() != 420 {
		t.Fatalf("maul movement distance = %v", wolf.creatureAI.GetSkillMovementDistanceCm())
	}
	if wolf.creatureAI.GetSkillActionLockMs() != 960 {
		t.Fatalf("maul action lock = %d", wolf.creatureAI.GetSkillActionLockMs())
	}
}

func TestWolfBrainDoesNotRepeatSkillWhileCooldownIsActive(t *testing.T) {
	runtime := NewRuntimeWithContracts(DevFixtureRuntimeContracts())
	player := runtime.ensurePlayerLocked("local_player")
	wolf := runtime.ensureWolfLocked(player)

	wolf.position = vector{x: player.position.x + 520, y: player.position.y, z: player.position.z}
	runtime.updateWolfPolicyLocked(wolf, player)
	if wolf.creatureAI.GetSelectedSkillId() != "lunge" {
		t.Fatalf("first selected skill = %q, want lunge", wolf.creatureAI.GetSelectedSkillId())
	}
	if len(wolf.creatureCooldownUntil) == 0 {
		t.Fatal("lunge did not start a creature cooldown")
	}

	wolf.skillRuntime = &gamev1.SkillRuntimeState{State: "idle"}
	runtime.updateWolfPolicyLocked(wolf, player)
	if wolf.creatureAI.GetSelectedSkillId() == "lunge" {
		t.Fatalf("cooldown skill repeated immediately: %#v", wolf.creatureAI)
	}
}

func TestWolfBrainUsesStaminaBudgetBeforeSelectingSkill(t *testing.T) {
	runtime := NewRuntimeWithContracts(DevFixtureRuntimeContracts())
	player := runtime.ensurePlayerLocked("local_player")
	wolf := runtime.ensureWolfLocked(player)

	wolf.position = vector{x: player.position.x + 120, y: player.position.y, z: player.position.z}
	wolf.stamina = 4
	runtime.tick = 220
	runtime.updateWolfPolicyLocked(wolf, player)

	if wolf.creatureAI.GetSelectedSkillId() != "wolf_dodge" {
		t.Fatalf("selected skill = %q, want affordable wolf_dodge instead of maul", wolf.creatureAI.GetSelectedSkillId())
	}
	if math.Abs(wolf.stamina-1.4) > 0.001 {
		t.Fatalf("wolf stamina after dodge = %.1f, want 1.4", wolf.stamina)
	}
}

func TestWolfBrainSpendsSkillStaminaOnlyWhenStartingSkill(t *testing.T) {
	runtime := NewRuntimeWithContracts(DevFixtureRuntimeContracts())
	player := runtime.ensurePlayerLocked("local_player")
	wolf := runtime.ensureWolfLocked(player)

	wolf.position = vector{x: player.position.x + 520, y: player.position.y, z: player.position.z}
	wolf.stamina = 40
	runtime.tick = 250
	runtime.updateWolfPolicyLocked(wolf, player)

	if wolf.creatureAI.GetSelectedSkillId() != "lunge" {
		t.Fatalf("selected skill = %q, want lunge", wolf.creatureAI.GetSelectedSkillId())
	}
	if math.Abs(wolf.stamina-16.4) > 0.001 {
		t.Fatalf("wolf stamina after starting lunge = %.1f, want 16.4", wolf.stamina)
	}

	runtime.tick = 251
	runtime.updateWolfPolicyLocked(wolf, player)
	if math.Abs(wolf.stamina-16.8) > 0.001 {
		t.Fatalf("active lunge should only regenerate, got stamina %.1f", wolf.stamina)
	}
}

func TestMovementValidationRuntimeDoesNotSpawnCreature(t *testing.T) {
	runtime := NewRuntimeWithOptions(DevFixtureRuntimeContracts(), RuntimeOptions{MovementValidation: true})
	player := runtime.ensurePlayerLocked("local_player")
	if player == nil {
		t.Fatal("player was not created")
	}
	if runtime.spawnedCreatureCountLocked() != 0 {
		t.Fatalf("spawned creature count = %d, want 0", runtime.spawnedCreatureCountLocked())
	}

	runtime.updateCreaturePoliciesLocked()
	if runtime.spawnedCreatureCountLocked() != 0 {
		t.Fatalf("movement validation runtime spawned creature after policy update: %d", runtime.spawnedCreatureCountLocked())
	}
}

type fakeRuntimeContractSource struct {
	missingActions            map[string]bool
	missingSkills             map[string]bool
	missingCombatCoreProfiles map[string]bool
	missingDefenseContracts   map[string]bool
	missingCombatModes        bool
}

func (f fakeRuntimeContractSource) GetCombatCoreProfile(_ context.Context, req *dbv1.IdRequest, _ ...grpc.CallOption) (*dbv1.CombatCoreProfileResponse, error) {
	if f.missingCombatCoreProfiles[req.GetId()] {
		return &dbv1.CombatCoreProfileResponse{Found: false}, nil
	}
	switch req.GetId() {
	case playerCombatCoreProfileID:
		return &dbv1.CombatCoreProfileResponse{Found: true, Profile: &dbv1.CombatCoreProfile{
			DamageDealtMultiplier:   1,
			DamageTakenMultiplier:   1,
			CanBlock:                true,
			BlockDamageReduction:    1,
			MaxPosture:              100,
			PostureDamageMultiplier: 1,
			PostureBreakDurationMs:  2200,
			CanParry:                true,
			ParryWindowMs:           220,
			ParryRewardMultiplier:   1.4,
			DodgeIframeMs:           320,
		}}, nil
	case creatureCombatCoreProfileID:
		return &dbv1.CombatCoreProfileResponse{Found: true, Profile: &dbv1.CombatCoreProfile{
			DamageDealtMultiplier:   0.95,
			DamageTakenMultiplier:   1.05,
			CanBlock:                false,
			BlockDamageReduction:    0,
			MaxPosture:              65,
			PostureDamageMultiplier: 1.15,
			PostureBreakDurationMs:  1800,
			CanParry:                false,
			ParryWindowMs:           0,
			ParryRewardMultiplier:   1,
			DodgeIframeMs:           220,
		}}, nil
	default:
		return &dbv1.CombatCoreProfileResponse{Found: false}, nil
	}
}

func (f fakeRuntimeContractSource) GetCombatDefenseContract(_ context.Context, req *dbv1.IdRequest, _ ...grpc.CallOption) (*dbv1.CombatDefenseContractResponse, error) {
	if f.missingDefenseContracts[req.GetId()] {
		return &dbv1.CombatDefenseContractResponse{Found: false}, nil
	}
	switch req.GetId() {
	case playerGuardDefenseContractID:
		return &dbv1.CombatDefenseContractResponse{Found: true, Contract: &dbv1.CombatDefenseContract{
			Id:                         req.GetId(),
			Name:                       "Player Shield Guard",
			DefenseType:                "shield_block",
			FrontalArcDeg:              120,
			DefenderMarginLeftRatio:    0.30,
			DefenderMarginRightRatio:   0.30,
			StaminaDamageOnlyOnBlock:   true,
			HealthDamageOnUnblockedHit: true,
			PostureDamageOnBlock:       true,
			GuardDamageMultiplier:      1,
			BlockStaminaDrainPerSecond: 2,
		}}, nil
	case creatureGuardDefenseContractID:
		return &dbv1.CombatDefenseContractResponse{Found: true, Contract: &dbv1.CombatDefenseContract{
			Id:                         req.GetId(),
			Name:                       "Wolf Attack Vs Guard",
			DefenseType:                "incoming_melee",
			FrontalArcDeg:              120,
			DefenderMarginLeftRatio:    0.30,
			DefenderMarginRightRatio:   0.30,
			StaminaDamageOnlyOnBlock:   true,
			HealthDamageOnUnblockedHit: true,
			PostureDamageOnBlock:       true,
			GuardDamageMultiplier:      1,
		}}, nil
	default:
		return &dbv1.CombatDefenseContractResponse{Found: false}, nil
	}
}

func (f fakeRuntimeContractSource) GetSkill(_ context.Context, req *dbv1.IdRequest, _ ...grpc.CallOption) (*dbv1.SkillResponse, error) {
	if f.missingSkills[req.GetId()] {
		return &dbv1.SkillResponse{Found: false}, nil
	}
	return &dbv1.SkillResponse{
		Found: true,
		Skill: &dbv1.Skill{
			Id:            req.GetId(),
			StaminaCost:   fakeSkillStaminaCost(req.GetId()),
			BaseDamage:    fakeSkillDamage(req.GetId()),
			PostureDamage: fakeSkillPostureDamage(req.GetId()),
			MaxRange:      300,
			MaxTargets:    1,
			IsBlockable:   true,
		},
	}, nil
}

func (f fakeRuntimeContractSource) GetSkillImpactProfile(_ context.Context, req *dbv1.IdRequest, _ ...grpc.CallOption) (*dbv1.SkillImpactProfileResponse, error) {
	if f.missingSkills[req.GetId()] {
		return &dbv1.SkillImpactProfileResponse{Found: false}, nil
	}
	_, posture := fixturePlayerSkillDamage(req.GetId())
	if posture == 0 {
		_, posture = fixtureCreatureSkillDamage(req.GetId())
	}
	profile := fixtureSkillImpactProfile(req.GetId(), posture)
	if profile == nil {
		return &dbv1.SkillImpactProfileResponse{Found: false}, nil
	}
	return &dbv1.SkillImpactProfileResponse{Found: true, Profile: profile}, nil
}

func fakeSkillStaminaCost(skillID string) float64 {
	switch skillID {
	case "wolf_dodge":
		return 6
	case "bite":
		return 10
	case "maul":
		return 22
	case "lunge":
		return 24
	default:
		return 0
	}
}

func fakeSkillDamage(skillID string) float64 {
	if skillID == "wolf_dodge" {
		return 0
	}
	return 12
}

func fakeSkillPostureDamage(skillID string) float64 {
	if skillID == "wolf_dodge" {
		return 0
	}
	return 20
}

func (f fakeRuntimeContractSource) GetSkillHitboxProfiles(_ context.Context, req *dbv1.IdRequest, _ ...grpc.CallOption) (*dbv1.SkillHitboxProfilesResponse, error) {
	if f.missingSkills[req.GetId()] {
		return &dbv1.SkillHitboxProfilesResponse{Found: false}, nil
	}
	targetType := "enemy"
	maxTargets := int32(1)
	damageGroupID := "damage_group_" + req.GetId()
	return &dbv1.SkillHitboxProfilesResponse{
		Found: true,
		Profiles: []*dbv1.SkillHitboxProfile{{
			Id:            "hitbox_" + req.GetId(),
			SkillId:       req.GetId(),
			HitboxShape:   "temporal_sweep",
			HitboxStartMs: 0,
			HitboxEndMs:   160,
			Length:        300,
			Radius:        60,
			Angle:         90,
			TargetType:    &targetType,
			MaxTargets:    &maxTargets,
			Priority:      20,
			DamageGroupId: damageGroupID,
			MotionProfile: &dbv1.SkillHitboxMotionProfile{
				Id:            "motion_" + req.GetId(),
				Enabled:       true,
				MotionType:    "timeline_sweep",
				TimeBasis:     "hitbox_window_normalized",
				Interpolation: "linear",
				SweepShape:    "capsule_strip",
				DamageGroupId: damageGroupID,
				Samples: []*dbv1.SkillHitboxMotionSample{
					fixtureHitboxMotionSample(0, 0.00, 40, 0, 90, 90, 0, 120, 45, 90, 0, 0),
					fixtureHitboxMotionSample(1, 1.00, 120, 0, 90, 90, 0, 120, 45, 180, 0, 0),
				},
			},
		}},
	}, nil
}

func (f fakeRuntimeContractSource) GetSkillActionTiming(_ context.Context, req *dbv1.IdRequest, _ ...grpc.CallOption) (*dbv1.SkillActionTimingResponse, error) {
	if f.missingSkills[req.GetId()] {
		return &dbv1.SkillActionTimingResponse{Found: false}, nil
	}
	return &dbv1.SkillActionTimingResponse{
		Found: true,
		Contract: &dbv1.SkillActionTimingContract{
			SkillId:            req.GetId(),
			WindupMs:           10,
			ActiveMs:           120,
			RecoveryMs:         60,
			CooldownMs:         0,
			ComboWindowMs:      2000,
			MovementLockPolicy: "contract",
			QueuePolicy:        "queue_after_recovery",
			CancelPolicy:       "contract_cancel_windows",
		},
	}, nil
}

func (f fakeRuntimeContractSource) GetSkillMovementActionBinding(_ context.Context, req *dbv1.IdRequest, _ ...grpc.CallOption) (*dbv1.SkillMovementActionBindingResponse, error) {
	if f.missingSkills[req.GetId()] {
		return &dbv1.SkillMovementActionBindingResponse{Found: false}, nil
	}
	contractID := fakeSkillMovementActionContractID(req.GetId())
	actionType := "grounded_skill"
	reconciliation := "grounded_skill_action_reconciliation"
	switch req.GetId() {
	case "lunge":
		actionType = "leap"
		reconciliation = "leap_reconciliation"
	case "wolf_dodge":
		actionType = "dodge"
		reconciliation = "dodge_reconciliation"
	}
	return &dbv1.SkillMovementActionBindingResponse{
		Found: true,
		Binding: &dbv1.SkillMovementActionBinding{
			SkillId:                  req.GetId(),
			MovementActionContractId: contractID,
			StartsAtPhase:            "active",
			HandoffPolicy:            "explicit_recovery_handoff",
			NormalInputPolicy:        "blocked_during_owned_root",
			TargetPolicy:             "aim_direction",
			ContactPolicy:            fakeMovementActionContract(contractID, req.GetId(), actionType, reconciliation).GetContactPolicy(),
			IsEnabled:                true,
			MovementActionContract:   fakeMovementActionContract(contractID, req.GetId(), actionType, reconciliation),
		},
	}, nil
}

func fakeSkillMovementActionContractID(skillID string) string {
	switch skillID {
	case "player_basic_attack_1":
		return "basic_attack_1_forward_cut_v1"
	case "player_basic_attack_2":
		return "basic_attack_2_cross_cut_v1"
	case "player_basic_attack_3":
		return "basic_attack_3_shield_drive_v1"
	case "player_shield_bash":
		return "shield_bash_front_push_v1"
	case "player_shield_rush":
		return "shield_rush_front_contact_v1"
	case "bite":
		return "wolf_bite_melee_commit_v1"
	case "lunge":
		return "low_fast_lunge_v1"
	case "wolf_dodge":
		return "wolf_dodge_lateral_leap_v1"
	case "maul":
		return "wolf_maul_lateral_counter_v1"
	default:
		return "db_" + skillID + "_movement"
	}
}

func (f fakeRuntimeContractSource) GetWeaponCombatModeSlots(context.Context, *dbv1.IdRequest, ...grpc.CallOption) (*dbv1.WeaponCombatModeSlotsResponse, error) {
	if f.missingCombatModes {
		return &dbv1.WeaponCombatModeSlotsResponse{Found: false}, nil
	}
	return &dbv1.WeaponCombatModeSlotsResponse{
		Found: true,
		Slots: []*dbv1.WeaponCombatModeSlot{
			{CombatModeId: "mode_sword_shield_vanguard", InputSlot: "M1", IsBasicAttack: false, IsEnabled: false},
			{CombatModeId: "mode_sword_shield_vanguard", InputSlot: "Q", IsEnabled: false},
			{CombatModeId: "mode_sword_shield_vanguard", InputSlot: "R", IsEnabled: false},
			{CombatModeId: "mode_sword_shield_vanguard", InputSlot: "F", IsEnabled: false},
			{CombatModeId: "mode_sword_shield_bulwark", InputSlot: "M1", SkillId: "player_basic_attack_1", IsBasicAttack: true, IsEnabled: true},
			{CombatModeId: "mode_sword_shield_bulwark", InputSlot: "Q", IsEnabled: false},
			{CombatModeId: "mode_sword_shield_bulwark", InputSlot: "R", SkillId: "player_shield_bash", IsEnabled: true},
			{CombatModeId: "mode_sword_shield_bulwark", InputSlot: "F", SkillId: "player_shield_rush", IsEnabled: true},
		},
	}, nil
}

func (f fakeRuntimeContractSource) GetMovementActionContract(_ context.Context, req *dbv1.IdRequest, _ ...grpc.CallOption) (*dbv1.MovementActionContractResponse, error) {
	if f.missingActions[req.GetId()] {
		return &dbv1.MovementActionContractResponse{Found: false}, nil
	}
	actionType := "move"
	if req.GetId() == "turn_v1_rate_limited_contextual" {
		actionType = "turn"
	} else if req.GetId() == "dodge_v1_full_iframe" {
		actionType = "dodge"
	} else if req.GetId() == "jump_v1_authoritative_grounded_handoff" {
		actionType = "leap"
	} else if strings.HasPrefix(req.GetId(), "basic_attack_") || strings.HasPrefix(req.GetId(), "shield_") || strings.HasPrefix(req.GetId(), "wolf_bite_") || strings.HasPrefix(req.GetId(), "wolf_maul_") {
		actionType = "grounded_skill"
	} else if req.GetId() == "low_fast_lunge_v1" || strings.HasPrefix(req.GetId(), "wolf_lunge_") {
		actionType = "leap"
	}
	return &dbv1.MovementActionContractResponse{
		Found:    true,
		Contract: fakeMovementActionContract(req.GetId(), "", actionType, "grounded_move_reconciliation"),
	}, nil
}

func (fakeRuntimeContractSource) GetMovementReconciliationContract(context.Context, *dbv1.IdRequest, ...grpc.CallOption) (*dbv1.MovementReconciliationContractResponse, error) {
	return &dbv1.MovementReconciliationContractResponse{Found: false}, nil
}

func (fakeRuntimeContractSource) GetRuntimeMovementReconciliationProfile(_ context.Context, req *dbv1.IdRequest, _ ...grpc.CallOption) (*dbv1.RuntimeMovementReconciliationProfileResponse, error) {
	if req.GetId() != runtimeMovementReconciliationProfileID {
		return &dbv1.RuntimeMovementReconciliationProfileResponse{Found: false}, nil
	}
	return &dbv1.RuntimeMovementReconciliationProfileResponse{
		Found: true,
		Profile: &dbv1.RuntimeMovementReconciliationProfile{
			ProfileId:                         runtimeMovementReconciliationProfileID,
			MaxSpeed:                          470,
			SprintSpeedMultiplier:             1.45,
			Acceleration:                      2048,
			Deceleration:                      2048,
			GroundFriction:                    8,
			AirAcceleration:                   768,
			JumpHeight:                        180,
			JumpDurationMs:                    620,
			RotationRateYaw:                   720,
			GravityScale:                      1,
			BrakingFrictionFactor:             2,
			MaxSlopeDeg:                       45,
			StepHeight:                        45,
			BaseDeadzone:                      25,
			GroundedSpeedDeadzoneFactor:       0.08,
			GroundedSpeedDeadzoneMin:          35,
			GroundedSpeedDeadzoneMax:          90,
			GroundedTransitionDeadzoneMin:     34,
			MoveSustainDeadzone:               45,
			MoveSustainTransitionDeadzone:     65,
			AirborneDeadzone:                  120,
			LeapRecentDeadzone:                140,
			LeapAirborneSnapshotDeadzone:      165,
			LeapLandingDeadzoneFactor:         0.12,
			LeapLandingDeadzoneMin:            80,
			LeapLandingDeadzoneMax:            180,
			LeapLandingClampIgnoreDeadzone:    145,
			LeapLandingSoftSnapDeadzone:       145,
			DodgeRecentDeadzone:               90,
			DodgeActiveDeadzone:               90,
			DodgeExitDeadzoneFactor:           0.12,
			DodgeExitDeadzoneMin:              65,
			DodgeExitDeadzoneMax:              180,
			PostActionGroundedDeadzone:        55,
			CorrectionMaxStep:                 80,
			HardSnapDistance:                  1400,
			SevereDesyncDistance:              2200,
			VisualSmoothingMs:                 80,
			VisualSmoothingMaxDistance:        260,
			RemoteVisualInterpolationMs:       100,
			RemoteVisualMaxExtrapolationMs:    100,
			RemoteVisualHardSnapDistance:      600,
			DodgeCarryHandoffMs:               120,
			LeapLandingCorrectionGraceMs:      120,
			LeapGroundedCarryHandoffMs:        70,
			MovementTurnResubmitDotThreshold:  0.92,
			MovementTurnResubmitMinIntervalMs: 33,
			MovementSubmitIntervalMs:          33,
			SnapshotPollIntervalMs:            33,
			StrafeSpeedMultiplier:             0.92,
			BackpedalSpeedMultiplier:          0.50,
			StrafeSprintSpeedMultiplier:       0.75,
			BackpedalSprintSpeedMultiplier:    0.50,
		},
	}, nil
}

func (fakeRuntimeContractSource) GetCreatureBehaviorRuntimeContract(_ context.Context, req *dbv1.IdRequest, _ ...grpc.CallOption) (*dbv1.CreatureBehaviorRuntimeContractResponse, error) {
	if req.GetId() != "contract_wolf_pack_harasser_v1" {
		return &dbv1.CreatureBehaviorRuntimeContractResponse{Found: false}, nil
	}
	return &dbv1.CreatureBehaviorRuntimeContractResponse{
		Found: true,
		Contract: &dbv1.CreatureBehaviorRuntimeContract{
			Id:                        req.GetId(),
			CreatureTemplateId:        "steppe_wolf",
			RangePolicyJson:           `{"desiredRangeCm":560,"chaseRangeCm":860,"retreatRangeCm":340,"orbitSpeedCmS":150,"chaseSpeedCmS":310,"lungeSpeedCmS":380,"maulSpeedCmS":345,"retreatSpeedCmS":260}`,
			PressurePolicyJson:        `{"repeatSkillPenaltyMultiplier":0.35,"dodgeUnderPressure":true,"maulCounterUnderPressure":true,"maulCounterChance":0.30,"dodgeRetreatMultiplier":0.70,"globalDodgeMultiplier":0.85,"commitThreatWeight":0.28,"closingThreatWeight":0.18,"defensiveBiteWeight":0.14,"fleeingLungeWeight":0.20,"lowResourceRiskFloor":0.16,"dodgeCommittedThreatMultiplier":1.12,"vulnerableBiteMultiplier":1.16,"vulnerableMaulMultiplier":1.16,"tacticalDestinationDistanceCm":180}`,
			StaminaPolicyJson:         `{"max":100,"dodgeCostMultiplier":0.50,"regenPerSecond":12}`,
			TargetOpportunityPolicyId: "opportunity_wolf_harasser_v1",
			OrbitPolicyId:             "orbit_wolf_harasser_combat_walk_v1",
		},
	}, nil
}

func (fakeRuntimeContractSource) GetCreatureRuntimeData(_ context.Context, req *dbv1.IdRequest, _ ...grpc.CallOption) (*dbv1.CreatureRuntimeDataResponse, error) {
	if req.GetId() != "steppe_wolf" {
		return &dbv1.CreatureRuntimeDataResponse{Found: false}, nil
	}
	return &dbv1.CreatureRuntimeDataResponse{
		Found: true,
		Template: &dbv1.CreatureTemplate{
			Id:                    "steppe_wolf",
			Name:                  "Steppe Wolf",
			Archetype:             "beast",
			ImpactResponseProfile: "creature_flesh_blood_red",
		},
	}, nil
}

func (fakeRuntimeContractSource) GetCreatureTargetOpportunityPolicy(_ context.Context, req *dbv1.IdRequest, _ ...grpc.CallOption) (*dbv1.CreatureTargetOpportunityPolicyResponse, error) {
	if req.GetId() != "opportunity_wolf_harasser_v1" {
		return &dbv1.CreatureTargetOpportunityPolicyResponse{Found: false}, nil
	}
	return &dbv1.CreatureTargetOpportunityPolicyResponse{
		Found: true,
		Policy: &dbv1.CreatureTargetOpportunityPolicy{
			Id:                          req.GetId(),
			CommitAngleMaxDeg:           180,
			MinCommitDistanceCm:         120,
			MaxCommitDistanceCm:         760,
			ApproachMinDistanceCm:       260,
			ApproachMaxDistanceCm:       760,
			BiteRangeCm:                 230,
			LungeMinRangeCm:             180,
			LungeMaxRangeCm:             700,
			MaulPressureThreshold:       0.72,
			TargetMemoryMs:              5200,
			NoReadySkillMemoryPolicy:    "observe_only",
			CandidateCooldownVisibility: true,
			AllowBacksideCommit:         true,
		},
	}, nil
}

func (fakeRuntimeContractSource) GetCreatureOrbitPolicy(_ context.Context, req *dbv1.IdRequest, _ ...grpc.CallOption) (*dbv1.CreatureOrbitPolicyResponse, error) {
	if req.GetId() != "orbit_wolf_harasser_combat_walk_v1" {
		return &dbv1.CreatureOrbitPolicyResponse{Found: false}, nil
	}
	return &dbv1.CreatureOrbitPolicyResponse{
		Found: true,
		Policy: &dbv1.CreatureOrbitPolicy{
			Id:                             req.GetId(),
			BehaviorContractId:             "contract_wolf_pack_harasser_v1",
			OrbitLocomotionMode:            "combat_walk",
			OrbitSpeedScale:                0.75,
			MinOrbitDurationMs:             2600,
			SideSwitchCooldownMs:           2600,
			AllowSideSwitchWhenTargetFaces: true,
			PreferLongSideCommit:           true,
			SideFlipChanceMultiplier:       0.55,
			LockSideDuringSetup:            true,
		},
	}, nil
}

func (fakeRuntimeContractSource) GetCreatureEvasionPolicies(_ context.Context, req *dbv1.IdRequest, _ ...grpc.CallOption) (*dbv1.CreatureEvasionPoliciesResponse, error) {
	if req.GetId() != "contract_wolf_pack_harasser_v1" {
		return &dbv1.CreatureEvasionPoliciesResponse{Found: false}, nil
	}
	return &dbv1.CreatureEvasionPoliciesResponse{
		Found: true,
		Policies: []*dbv1.CreatureEvasionPolicy{
			{
				Id:                      "evasion_wolf_harasser_dodge_v1",
				BehaviorContractId:      req.GetId(),
				DodgeSkillId:            "wolf_dodge",
				MaxChainCount:           4,
				StaminaCostMultiplier:   0.5,
				RetreatChanceMultiplier: 0.7,
				LateralBias:             0.72,
				BackstepBias:            0.28,
				PressureThreshold:       0.42,
				CooldownMs:              260,
			},
		},
	}, nil
}

func (fakeRuntimeContractSource) GetCreatureSkillSetupPolicies(_ context.Context, req *dbv1.IdRequest, _ ...grpc.CallOption) (*dbv1.CreatureSkillSetupPoliciesResponse, error) {
	if req.GetId() != "contract_wolf_pack_harasser_v1" {
		return &dbv1.CreatureSkillSetupPoliciesResponse{Found: false}, nil
	}
	return &dbv1.CreatureSkillSetupPoliciesResponse{
		Found: true,
		Policies: []*dbv1.CreatureSkillSetupPolicy{
			{Id: "wolf_lunge_flank_windup_v1", BehaviorContractId: req.GetId(), SkillId: "lunge", SetupType: "moving_windup", MinSetupMs: 3000, MaxSetupMs: 4200, CommitDistanceCm: 520, PreferredMinRangeCm: 180, PreferredMaxRangeCm: 700, MovementTactic: "circle_then_curve_to_target", LockSideDuringSetup: true, IsEnabled: true},
			{Id: "wolf_lunge_chase_windup_v1", BehaviorContractId: req.GetId(), SkillId: "lunge", SetupType: "chase_windup", MinSetupMs: 1200, MaxSetupMs: 2600, CommitDistanceCm: 640, PreferredMinRangeCm: 520, PreferredMaxRangeCm: 1200, MovementTactic: "run_chase_then_jump", LockSideDuringSetup: false, IsEnabled: true},
			{Id: "wolf_maul_pressure_counter_v1", BehaviorContractId: req.GetId(), SkillId: "maul", SetupType: "pressure_counter", MinSetupMs: 160, MaxSetupMs: 420, CommitDistanceCm: 220, PreferredMinRangeCm: 0, PreferredMaxRangeCm: 260, MovementTactic: "lateral_counter_dash", LockSideDuringSetup: true, IsEnabled: true},
		},
	}, nil
}

func (fakeRuntimeContractSource) GetCreatureSkillBehaviorBindings(_ context.Context, req *dbv1.IdRequest, _ ...grpc.CallOption) (*dbv1.CreatureSkillBehaviorBindingsResponse, error) {
	if req.GetId() != "contract_wolf_pack_harasser_v1" {
		return &dbv1.CreatureSkillBehaviorBindingsResponse{Found: false}, nil
	}
	return &dbv1.CreatureSkillBehaviorBindingsResponse{
		Found: true,
		Bindings: []*dbv1.CreatureSkillBehaviorBinding{
			{Id: "bind_bite_approach", BehaviorContractId: req.GetId(), SkillId: "bite", TacticalState: "approach", DecisionPhase: "acquire", MinRangeCm: 0, MaxRangeCm: 260, Priority: 60, UsageWeight: 0.9, CooldownGroup: "wolf_bite", RequiresLineOfSight: true, IsEnabled: true},
			{Id: "bind_lunge_circle", BehaviorContractId: req.GetId(), SkillId: "lunge", TacticalState: "circle", DecisionPhase: "reposition", SetupPolicyId: "wolf_lunge_flank_windup_v1", MinRangeCm: 180, MaxRangeCm: 700, Priority: 90, UsageWeight: 1.1, CooldownGroup: "wolf_lunge", RequiresLineOfSight: true, IsEnabled: true},
			{Id: "bind_maul_pressure", BehaviorContractId: req.GetId(), SkillId: "maul", TacticalState: "pressure", DecisionPhase: "counter", MinRangeCm: 0, MaxRangeCm: 260, Priority: 100, UsageWeight: 0.7, CooldownGroup: "wolf_maul", RequiresLineOfSight: true, IsEnabled: true},
			{Id: "bind_dodge_pressure", BehaviorContractId: req.GetId(), SkillId: "wolf_dodge", TacticalState: "pressure", DecisionPhase: "evade", MinRangeCm: 0, MaxRangeCm: 420, Priority: 110, UsageWeight: 1.2, CooldownGroup: "wolf_dodge", RequiresLineOfSight: false, IsEnabled: true},
		},
	}, nil
}

func fakeMovementActionContract(id string, abilityKey string, actionType string, reconciliation string) *dbv1.MovementActionContract {
	metadata := "{}"
	if abilityKey != "" {
		metadata = `{"ability_key":"` + abilityKey + `"}`
	}
	durationMS := int32(240)
	activeMS := int32(160)
	recoveryMS := int32(80)
	distanceCM := float64(120)
	baseSpeedCMS := float64(600)
	contactPolicy := "authoritative_contact"
	switch id {
	case "dodge_v1_full_iframe":
		durationMS = 320
		activeMS = 260
		recoveryMS = 60
		distanceCM = 360
		baseSpeedCMS = 1125
		contactPolicy = "iframe"
	case "shield_rush_front_contact_v1":
		durationMS = 1100
		activeMS = 720
		recoveryMS = 260
		distanceCM = 864
		baseSpeedCMS = 1033.2
		contactPolicy = "multi_target_carry_push"
		metadata = `{"ability_key":"player_shield_rush","front_contact_offset_cm":8,"front_contact_depth_cm":54,"source":"test_db_contract"}`
	case "shield_bash_front_push_v1":
		durationMS = 300
		activeMS = 170
		recoveryMS = 120
		distanceCM = 95
		baseSpeedCMS = 541
		contactPolicy = "multi_target_push"
	case "basic_attack_1_forward_cut_v1":
		durationMS = 350
		activeMS = 140
		recoveryMS = 120
		distanceCM = 84
		baseSpeedCMS = 240
	case "basic_attack_2_cross_cut_v1":
		durationMS = 370
		activeMS = 150
		recoveryMS = 120
		distanceCM = 42
		baseSpeedCMS = 114
	case "basic_attack_3_shield_drive_v1":
		durationMS = 620
		activeMS = 260
		recoveryMS = 180
		distanceCM = 252
		baseSpeedCMS = 406.4
		contactPolicy = "carry_contact"
	case "wolf_bite_melee_commit_v1":
		durationMS = 520
		activeMS = 220
		recoveryMS = 180
		distanceCM = 0
		baseSpeedCMS = 0
		contactPolicy = "melee_contact"
	case "low_fast_lunge_v1":
		durationMS = 860
		activeMS = 380
		recoveryMS = 240
		distanceCM = 918
		baseSpeedCMS = 1310
		contactPolicy = "airborne_passthrough"
	case "wolf_dodge_lateral_leap_v1":
		durationMS = 520
		activeMS = 420
		recoveryMS = 100
		distanceCM = 210
		baseSpeedCMS = 520
		contactPolicy = "iframe"
	case "wolf_maul_lateral_counter_v1":
		durationMS = 920
		activeMS = 520
		recoveryMS = 220
		distanceCM = 420
		baseSpeedCMS = 690
		contactPolicy = "lateral_counter_contact"
	}
	return &dbv1.MovementActionContract{
		Id:                       id,
		ActionType:               actionType,
		DurationMs:               durationMS,
		ActiveMs:                 activeMS,
		RecoveryMs:               recoveryMS,
		DistanceCm:               distanceCM,
		BaseSpeedCmS:             baseSpeedCMS,
		PhaseWindowPolicy:        "server_authoritative",
		PredictionErrorPolicy:    "bounded_smooth_correction",
		ReconciliationContractId: reconciliation,
		RootMotionOwner:          "skill",
		ContactPolicy:            contactPolicy,
		MetadataJson:             metadata,
		ReconciliationContract: &dbv1.MovementReconciliationContract{
			Id:       reconciliation,
			Category: reconciliation,
		},
	}
}

func hasCombatModeSlot(slots []*gamev1.CombatModeSlot, combatModeID string, slotIndex uint32, skillID string) bool {
	for _, slot := range slots {
		if slot.GetCombatModeId() == combatModeID && slot.GetSlotIndex() == slotIndex && slot.GetSkillId() == skillID && slot.GetEnabled() {
			return true
		}
	}
	return false
}

func hasEmptyCombatModeSlot(slots []*gamev1.CombatModeSlot, combatModeID string, slotIndex uint32) bool {
	for _, slot := range slots {
		if slot.GetCombatModeId() == combatModeID && slot.GetSlotIndex() == slotIndex && slot.GetSkillId() == "" && !slot.GetEnabled() {
			return true
		}
	}
	return false
}

func hasCreatureSkillBehaviorBinding(bindings []CreatureSkillBehaviorRuntimeBinding, skillID string, tacticalState string, decisionPhase string) bool {
	for _, binding := range bindings {
		if binding.SkillID == skillID && binding.TacticalState == tacticalState && binding.DecisionPhase == decisionPhase && binding.Enabled {
			return true
		}
	}
	return false
}

func hasCreatureSkillSetupPolicy(policies []CreatureSkillSetupRuntimePolicy, id string, skillID string, movementTactic string) bool {
	for _, policy := range policies {
		if policy.ID == id && policy.SkillID == skillID && policy.MovementTactic == movementTactic && policy.Enabled {
			return true
		}
	}
	return false
}
