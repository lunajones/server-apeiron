package gameapi

import (
	"context"
	"testing"

	dbv1 "db-apeiron/gen/apeiron/v1"

	"google.golang.org/grpc"
)

func TestRecoveredRuntimeContractsExposeRequiredSkillContracts(t *testing.T) {
	contracts := RecoveredRuntimeContracts()
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

func TestRecoveredRuntimeContractsExposeCreatureSkillContracts(t *testing.T) {
	contracts := RecoveredRuntimeContracts()
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
}

func TestLoadRuntimeContractsFromDBUsesRequiredSkillBindings(t *testing.T) {
	source := fakeRuntimeContractSource{}
	contracts := LoadRuntimeContractsFromDB(context.Background(), source, source)

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
}

func TestWolfMaulPublishesSelectedSkillMovementContract(t *testing.T) {
	runtime := NewRuntimeWithContracts(RecoveredRuntimeContracts())
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

func TestMovementValidationRuntimeDoesNotSpawnCreature(t *testing.T) {
	runtime := NewRuntimeWithOptions(RecoveredRuntimeContracts(), RuntimeOptions{MovementValidation: true})
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

type fakeRuntimeContractSource struct{}

func (fakeRuntimeContractSource) GetSkillActionTiming(_ context.Context, req *dbv1.IdRequest, _ ...grpc.CallOption) (*dbv1.SkillActionTimingResponse, error) {
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

func (fakeRuntimeContractSource) GetSkillMovementActionBinding(_ context.Context, req *dbv1.IdRequest, _ ...grpc.CallOption) (*dbv1.SkillMovementActionBindingResponse, error) {
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

func (fakeRuntimeContractSource) GetMovementActionContract(_ context.Context, req *dbv1.IdRequest, _ ...grpc.CallOption) (*dbv1.MovementActionContractResponse, error) {
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

func (fakeRuntimeContractSource) GetCreatureBehaviorRuntimeContract(context.Context, *dbv1.IdRequest, ...grpc.CallOption) (*dbv1.CreatureBehaviorRuntimeContractResponse, error) {
	return &dbv1.CreatureBehaviorRuntimeContractResponse{Found: false}, nil
}

func (fakeRuntimeContractSource) GetCreatureEvasionPolicies(context.Context, *dbv1.IdRequest, ...grpc.CallOption) (*dbv1.CreatureEvasionPoliciesResponse, error) {
	return &dbv1.CreatureEvasionPoliciesResponse{Found: false}, nil
}

func (fakeRuntimeContractSource) GetCreatureSkillSetupPolicies(context.Context, *dbv1.IdRequest, ...grpc.CallOption) (*dbv1.CreatureSkillSetupPoliciesResponse, error) {
	return &dbv1.CreatureSkillSetupPoliciesResponse{Found: false}, nil
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
