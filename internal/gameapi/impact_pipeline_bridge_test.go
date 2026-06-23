package gameapi

import (
	"context"
	"testing"
	"time"

	gamev1 "server-apeiron/gen/apeiron/game/v1"
)

func TestRuntimeSkillImpactUsesCombatPipelineParry(t *testing.T) {
	t.Parallel()

	runtime := NewRuntimeWithContracts(DevFixtureRuntimeContracts())
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

	runtime := NewRuntimeWithContracts(DevFixtureRuntimeContracts())
	sessionID := "runtime-skill-impact-iframe"
	attachRuntimePlayer(t, runtime, sessionID)
	player := runtime.ensurePlayerLocked("local_player")
	wolf := runtime.ensureWolfLocked(player)
	wolf.position = vector{x: player.position.x + 50, y: player.position.y, z: player.position.z}
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

	runtime := NewRuntimeWithContracts(DevFixtureRuntimeContracts())
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

	runtime := NewRuntimeWithContracts(DevFixtureRuntimeContracts())
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
	if wolf.actionMotion.MotionSource != "impact_control" {
		t.Fatalf("target control motion source = %q", wolf.actionMotion.MotionSource)
	}
	startX := wolf.position.x
	wolf.actionMotion.StartedAt = time.Now().Add(-time.Duration(contract.ControlEffects[0].GetDurationMs()/2) * time.Millisecond)
	runtime.advanceActionMotionLocked(wolf, time.Now())
	if wolf.position.x <= startX {
		t.Fatalf("target control motion did not move forward: %.1f -> %.1f", startX, wolf.position.x)
	}
	wolf.actionMotion.StartedAt = time.Now().Add(-time.Duration(contract.ControlEffects[0].GetDurationMs()+20) * time.Millisecond)
	runtime.advanceActionMotionLocked(wolf, time.Now())
	if wolf.actionMotion != nil {
		t.Fatalf("completed impact control left actionMotion active: %#v", wolf.actionMotion)
	}
	if wolf.skillState != "idle" || wolf.combatState != "ready" {
		t.Fatalf("completed impact control left target state skill=%q combat=%q", wolf.skillState, wolf.combatState)
	}
	if wolf.locomotion == nil || wolf.locomotion.GetAction() != "post_impact_control" {
		t.Fatalf("completed impact control did not publish post-control locomotion: %#v", wolf.locomotion)
	}
}

func TestWolfMaulImpactControlUsesSourceActionDirection(t *testing.T) {
	t.Parallel()

	runtime := NewRuntimeWithContracts(DevFixtureRuntimeContracts())
	sessionID := "runtime-maul-impact-control"
	attachRuntimePlayer(t, runtime, sessionID)
	player := runtime.ensurePlayerLocked("local_player")
	wolf := runtime.ensureWolfLocked(player)
	player.position = vector{x: 0, y: 0, z: 98}
	wolf.position = vector{x: 120, y: 0, z: 98}

	maul := runtime.contracts.skillContract("maul")
	wolf.actionMotion = &actionMotionState{
		SkillID:       "maul",
		MotionSource:  "skill_root_motion",
		StartedAt:     time.Now(),
		StartPosition: wolf.position,
		Direction:     vector{x: 0, y: 1},
		Contract:      maul.MovementAction,
	}

	impact, ok := runtime.resolveRuntimeSkillImpact(wolf, player, maul, maul.Hitboxes[0], wolf.position, vector{x: 1, y: 0})
	if !ok {
		t.Fatal("expected Maul impact")
	}
	if len(impact.StatusApplied) != 1 || impact.StatusApplied[0] != "impact_wolf_maul_lateral_grab" {
		t.Fatalf("maul control status was not applied: %#v", impact.StatusApplied)
	}
	if impact.ControlType != "grab" {
		t.Fatalf("maul control type = %q", impact.ControlType)
	}
	if impact.ControlReleasePolicy != "lateral_grab_release" {
		t.Fatalf("maul release policy = %q", impact.ControlReleasePolicy)
	}
	if player.actionMotion == nil {
		t.Fatal("maul impact control did not start player action motion")
	}
	if player.actionMotion.MotionSource != "impact_control" {
		t.Fatalf("player control motion source = %q", player.actionMotion.MotionSource)
	}
	if player.actionMotion.Direction.y <= 0 || player.actionMotion.Direction.x != 0 {
		t.Fatalf("maul control direction = %#v, want source action direction", player.actionMotion.Direction)
	}
	startY := player.position.y
	player.actionMotion.StartedAt = time.Now().Add(-time.Duration(maul.ControlEffects[0].GetDurationMs()/2) * time.Millisecond)
	runtime.advanceActionMotionLocked(player, time.Now())
	if player.position.y <= startY {
		t.Fatalf("maul control did not drag player laterally: %.1f -> %.1f", startY, player.position.y)
	}
}

func TestImpactControlInterruptsCreatureActionAndCancelsPendingDamage(t *testing.T) {
	t.Parallel()

	runtime := NewRuntimeWithContracts(DevFixtureRuntimeContracts())
	sessionID := "runtime-impact-control-interrupts-creature"
	attachRuntimePlayer(t, runtime, sessionID)
	player := runtime.ensurePlayerLocked("local_player")
	wolf := runtime.ensureWolfLocked(player)
	player.position = vector{x: 0, y: 0, z: 98}
	wolf.position = vector{x: 120, y: 0, z: 98}

	lunge := runtime.contracts.skillContract("lunge")
	lungeStart := time.Now()
	lungeInstance := runtime.newCreatureActionInstance(wolf, "lunge", lunge, wolf.position, lungeStart)
	wolf.actionInstance = &lungeInstance
	wolf.skillRuntime = &gamev1.SkillRuntimeState{
		CurrentSkillId: "lunge",
		State:          "active",
		StartedAtMs:    lungeStart.UnixMilli(),
	}
	if !runtime.enqueueCreatureSkillImpactLocked(wolf, player, lunge, lungeStart) {
		t.Fatal("failed to enqueue pending lunge impact")
	}
	if runtime.impacts == nil || runtime.impacts.PendingCount() != 1 {
		t.Fatalf("pending lunge impact count = %d", runtime.impacts.PendingCount())
	}

	shieldRush := runtime.contracts.skillContract("player_shield_rush")
	impact, ok := runtime.resolveRuntimeSkillImpact(player, wolf, shieldRush, shieldRush.Hitboxes[0], player.position, vector{x: 1, y: 0})
	if !ok {
		t.Fatal("expected Shield Rush impact")
	}
	if len(impact.StatusApplied) == 0 {
		t.Fatalf("Shield Rush did not apply control: %#v", impact)
	}
	if wolf.actionInstance != nil {
		t.Fatalf("impact control did not clear interrupted creature action instance: %#v", wolf.actionInstance)
	}
	if wolf.actionMotion == nil || wolf.actionMotion.MotionSource != "impact_control" {
		t.Fatalf("impact control did not become target motion: %#v", wolf.actionMotion)
	}
	if runtime.impacts.PendingCount() != 0 {
		t.Fatalf("interrupted lunge impact remained pending: %d", runtime.impacts.PendingCount())
	}

	healthBefore := player.health
	impacts := runtime.runPendingSkillImpactSchedulesLocked(lungeStart.Add(4 * time.Second))
	if len(impacts) != 0 {
		t.Fatalf("interrupted lunge still resolved impacts: %#v", impacts)
	}
	if player.health != healthBefore {
		t.Fatalf("interrupted lunge changed player health: %.1f -> %.1f", healthBefore, player.health)
	}
}
