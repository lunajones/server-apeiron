package gameapi

import "testing"

// TestPlayerImpactResponseProfileContractDriven evolves the player impact response from a
// hardcoded string toward a contract-driven value (mirrors the wolf's DB-loaded
// impact_response_profile). An empty contract value falls back to the canonical default so
// current in-game behavior is preserved until a DB player-material profile is introduced.
func TestPlayerImpactResponseProfileContractDriven(t *testing.T) {
	if got := (RuntimeContracts{}).playerImpactResponse(); got != "flesh_blood_red" {
		t.Fatalf("default = %q, want flesh_blood_red", got)
	}
	c := RuntimeContracts{PlayerImpactResponseProfile: " flesh_blood_red_heavy "}
	if got := c.playerImpactResponse(); got != "flesh_blood_red_heavy" {
		t.Fatalf("contract value = %q, want flesh_blood_red_heavy (trimmed)", got)
	}
}
