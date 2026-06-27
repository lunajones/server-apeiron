package gameapi

import "testing"

// TestDamageEventPublishesRealDamageType locks Damage Slice 5: the snapshot damage event carries
// the hit's actual damage type and family (no longer the hardcoded "physical"), so the client can
// show type-correct hit feedback.
func TestDamageEventPublishesRealDamageType(t *testing.T) {
	runtime := NewRuntimeWithContracts(DevFixtureRuntimeContracts())
	player := runtime.ensurePlayerLocked("local_player")
	wolf := runtime.ensureWolfLocked(player)

	impacts := []runtimeSkillImpact{{
		SourceID:      wolf.id,
		TargetID:      player.id,
		SkillID:       "lunge",
		DamageType:    "fire",
		DamageFamily:  "chemical",
		DamageApplied: 10,
	}}

	events := runtime.damageEventsFromImpactsLocked(impacts)
	if len(events) == 0 {
		t.Fatal("no damage event produced")
	}
	md := events[0].GetMetadata()
	if md["damage_type"] != "fire" {
		t.Fatalf("event damage_type = %q, want fire (real type, not hardcoded physical)", md["damage_type"])
	}
	if md["damage_family"] != "chemical" {
		t.Fatalf("event damage_family = %q, want chemical", md["damage_family"])
	}
}
