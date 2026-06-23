package gameapi

import (
	"context"
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
	if !hasCombatModeSlot(contracts.CombatModes, "mode_sword_shield_bulwark", 3, "player_shield_rush") {
		t.Fatalf("DB combat mode slots did not load Bulwark F -> Shield Rush: %#v", contracts.CombatModes)
	}
	if hasCombatModeSlot(contracts.CombatModes, "mode_sword_shield_vanguard", 2, "player_shield_bash") {
		t.Fatalf("Vanguard must not inherit Bulwark R skill from fallback")
	}
	if hasCombatModeSlot(contracts.CombatModes, "mode_sword_shield_vanguard", 1, "player_basic_attack_1") {
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
	if contracts.WolfPolicy.LungeMinRangeCM != 180 || contracts.WolfPolicy.LungeMaxRangeCM != 700 {
		t.Fatalf("wolf lunge range = %.0f..%.0f", contracts.WolfPolicy.LungeMinRangeCM, contracts.WolfPolicy.LungeMaxRangeCM)
	}
	if !hasCreatureSkillBehaviorBinding(contracts.WolfPolicy.SkillBehaviorBindings, "lunge", "circle", "reposition") {
		t.Fatalf("wolf lunge circle/reposition binding missing: %#v", contracts.WolfPolicy.SkillBehaviorBindings)
	}
	if !hasCreatureSkillBehaviorBinding(contracts.WolfPolicy.SkillBehaviorBindings, "maul", "pressure", "counter") {
		t.Fatalf("wolf maul pressure/counter binding missing: %#v", contracts.WolfPolicy.SkillBehaviorBindings)
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
	missingActions     map[string]bool
	missingSkills      map[string]bool
	missingCombatModes bool
}

func (f fakeRuntimeContractSource) GetSkill(_ context.Context, req *dbv1.IdRequest, _ ...grpc.CallOption) (*dbv1.SkillResponse, error) {
	if f.missingSkills[req.GetId()] {
		return &dbv1.SkillResponse{Found: false}, nil
	}
	return &dbv1.SkillResponse{
		Found: true,
		Skill: &dbv1.Skill{
			Id:            req.GetId(),
			BaseDamage:    12,
			PostureDamage: 20,
			MaxRange:      300,
			MaxTargets:    1,
			IsBlockable:   true,
		},
	}, nil
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
			MoveSustainDeadzone:               45,
			MoveSustainTransitionDeadzone:     65,
			AirborneDeadzone:                  120,
			LeapRecentDeadzone:                140,
			LeapAirborneSnapshotDeadzone:      165,
			LeapLandingDeadzoneFactor:         0.12,
			LeapLandingDeadzoneMin:            80,
			LeapLandingDeadzoneMax:            180,
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

func (fakeRuntimeContractSource) GetCreatureSkillSetupPolicies(context.Context, *dbv1.IdRequest, ...grpc.CallOption) (*dbv1.CreatureSkillSetupPoliciesResponse, error) {
	return &dbv1.CreatureSkillSetupPoliciesResponse{Found: false}, nil
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

func hasCreatureSkillBehaviorBinding(bindings []CreatureSkillBehaviorRuntimeBinding, skillID string, tacticalState string, decisionPhase string) bool {
	for _, binding := range bindings {
		if binding.SkillID == skillID && binding.TacticalState == tacticalState && binding.DecisionPhase == decisionPhase && binding.Enabled {
			return true
		}
	}
	return false
}
