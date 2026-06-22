package gameapi

import (
	"context"
	"testing"
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
}

// TestRecoveredPlayerSkillsCarrySeedDamage locks that player skill contracts carry the
// canonical base/posture damage from db-apeiron seed 013 (damage-pipeline brick 2a).
// When the DB skill proto exposes damage (brick 2b), this should be sourced from the DB.
func TestRecoveredPlayerSkillsCarrySeedDamage(t *testing.T) {
	contracts := RecoveredRuntimeContracts()

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
