package gameapi

import (
	"context"
	"math"
	"strings"
	"testing"

	dbv1 "db-apeiron/gen/apeiron/v1"
	gamev1 "server-apeiron/gen/apeiron/game/v1"

	"google.golang.org/grpc"
)

func TestRecoveryFixtureRuntimeContractsExposeRequiredSkillContracts(t *testing.T) {
	contracts := RecoveryFixtureRuntimeContracts()
	if err := contracts.ValidateRequiredCoverage(false); err != nil {
		t.Fatalf("recovered contract coverage failed: %v", err)
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

func TestRecoveryFixtureRuntimeContractsExposeCreatureSkillContracts(t *testing.T) {
	contracts := RecoveryFixtureRuntimeContracts()
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
		t.Fatalf("recovered wolf lunge binding missing: %#v", contracts.WolfPolicy.SkillBehaviorBindings)
	}
	if !hasCreatureSkillBehaviorBinding(contracts.WolfPolicy.SkillBehaviorBindings, "maul", "pressure", "counter") {
		t.Fatalf("recovered wolf maul binding missing: %#v", contracts.WolfPolicy.SkillBehaviorBindings)
	}
}

func TestRecoveryFixtureCombatModesKeepBulwarkAndVanguardSeparate(t *testing.T) {
	contracts := RecoveryFixtureRuntimeContracts()
	if !hasCombatModeSlot(contracts.CombatModes, swordShieldBulwarkModeID, 0, "player_basic_attack_1") {
		t.Fatalf("recovered Bulwark M1 slot missing: %#v", contracts.CombatModes)
	}
	if !hasCombatModeSlot(contracts.CombatModes, swordShieldBulwarkModeID, 2, "player_shield_bash") {
		t.Fatalf("recovered Bulwark R slot missing: %#v", contracts.CombatModes)
	}
	if !hasCombatModeSlot(contracts.CombatModes, swordShieldBulwarkModeID, 3, "player_shield_rush") {
		t.Fatalf("recovered Bulwark F slot missing: %#v", contracts.CombatModes)
	}
	for _, slotIndex := range []uint32{0, 1, 2, 3, 4} {
		if !hasEmptyCombatModeSlot(contracts.CombatModes, swordShieldVanguardModeID, slotIndex) {
			t.Fatalf("recovered Vanguard slot %d should be empty/disabled: %#v", slotIndex, contracts.CombatModes)
		}
	}
}

func TestLoadRuntimeContractsFromDBUsesRequiredSkillBindings(t *testing.T) {
	source := fakeRuntimeContractSource{}
	contracts := LoadRuntimeContractsFromDB(context.Background(), source, source)
	if err := contracts.ValidateRequiredCoverage(true); err != nil {
		t.Fatalf("db contract coverage failed: %v", err)
	}
	if contracts.Source != "db_contracts" {
		t.Fatalf("source = %q, want db_contracts", contracts.Source)
	}
	if contracts.MovementProfile.GetProfileId() != runtimeMovementReconciliationProfileID {
		t.Fatalf("movement reconciliation profile = %q", contracts.MovementProfile.GetProfileId())
	}

	for _, skillID := range requiredRuntimeSkillIDs() {
		skill := contracts.skillContract(skillID)
		if skill.MovementActionContractID != "db_"+skillID+"_movement" {
			t.Fatalf("%s movement binding = %q", skillID, skill.MovementActionContractID)
		}
		if contracts.ActionContracts[skillID].ID != "db_"+skillID+"_movement" {
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
	if contracts.WolfPolicy.RepeatSkillPenaltyWindowMS != 1200 || contracts.WolfPolicy.RepeatSkillPenaltyMultiplier != 0.65 {
		t.Fatalf("wolf repeat policy = window %d multiplier %.2f", contracts.WolfPolicy.RepeatSkillPenaltyWindowMS, contracts.WolfPolicy.RepeatSkillPenaltyMultiplier)
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

func TestLoadRuntimeContractsFromDBDoesNotLeakRecoveredCombatFallback(t *testing.T) {
	source := fakeRuntimeContractSource{
		missingCombatCoreProfiles: map[string]bool{playerCombatCoreProfileID: true},
		missingDefenseContracts:   map[string]bool{playerGuardDefenseContractID: true},
	}
	contracts := LoadRuntimeContractsFromDB(context.Background(), source, source)

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
		t.Fatalf("DB loader leaked recovered player combat core fallback: %#v", got)
	}
	if got := contracts.Defense.Contracts[playerGuardDefenseContractID]; got != nil {
		t.Fatalf("DB loader leaked recovered player guard fallback: %#v", got)
	}
}

func TestLoadRuntimeContractsFromDBRejectsMissingRequiredBinding(t *testing.T) {
	source := fakeRuntimeContractSource{missingSkills: map[string]bool{"maul": true}}
	contracts := LoadRuntimeContractsFromDB(context.Background(), source, source)

	err := contracts.ValidateRequiredCoverage(true)
	if err == nil {
		t.Fatal("expected missing DB runtime contract to fail strict coverage")
	}
	if !strings.Contains(err.Error(), "missing skill runtime maul") {
		t.Fatalf("coverage error = %v", err)
	}
}

func TestLoadRuntimeContractsFromDBDoesNotLeakRecoveredAbilityFallback(t *testing.T) {
	source := fakeRuntimeContractSource{missingActions: map[string]bool{"dodge_v1_full_iframe": true}}
	contracts := LoadRuntimeContractsFromDB(context.Background(), source, source)

	err := contracts.ValidateRequiredCoverage(true)
	if err == nil {
		t.Fatal("expected missing DB movement action to fail strict coverage")
	}
	if !strings.Contains(err.Error(), "missing movement action dodge -> dodge_v1_full_iframe") {
		t.Fatalf("coverage error = %v", err)
	}
	if got := contracts.ActionContracts["dodge"]; got.ID != "" {
		t.Fatalf("DB loader leaked recovered dodge fallback: %#v", got)
	}
}

func TestLoadRuntimeContractsFromDBDoesNotLeakRecoveredCombatModeFallback(t *testing.T) {
	source := fakeRuntimeContractSource{missingCombatModes: true}
	contracts := LoadRuntimeContractsFromDB(context.Background(), source, source)

	err := contracts.ValidateRequiredCoverage(true)
	if err == nil {
		t.Fatal("expected missing DB combat mode slots to fail strict coverage")
	}
	if !strings.Contains(err.Error(), "missing weapon combat mode slots weaponkit_sword_shield") {
		t.Fatalf("coverage error = %v", err)
	}
	if len(contracts.CombatModes) != 0 {
		t.Fatalf("DB loader leaked recovered combat modes: %#v", contracts.CombatModes)
	}
}

func TestDBRuntimeContractsDoNotInventMissingFallbacks(t *testing.T) {
	contracts := RuntimeContracts{Source: "db_contracts"}

	if got := contracts.contractForAbility("dodge"); got.ID != "" {
		t.Fatalf("strict DB contract invented ability fallback: %#v", got)
	}
	if got := contracts.skillContract("player_shield_rush"); got.MovementAction.ID != "" || got.Enabled {
		t.Fatalf("strict DB contract invented skill fallback: %#v", got)
	}
}

func TestWolfMaulPublishesSelectedSkillMovementContract(t *testing.T) {
	runtime := NewRuntimeWithContracts(RecoveryFixtureRuntimeContracts())
	player := runtime.ensurePlayerLocked("local_player")
	wolf := runtime.ensureWolfLocked(player)

	wolf.position = vector{x: player.position.x + 120, y: player.position.y, z: player.position.z}
	runtime.tick = 150
	runtime.updateWolfPolicyLocked(wolf, player)

	if wolf.creatureAI.GetSelectedSkillId() != "maul" {
		t.Fatalf("selected skill = %q, want maul", wolf.creatureAI.GetSelectedSkillId())
	}
	if wolf.creatureAI.GetSkillMovementType() != "grounded_skill" {
		t.Fatalf("maul movement type = %q", wolf.creatureAI.GetSkillMovementType())
	}
	if wolf.creatureAI.GetSkillMovementDistanceCm() != 140 {
		t.Fatalf("maul movement distance = %v", wolf.creatureAI.GetSkillMovementDistanceCm())
	}
	if wolf.creatureAI.GetSkillActionLockMs() != 800 {
		t.Fatalf("maul action lock = %d", wolf.creatureAI.GetSkillActionLockMs())
	}
}

func TestWolfBrainDoesNotRepeatSkillWhileCooldownIsActive(t *testing.T) {
	runtime := NewRuntimeWithContracts(RecoveryFixtureRuntimeContracts())
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
	runtime := NewRuntimeWithContracts(RecoveryFixtureRuntimeContracts())
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
	runtime := NewRuntimeWithContracts(RecoveryFixtureRuntimeContracts())
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
	runtime := NewRuntimeWithOptions(RecoveryFixtureRuntimeContracts(), RuntimeOptions{MovementValidation: true})
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
			BaseDamage:    12,
			PostureDamage: 20,
			MaxRange:      300,
			MaxTargets:    1,
			IsBlockable:   true,
		},
	}, nil
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

func (f fakeRuntimeContractSource) GetSkillHitboxProfiles(_ context.Context, req *dbv1.IdRequest, _ ...grpc.CallOption) (*dbv1.SkillHitboxProfilesResponse, error) {
	if f.missingSkills[req.GetId()] {
		return &dbv1.SkillHitboxProfilesResponse{Found: false}, nil
	}
	targetType := "enemy"
	maxTargets := int32(1)
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
	contractID := "db_" + req.GetId() + "_movement"
	return &dbv1.SkillMovementActionBindingResponse{
		Found: true,
		Binding: &dbv1.SkillMovementActionBinding{
			SkillId:                  req.GetId(),
			MovementActionContractId: contractID,
			StartsAtPhase:            "active",
			HandoffPolicy:            "explicit_recovery_handoff",
			NormalInputPolicy:        "blocked_during_owned_root",
			TargetPolicy:             "aim_direction",
			ContactPolicy:            "authoritative_contact",
			IsEnabled:                true,
			MovementActionContract:   fakeMovementActionContract(contractID, req.GetId(), "grounded_skill", "grounded_skill_action_reconciliation"),
		},
	}, nil
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
			BackpedalSpeedMultiplier:          0.65,
			StrafeSprintSpeedMultiplier:       0.75,
			BackpedalSprintSpeedMultiplier:    0.75,
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
			PressurePolicyJson:        `{"repeatSkillPenaltyMultiplier":0.65}`,
			StaminaPolicyJson:         `{"max":100,"dodgeCostMultiplier":0.50,"regenPerSecond":12}`,
			TargetOpportunityPolicyId: "opportunity_wolf_harasser_v1",
			OrbitPolicyId:             "orbit_wolf_harasser_combat_walk_v1",
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
			TargetMemoryMs:              1200,
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
			OrbitSpeedScale:                0.55,
			MinOrbitDurationMs:             700,
			SideSwitchCooldownMs:           900,
			AllowSideSwitchWhenTargetFaces: true,
			PreferLongSideCommit:           true,
			SideFlipChanceMultiplier:       0.35,
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
				LateralBias:             0.8,
				BackstepBias:            0.2,
				PressureThreshold:       0.55,
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
			{Id: "wolf_maul_pressure_counter_v1", BehaviorContractId: req.GetId(), SkillId: "maul", SetupType: "pressure_counter", MinSetupMs: 120, MaxSetupMs: 320, CommitDistanceCm: 160, PreferredMinRangeCm: 0, PreferredMaxRangeCm: 220, MovementTactic: "lateral_counter_dash", LockSideDuringSetup: true, IsEnabled: true},
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
			{Id: "bind_lunge_circle", BehaviorContractId: req.GetId(), SkillId: "lunge", TacticalState: "circle", DecisionPhase: "reposition", SetupPolicyId: "setup_wolf_lunge_orbit_windup_v1", MinRangeCm: 180, MaxRangeCm: 700, Priority: 90, UsageWeight: 1.1, CooldownGroup: "wolf_lunge", RequiresLineOfSight: true, IsEnabled: true},
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
	return &dbv1.MovementActionContract{
		Id:                       id,
		ActionType:               actionType,
		DurationMs:               240,
		ActiveMs:                 160,
		RecoveryMs:               80,
		DistanceCm:               120,
		BaseSpeedCmS:             600,
		PhaseWindowPolicy:        "server_authoritative",
		PredictionErrorPolicy:    "bounded_smooth_correction",
		ReconciliationContractId: reconciliation,
		RootMotionOwner:          "skill",
		ContactPolicy:            "authoritative_contact",
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
