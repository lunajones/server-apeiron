package gameapi

import (
	"context"
	"testing"
)

func TestRuntimeSkillImpactUsesCombatPipelineParry(t *testing.T) {
	t.Parallel()

	runtime := NewRuntimeWithContracts(RecoveryFixtureRuntimeContracts())
	sessionID := "runtime-skill-impact-parry"
	attachRuntimePlayer(t, runtime, sessionID)
	player := runtime.ensurePlayerLocked("local_player")
	wolf := runtime.ensureWolfLocked(player)
	wolf.position = vector{x: player.position.x + 120, y: player.position.y, z: player.position.z}
	wolf.yaw = 180
	wolf.combatState = "parry"
	beforeHealth := wolf.health
	beforePosture := wolf.posture

	if _, err := runtime.SubmitCommand(context.Background(), testRuntimeCastSkillCommand(sessionID, 1, "player_basic_attack_1", gamev1Vector(1, 0, 0))); err != nil {
		t.Fatalf("SubmitCommand failed: %v", err)
	}
	if wolf.health != beforeHealth {
		t.Fatalf("parried hit changed health: got %.1f want %.1f", wolf.health, beforeHealth)
	}
	if wolf.posture != beforePosture {
		t.Fatalf("parried hit changed posture: got %.1f want %.1f", wolf.posture, beforePosture)
	}
}

func TestRuntimeSkillImpactUsesCombatPipelineIframe(t *testing.T) {
	t.Parallel()

	runtime := NewRuntimeWithContracts(RecoveryFixtureRuntimeContracts())
	sessionID := "runtime-skill-impact-iframe"
	attachRuntimePlayer(t, runtime, sessionID)
	player := runtime.ensurePlayerLocked("local_player")
	wolf := runtime.ensureWolfLocked(player)
	wolf.position = vector{x: player.position.x + 120, y: player.position.y, z: player.position.z}
	wolf.combatState = "iframe"
	beforeHealth := wolf.health
	beforePosture := wolf.posture

	if _, err := runtime.SubmitCommand(context.Background(), testRuntimeCastSkillCommand(sessionID, 1, "player_basic_attack_1", gamev1Vector(1, 0, 0))); err != nil {
		t.Fatalf("SubmitCommand failed: %v", err)
	}
	if wolf.health != beforeHealth {
		t.Fatalf("iframe hit changed health: got %.1f want %.1f", wolf.health, beforeHealth)
	}
	if wolf.posture != beforePosture {
		t.Fatalf("iframe hit changed posture: got %.1f want %.1f", wolf.posture, beforePosture)
	}
}
