package gameapi

import (
	"testing"

	"server-apeiron/internal/movement"
)

// unrealRecognizedSkillReconciliationModes mirrors the skill cases of
// ApeironGameServerBridge.cpp:ApeironReconciliationModeFromServerString in the Unreal
// client. Any other string is parsed as EApeironPlayerReconciliationMode::None, which
// makes the player skill reconcile as a generic correction instead of the dedicated
// skill-grounded replay — the end-of-skill (and mid-skill) rubberband on R/F.
var unrealRecognizedSkillReconciliationModes = map[string]bool{
	"SkillGroundedAction":   true,
	"grounded_skill_action": true,
	"grounded_skill":        true,
	"skill_grounded_action": true,
}

// TestPlayerSkillPublishesUnrealRecognizedReconciliationMode locks the cross-layer
// contract: the reconciliation_mode the server publishes for a player skill must be a
// string the Unreal client maps to SkillGroundedAction. The verbose contract-id form
// "grounded_skill_action_reconciliation" parsed as None and was the Shield Rush/Bash
// rubberband cause.
func TestPlayerSkillPublishesUnrealRecognizedReconciliationMode(t *testing.T) {
	contract := fixtureSkillContract("player_shield_rush", 320, 240, 140, 60).MovementAction
	mode := movement.ReconciliationMode(contract)
	if !unrealRecognizedSkillReconciliationModes[mode] {
		t.Fatalf("player skill reconciliation_mode = %q is NOT recognized by the Unreal client "+
			"(ApeironReconciliationModeFromServerString) -> parsed as None -> rubberband. "+
			"Recognized: SkillGroundedAction / grounded_skill_action / skill_grounded_action", mode)
	}
}
