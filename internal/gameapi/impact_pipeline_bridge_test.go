package gameapi

import (
	"context"
	"testing"
	"time"
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

func TestRuntimeSkillImpactCarriesTargetImpactResponseProfile(t *testing.T) {
	t.Parallel()

	runtime := NewRuntimeWithContracts(RecoveryFixtureRuntimeContracts())
	sessionID := "runtime-skill-impact-response-profile"
	attachRuntimePlayer(t, runtime, sessionID)
	player := runtime.ensurePlayerLocked("local_player")
	wolf := runtime.ensureWolfLocked(player)
	wolf.position = vector{x: player.position.x + 120, y: player.position.y, z: player.position.z}
	wolf.impactResponseProfile = "creature_flesh_blood_red"

	contract := runtime.contracts.skillContract("player_basic_attack_1")
	profile := contract.Hitboxes[0]
	impact, ok := runtime.resolveRuntimeSkillImpact(player, wolf, contract, profile, player.position, vector{x: 1, y: 0})
	if !ok {
		t.Fatal("expected player impact resolution")
	}
	if impact.ImpactResponseProfile != "creature_flesh_blood_red" {
		t.Fatalf("impact response profile = %q", impact.ImpactResponseProfile)
	}
	if impact.ImpactType == "" {
		t.Fatal("impact type was not populated")
	}
}

func TestRuntimeSkillImpactAppliesContractControlEffects(t *testing.T) {
	t.Parallel()

	runtime := NewRuntimeWithContracts(RecoveryFixtureRuntimeContracts())
	sessionID := "runtime-skill-impact-control-effect"
	attachRuntimePlayer(t, runtime, sessionID)
	player := runtime.ensurePlayerLocked("local_player")
	wolf := runtime.ensureWolfLocked(player)
	wolf.position = vector{x: player.position.x + 120, y: player.position.y, z: player.position.z}

	contract := runtime.contracts.skillContract("player_shield_rush")
	profile := contract.Hitboxes[0]
	impact, ok := runtime.resolveRuntimeSkillImpact(player, wolf, contract, profile, player.position, vector{x: 1, y: 0})
	if !ok {
		t.Fatal("expected Shield Rush impact resolution")
	}
	if len(impact.StatusApplied) != 1 || impact.StatusApplied[0] != "impact_shield_rush_carry_push" {
		t.Fatalf("control status was not applied through pipeline: %#v", impact.StatusApplied)
	}
	if impact.ControlType != "carry_push" {
		t.Fatalf("control type = %q", impact.ControlType)
	}
	if impact.ControlReleasePolicy != "multi_target_carry_push_forward_release" {
		t.Fatalf("release policy = %q", impact.ControlReleasePolicy)
	}
	if wolf.actionMotion == nil {
		t.Fatal("applied control effect did not start target action motion")
	}
	if wolf.actionMotion.Contract.AbilityKey != "impact_shield_rush_carry_push" {
		t.Fatalf("target control action ability = %q", wolf.actionMotion.Contract.AbilityKey)
	}
	if wolf.actionMotion.Contract.DistanceCM != contract.ControlEffects[0].GetDistanceCm() {
		t.Fatalf("target control distance = %.1f, want %.1f", wolf.actionMotion.Contract.DistanceCM, contract.ControlEffects[0].GetDistanceCm())
	}
	startX := wolf.position.x
	wolf.actionMotion.StartedAt = time.Now().Add(-time.Duration(contract.ControlEffects[0].GetDurationMs()/2) * time.Millisecond)
	runtime.advanceActionMotionLocked(wolf, time.Now())
	if wolf.position.x <= startX {
		t.Fatalf("target control motion did not move forward: %.1f -> %.1f", startX, wolf.position.x)
	}
}
