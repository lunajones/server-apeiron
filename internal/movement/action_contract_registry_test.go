package movement

import "testing"

func TestActionContractRegistryOrdersPreferredAndExtras(t *testing.T) {
	registry := NewActionContractRegistry(map[string]RuntimeActionContract{
		"turn":   {ID: "turn_v1", AbilityKey: "turn"},
		"custom": {ID: "custom_v1", AbilityKey: "custom"},
		"move":   {ID: "move_v1", AbilityKey: "move"},
	})

	keys := registry.OrderedKeys([]string{"move", "turn"})
	want := []string{"move", "turn", "custom"}
	if len(keys) != len(want) {
		t.Fatalf("expected %d keys, got %d: %v", len(want), len(keys), keys)
	}
	for i := range want {
		if keys[i] != want[i] {
			t.Fatalf("key %d = %q, want %q; got %v", i, keys[i], want[i], keys)
		}
	}
}

func TestRuntimeActionContractClassification(t *testing.T) {
	skill := RuntimeActionContract{
		ID:                       "player_shield_rush_v1",
		ActionType:               "grounded_skill",
		RootMotionOwner:          "skill",
		ReconciliationContractID: "grounded_skill_action_reconciliation",
	}
	if got := ActionFamily(skill); got != "skill_movement" {
		t.Fatalf("skill action family = %q", got)
	}
	if got := ReconciliationMode(skill); got != "grounded_skill_action_reconciliation" {
		t.Fatalf("skill reconciliation = %q", got)
	}
	if got := ContractHash(skill); got != "grounded_skill_action_reconciliation" {
		t.Fatalf("skill contract hash = %q", got)
	}

	move := RuntimeActionContract{
		ID:                     "grounded_move_v1",
		ActionType:             "move",
		ReconciliationCategory: "grounded_move_reconciliation",
	}
	if got := ActionFamily(move); got != "movement" {
		t.Fatalf("move action family = %q", got)
	}
	if got := ReconciliationMode(move); got != "grounded_move_reconciliation" {
		t.Fatalf("move reconciliation = %q", got)
	}
}
