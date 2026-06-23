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
	wolf.position = vector{x: player.position.x - 160, y: player.position.y, z: player.position.z}
	wolf.yaw = 0
	player.yaw = 180
	player.combatState = "parry"
	beforeHealth := player.health
	beforePosture := player.posture

	contract := runtime.contracts.skillContract("bite")
	profile := contract.Hitboxes[0]
	impact, ok := runtime.resolveRuntimeSkillImpact(wolf, player, contract, profile, wolf.position, vector{x: 1, y: 0})
	if !ok {
		t.Fatal("expected creature bite impact resolution")
	}
	if !impact.Parried {
		t.Fatalf("impact was not parried: %#v", impact)
	}
	if impact.DamageApplied != 0 || impact.PostureApplied != 0 {
		t.Fatalf("parried hit applied damage/posture: %#v", impact)
	}
	if player.health != beforeHealth {
		t.Fatalf("resolve-only parry mutated health: got %.1f want %.1f", player.health, beforeHealth)
	}
	if player.posture != beforePosture {
		t.Fatalf("resolve-only parry mutated posture: got %.1f want %.1f", player.posture, beforePosture)
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
