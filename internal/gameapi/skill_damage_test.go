package gameapi

import (
	"context"
	"math"
	"testing"
	"time"

	gamev1 "server-apeiron/gen/apeiron/game/v1"
)

// TestLoadSkillRuntimeContractMapsDBDamage locks brick 2b: loadSkillRuntimeContract
// pulls base_damage/posture_damage/max_range from the DB Skill (GetSkill) into the
// runtime contract. The fake source returns 12/20/300.
func TestLoadSkillRuntimeContractMapsDBDamage(t *testing.T) {
	c, ok := loadSkillRuntimeContract(context.Background(), fakeRuntimeContractSource{}, "player_shield_rush")
	if !ok {
		t.Fatal("expected skill to load from DB source")
	}
	if c.Damage != 12 || c.PostureDamage != 20 || c.Range != 300 {
		t.Fatalf("DB damage mapping: damage=%v posture=%v range=%v, want 12/20/300", c.Damage, c.PostureDamage, c.Range)
	}
	if len(c.Hitboxes) == 0 {
		t.Fatal("DB hitbox profiles were not mapped into the runtime contract")
	}
}

// TestFixturePlayerSkillsCarrySeedDamage locks that player skill contracts carry the
// canonical base/posture damage from db-apeiron seed 013 (damage-pipeline brick 2a).
// When the DB skill proto exposes damage (brick 2b), this should be sourced from the DB.
func TestFixturePlayerSkillsCarrySeedDamage(t *testing.T) {
	contracts := DevFixtureRuntimeContracts()

	cases := map[string]struct {
		damage  float64
		posture float64
	}{
		"player_shield_rush":    {14, 34},
		"player_shield_bash":    {10, 26},
		"player_basic_attack_1": {8, 10},
		"player_basic_attack_2": {7, 9},
		"player_basic_attack_3": {6, 18},
	}

	for id, want := range cases {
		c := contracts.skillContract(id)
		if c.SkillID != id {
			t.Fatalf("missing skill contract %q (got %q)", id, c.SkillID)
		}
		if c.Damage != want.damage || c.PostureDamage != want.posture {
			t.Fatalf("%s damage=%v/%v, want %v/%v", id, c.Damage, c.PostureDamage, want.damage, want.posture)
		}
	}
}

func TestRuntimeSkillImpactAppliesDBContractDamage(t *testing.T) {
	t.Parallel()

	runtime := NewRuntimeWithOptions(DevFixtureRuntimeContracts(), RuntimeOptions{DisableCreatures: true})
	sessionID := "runtime-skill-impact-damage"
	attachRuntimePlayer(t, runtime, sessionID)
	player := runtime.ensurePlayerLocked("local_player")
	wolf := runtime.ensureWolfLocked(player)
	wolf.position = vector{x: player.position.x + 60, y: player.position.y, z: player.position.z}
	before := wolf.health

	ack, err := runtime.SubmitCommand(context.Background(), testRuntimeCastSkillCommand(sessionID, 1, "player_basic_attack_1", gamev1Vector(1, 0, 0)))
	if err != nil {
		t.Fatalf("SubmitCommand failed: %v", err)
	}
	if !ack.GetAccepted() {
		t.Fatalf("cast rejected: %s %s", ack.GetRejectionCode(), ack.GetMessage())
	}
	if wolf.health != before {
		t.Fatalf("SubmitCommand applied damage before temporal runner: %.1f -> %.1f", before, wolf.health)
	}
	contract := runtime.contracts.skillContract("player_basic_attack_1")
	impacts := runtime.runPendingSkillImpactSchedulesLocked(runtimeSkillImpactWindowTime(player, contract))
	if len(impacts) != 1 {
		t.Fatalf("pending impact runner resolved %d impacts, want 1", len(impacts))
	}
	wantHealth := before - 8*1.05
	if math.Abs(wolf.health-wantHealth) > 0.001 {
		t.Fatalf("wolf health = %v, want %v", wolf.health, wantHealth)
	}
	wantPosture := wolf.maxPosture - 10*1.15
	if math.Abs(wolf.posture-wantPosture) > 0.001 {
		t.Fatalf("wolf posture = %v, want %v", wolf.posture, wantPosture)
	}
}

func TestRuntimePlayerSkillImpactSchedulerDedupesActionInstance(t *testing.T) {
	t.Parallel()

	runtime := NewRuntimeWithOptions(DevFixtureRuntimeContracts(), RuntimeOptions{DisableCreatures: true})
	sessionID := "runtime-player-impact-scheduler-dedupe"
	attachRuntimePlayer(t, runtime, sessionID)
	player := runtime.ensurePlayerLocked("local_player")
	wolf := runtime.ensureWolfLocked(player)
	wolf.position = vector{x: player.position.x + 60, y: player.position.y, z: player.position.z}

	ack, err := runtime.SubmitCommand(context.Background(), testRuntimeCastSkillCommand(sessionID, 1, "player_basic_attack_1", gamev1Vector(1, 0, 0)))
	if err != nil {
		t.Fatalf("SubmitCommand failed: %v", err)
	}
	if !ack.GetAccepted() {
		t.Fatalf("cast rejected: %s %s", ack.GetRejectionCode(), ack.GetMessage())
	}
	if player.actionInstance == nil {
		t.Fatal("player basic attack did not create action instance")
	}

	contract := runtime.contracts.skillContract("player_basic_attack_1")
	impacts := runtime.runPendingSkillImpactSchedulesLocked(runtimeSkillImpactWindowTime(player, contract))
	if len(impacts) != 1 {
		t.Fatalf("first pending impact runner resolved %d impacts, want 1", len(impacts))
	}
	healthAfterFirstImpact := wolf.health
	again := runtime.runPendingSkillImpactSchedulesLocked(runtimeSkillImpactWindowTime(player, contract).Add(16 * time.Millisecond))
	if len(again) != 0 {
		t.Fatalf("same player action instance applied duplicate impacts: %d", len(again))
	}
	if wolf.health != healthAfterFirstImpact {
		t.Fatalf("duplicate player action changed wolf health: %.2f -> %.2f", healthAfterFirstImpact, wolf.health)
	}
}

func TestRuntimePendingImpactRunnerCatchesSkippedHitboxWindow(t *testing.T) {
	t.Parallel()

	runtime := NewRuntimeWithOptions(DevFixtureRuntimeContracts(), RuntimeOptions{DisableCreatures: true})
	sessionID := "runtime-player-impact-scheduler-skipped-window"
	attachRuntimePlayer(t, runtime, sessionID)
	player := runtime.ensurePlayerLocked("local_player")
	wolf := runtime.ensureWolfLocked(player)
	wolf.position = vector{x: player.position.x + 60, y: player.position.y, z: player.position.z}
	before := wolf.health

	ack, err := runtime.SubmitCommand(context.Background(), testRuntimeCastSkillCommand(sessionID, 1, "player_basic_attack_1", gamev1Vector(1, 0, 0)))
	if err != nil {
		t.Fatalf("SubmitCommand failed: %v", err)
	}
	if !ack.GetAccepted() {
		t.Fatalf("cast rejected: %s %s", ack.GetRejectionCode(), ack.GetMessage())
	}
	contract := runtime.contracts.skillContract("player_basic_attack_1")
	endMS, ok := skillLatestImpactWindowEndMS(contract)
	if !ok {
		t.Fatal("basic attack contract has no temporal impact window")
	}

	impacts := runtime.runPendingSkillImpactSchedulesLocked(player.actionInstance.StartedAt.Add(time.Duration(endMS+80) * time.Millisecond))
	if len(impacts) != 1 {
		t.Fatalf("skipped-window runner resolved %d impacts, want 1", len(impacts))
	}
	if wolf.health >= before {
		t.Fatalf("skipped-window runner did not damage wolf: %.1f -> %.1f", before, wolf.health)
	}
}

func TestSnapshotEmitsDamageEventWithImpactResponseProfile(t *testing.T) {
	t.Parallel()

	runtime := NewRuntimeWithOptions(DevFixtureRuntimeContracts(), RuntimeOptions{DisableCreatures: true})
	sessionID := "runtime-impact-event-profile"
	attachRuntimePlayer(t, runtime, sessionID)
	player := runtime.ensurePlayerLocked("local_player")
	wolf := runtime.ensureWolfLocked(player)
	wolf.position = vector{x: player.position.x + 60, y: player.position.y, z: player.position.z}

	contract := runtime.contracts.skillContract("player_basic_attack_1")
	evaluationMS := runtimeSkillImpactSnapshotElapsedMS(contract)
	startedAt := time.Now().Add(-time.Duration(evaluationMS) * time.Millisecond)
	runtime.enqueueSkillImpactScheduleLocked(skillImpactScheduleFromActionInstance(
		player,
		contract,
		"test-impact-event-profile",
		startedAt,
		player.position,
		vector{x: wolf.position.x, y: wolf.position.y, z: wolf.position.z},
		vector{x: 1, y: 0},
		0,
	))

	snapshot, err := runtime.GetSnapshot(context.Background(), &gamev1.SnapshotRequest{
		Context: &gamev1.RequestContext{SessionId: sessionID},
	})
	if err != nil {
		t.Fatalf("GetSnapshot failed: %v", err)
	}
	if len(snapshot.GetEvents()) == 0 {
		t.Fatal("snapshot did not emit damage event")
	}
	event := snapshot.GetEvents()[0]
	if event.GetType() != gamev1.SnapshotEventType_ENTITY_EVENT_TYPE_DAMAGE_APPLIED {
		t.Fatalf("event type = %s", event.GetType())
	}
	if got := event.GetMetadata()["impact_response_profile"]; got != "creature_flesh_blood_red" {
		t.Fatalf("impact response profile metadata = %q", got)
	}
	if got := event.GetMetadata()["feedback_authority"]; got != "server_damage_event" {
		t.Fatalf("feedback authority = %q", got)
	}
}

func TestSnapshotDamageEventCarriesAppliedControlMetadata(t *testing.T) {
	t.Parallel()

	runtime := NewRuntimeWithContracts(DevFixtureRuntimeContracts())
	sessionID := "runtime-impact-event-control"
	attachRuntimePlayer(t, runtime, sessionID)
	player := runtime.ensurePlayerLocked("local_player")
	wolf := runtime.ensureWolfLocked(player)
	wolf.position = vector{x: player.position.x + 50, y: player.position.y, z: player.position.z}

	contract := runtime.contracts.skillContract("player_shield_rush")
	evaluationMS := 520.0
	if _, ok := skillRuntimeHitboxContainsAt(contract, player.position, vector{x: wolf.position.x, y: wolf.position.y, z: wolf.position.z}, vector{x: 1, y: 0}, wolf.position, evaluationMS); !ok {
		t.Fatal("test setup target is outside Shield Rush temporal contact at evaluation time")
	}
	startedAt := time.Now().Add(-time.Duration(evaluationMS) * time.Millisecond)
	if !runtime.enqueueSkillImpactScheduleLocked(skillImpactScheduleFromActionInstance(
		player,
		contract,
		"test-impact-event-control",
		startedAt,
		player.position,
		vector{x: wolf.position.x, y: wolf.position.y, z: wolf.position.z},
		vector{x: 1, y: 0},
		0,
	)) {
		t.Fatal("failed to enqueue Shield Rush impact schedule")
	}

	events := runtime.damageEventsFromImpactsLocked(
		runtime.runPendingSkillImpactSchedulesLocked(startedAt.Add(time.Duration(evaluationMS+1) * time.Millisecond)),
	)
	if len(events) == 0 {
		t.Fatal("impact scheduler did not emit damage event")
	}
	metadata := events[0].GetMetadata()
	if got := metadata["control_applied"]; got != "true" {
		t.Fatalf("control_applied = %q", got)
	}
	if got := metadata["status_applied"]; got != "impact_shield_rush_carry_push" {
		t.Fatalf("status_applied = %q", got)
	}
	if got := metadata["control_type"]; got != "carry_push" {
		t.Fatalf("control_type = %q", got)
	}
	if got := metadata["control_release_policy"]; got != "multi_target_carry_push_forward_release" {
		t.Fatalf("control_release_policy = %q", got)
	}
	if got := metadata["control_distance_cm"]; got != "960.000" {
		t.Fatalf("control_distance_cm = %q", got)
	}
	if got := metadata["control_speed_cm_s"]; got != "1333.333" {
		t.Fatalf("control_speed_cm_s = %q", got)
	}
	if got := metadata["control_direction_policy"]; got != "source_forward" {
		t.Fatalf("control_direction_policy = %q", got)
	}
}

func TestRuntimeSkillImpactHonorsDirectionalBlock(t *testing.T) {
	t.Parallel()

	runtime := NewRuntimeWithContracts(DevFixtureRuntimeContracts())
	sessionID := "runtime-skill-impact-block"
	attachRuntimePlayer(t, runtime, sessionID)
	player := runtime.ensurePlayerLocked("local_player")
	wolf := runtime.ensureWolfLocked(player)
	wolf.position = vector{x: player.position.x - 160, y: player.position.y, z: player.position.z}
	wolf.yaw = 0
	player.yaw = 180
	player.combatState = "blocking"
	beforeHealth := player.health

	contract := runtime.contracts.skillContract("bite")
	profile := contract.Hitboxes[0]
	impact, ok := runtime.resolveRuntimeSkillImpact(wolf, player, contract, profile, wolf.position, vector{x: 1, y: 0})
	if !ok {
		t.Fatal("expected creature bite impact resolution")
	}
	if !impact.Blocked {
		t.Fatalf("impact was not blocked: %#v", impact)
	}
	if impact.DamageApplied != 0 {
		t.Fatalf("blocked hit applied health damage: got %v", impact.DamageApplied)
	}
	if player.health != beforeHealth {
		t.Fatalf("resolve-only impact mutated player health: got %v want %v", player.health, beforeHealth)
	}
	if impact.PostureApplied != contract.PostureDamage {
		t.Fatalf("blocked hit should apply posture pressure: got %v want %v", impact.PostureApplied, contract.PostureDamage)
	}
}

func TestRuntimeSkillImpactMissesOutsideHitbox(t *testing.T) {
	t.Parallel()

	runtime := NewRuntimeWithContracts(DevFixtureRuntimeContracts())
	sessionID := "runtime-skill-impact-miss"
	attachRuntimePlayer(t, runtime, sessionID)
	player := runtime.ensurePlayerLocked("local_player")
	wolf := runtime.ensureWolfLocked(player)
	wolf.position = vector{x: player.position.x + 80, y: player.position.y + 500, z: player.position.z}
	before := wolf.health

	if _, err := runtime.SubmitCommand(context.Background(), testRuntimeCastSkillCommand(sessionID, 1, "player_basic_attack_1", gamev1Vector(1, 0, 0))); err != nil {
		t.Fatalf("SubmitCommand failed: %v", err)
	}
	contract := runtime.contracts.skillContract("player_basic_attack_1")
	runtime.runPendingSkillImpactSchedulesLocked(runtimeSkillImpactWindowTime(player, contract))
	if wolf.health != before {
		t.Fatalf("outside hitbox changed health: got %v want %v", wolf.health, before)
	}
}

func runtimeSkillImpactWindowTime(source *entityState, contract SkillRuntimeContract) time.Time {
	elapsed := runtimeSkillImpactTestEvaluationElapsedMS(contract)
	if source != nil && source.actionInstance != nil {
		return source.actionInstance.StartedAt.Add(time.Duration(elapsed+1) * time.Millisecond)
	}
	return time.Now().Add(time.Duration(elapsed+1) * time.Millisecond)
}

func runtimeSkillImpactTestEvaluationElapsedMS(contract SkillRuntimeContract) float64 {
	if endMS, ok := skillLatestImpactWindowEndMS(contract); ok {
		return endMS
	}
	elapsed := skillImpactEvaluationElapsedMS(contract)
	if elapsed < 0 {
		return 0
	}
	return elapsed
}

func runtimeSkillImpactSnapshotElapsedMS(contract SkillRuntimeContract) float64 {
	elapsed := runtimeSkillImpactTestEvaluationElapsedMS(contract)
	if elapsed > 1 {
		return elapsed - 1
	}
	return elapsed
}

func attachRuntimePlayer(t *testing.T, runtime *Runtime, sessionID string) {
	t.Helper()
	if _, err := runtime.OpenSession(context.Background(), &gamev1.OpenSessionRequest{
		Context: &gamev1.RequestContext{SessionId: sessionID},
	}); err != nil {
		t.Fatalf("OpenSession failed: %v", err)
	}
	if _, err := runtime.AttachPlayer(context.Background(), &gamev1.AttachPlayerRequest{
		Context:  &gamev1.RequestContext{SessionId: sessionID},
		PlayerId: "local_player",
	}); err != nil {
		t.Fatalf("AttachPlayer failed: %v", err)
	}
}
