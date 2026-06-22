package gameapi

import "testing"

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
