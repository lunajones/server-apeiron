package gameapi

import (
	"context"
	"testing"

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

// TestRecoveredPlayerSkillsCarrySeedDamage locks that player skill contracts carry the
// canonical base/posture damage from db-apeiron seed 013 (damage-pipeline brick 2a).
// When the DB skill proto exposes damage (brick 2b), this should be sourced from the DB.
func TestRecoveredPlayerSkillsCarrySeedDamage(t *testing.T) {
	contracts := RecoveryFixtureRuntimeContracts()

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

	runtime := NewRuntimeWithContracts(RecoveryFixtureRuntimeContracts())
	sessionID := "runtime-skill-impact-damage"
	attachRuntimePlayer(t, runtime, sessionID)
	player := runtime.ensurePlayerLocked("local_player")
	wolf := runtime.ensureWolfLocked(player)
	wolf.position = vector{x: player.position.x + 120, y: player.position.y, z: player.position.z}
	before := wolf.health

	ack, err := runtime.SubmitCommand(context.Background(), testRuntimeCastSkillCommand(sessionID, 1, "player_basic_attack_1", gamev1Vector(1, 0, 0)))
	if err != nil {
		t.Fatalf("SubmitCommand failed: %v", err)
	}
	if !ack.GetAccepted() {
		t.Fatalf("cast rejected: %s %s", ack.GetRejectionCode(), ack.GetMessage())
	}
	if wolf.health != before-8 {
		t.Fatalf("wolf health = %v, want %v", wolf.health, before-8)
	}
	if wolf.posture != wolf.maxPosture-10 {
		t.Fatalf("wolf posture = %v, want %v", wolf.posture, wolf.maxPosture-10)
	}
}

func TestRuntimeSkillImpactHonorsDirectionalBlock(t *testing.T) {
	t.Parallel()

	runtime := NewRuntimeWithContracts(RecoveryFixtureRuntimeContracts())
	sessionID := "runtime-skill-impact-block"
	attachRuntimePlayer(t, runtime, sessionID)
	player := runtime.ensurePlayerLocked("local_player")
	wolf := runtime.ensureWolfLocked(player)
	wolf.position = vector{x: player.position.x + 120, y: player.position.y, z: player.position.z}
	wolf.yaw = 180
	wolf.combatState = "blocking"
	beforeHealth := wolf.health

	if _, err := runtime.SubmitCommand(context.Background(), testRuntimeCastSkillCommand(sessionID, 1, "player_basic_attack_1", gamev1Vector(1, 0, 0))); err != nil {
		t.Fatalf("SubmitCommand failed: %v", err)
	}
	if wolf.health != beforeHealth {
		t.Fatalf("blocked hit changed health: got %v want %v", wolf.health, beforeHealth)
	}
	if wolf.posture != wolf.maxPosture-10 {
		t.Fatalf("blocked hit should apply posture pressure: got %v want %v", wolf.posture, wolf.maxPosture-10)
	}
}

func TestRuntimeSkillImpactMissesOutsideHitbox(t *testing.T) {
	t.Parallel()

	runtime := NewRuntimeWithContracts(RecoveryFixtureRuntimeContracts())
	sessionID := "runtime-skill-impact-miss"
	attachRuntimePlayer(t, runtime, sessionID)
	player := runtime.ensurePlayerLocked("local_player")
	wolf := runtime.ensureWolfLocked(player)
	wolf.position = vector{x: player.position.x + 80, y: player.position.y + 500, z: player.position.z}
	before := wolf.health

	if _, err := runtime.SubmitCommand(context.Background(), testRuntimeCastSkillCommand(sessionID, 1, "player_basic_attack_1", gamev1Vector(1, 0, 0))); err != nil {
		t.Fatalf("SubmitCommand failed: %v", err)
	}
	if wolf.health != before {
		t.Fatalf("outside hitbox changed health: got %v want %v", wolf.health, before)
	}
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
