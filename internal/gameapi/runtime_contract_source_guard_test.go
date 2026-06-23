package gameapi

import (
	"context"
	"strings"
	"testing"

	gamev1 "server-apeiron/gen/apeiron/game/v1"
)

func TestRuntimeContractSourceNamesAreIntentionalAndDistinct(t *testing.T) {
	sources := []string{
		runtimeContractSourceDB,
		runtimeContractSourceDBIncomplete,
		runtimeContractSourceRecoveryFixture,
		runtimeContractSourceUnconfigured,
	}
	seen := map[string]bool{}
	for _, source := range sources {
		if strings.TrimSpace(source) == "" {
			t.Fatalf("runtime contract source must not be empty: %#v", sources)
		}
		if seen[source] {
			t.Fatalf("runtime contract source %q is duplicated: %#v", source, sources)
		}
		seen[source] = true
	}
}

func TestDefaultRuntimeIsUnconfiguredAndNeverLeaksRecoveryFixture(t *testing.T) {
	runtime := NewRuntime()

	stats, err := runtime.RuntimeStats(context.Background(), &gamev1.Empty{})
	if err != nil {
		t.Fatalf("RuntimeStats failed: %v", err)
	}
	if got := stats.GetPhaseStatus()["contract_source"]; got != runtimeContractSourceUnconfigured {
		t.Fatalf("default runtime source = %q, want %q", got, runtimeContractSourceUnconfigured)
	}
	for _, category := range []string{
		"contracts.runtime_movement_profile",
		"contracts.base_movement_actions",
		"contracts.skill_runtime_contracts",
		"contracts.wolf_brain_policy",
		"contracts.combat_core_profiles",
		"contracts.combat_defense_contracts",
		"contracts.combat_mode_slots",
	} {
		if got := stats.GetPhaseStatus()[category]; got != "blocked" {
			t.Fatalf("default runtime coverage %s = %q, want blocked: %#v", category, got, stats.GetPhaseStatus())
		}
	}
	if got := stats.GetPhaseStatus()["contracts.required.skill.player_shield_rush"]; got != "missing" {
		t.Fatalf("default runtime leaked recovered player_shield_rush readiness: %q", got)
	}
	if got := stats.GetPhaseStatus()["contracts.required.base_movement_action.dodge"]; !strings.HasPrefix(got, "missing") {
		t.Fatalf("default runtime leaked recovered dodge readiness: %q", got)
	}

	ready, err := runtime.Readiness(context.Background(), &gamev1.Empty{})
	if err != nil {
		t.Fatalf("Readiness failed: %v", err)
	}
	if ready.GetReady() {
		t.Fatal("default runtime reported ready without DB contracts")
	}
	if !stringSliceHasPrefix(ready.GetBlockers(), "runtime_movement_profile: movement reconciliation profile") {
		t.Fatalf("default runtime readiness blockers do not expose missing movement profile: %#v", ready.GetBlockers())
	}

	session, err := runtime.OpenSession(context.Background(), &gamev1.OpenSessionRequest{
		Context: &gamev1.RequestContext{SessionId: "source_guard", AccountId: "source_guard"},
	})
	if err != nil {
		t.Fatalf("OpenSession failed: %v", err)
	}
	if len(session.GetMovementActionContracts()) != 0 {
		t.Fatalf("default runtime leaked recovered movement manifest: %#v", session.GetMovementActionContracts())
	}
	if len(session.GetMovementActionContractPayloads()) != 0 {
		t.Fatalf("default runtime leaked recovered movement payloads: %#v", session.GetMovementActionContractPayloads())
	}
}

func TestRecoveryFixtureRuntimeIsExplicitDevTestOptIn(t *testing.T) {
	contracts := RecoveryFixtureRuntimeContracts()
	if contracts.Source != runtimeContractSourceRecoveryFixture {
		t.Fatalf("recovery fixture source = %q, want %q", contracts.Source, runtimeContractSourceRecoveryFixture)
	}

	runtime := NewRuntimeWithContracts(contracts)
	stats, err := runtime.RuntimeStats(context.Background(), &gamev1.Empty{})
	if err != nil {
		t.Fatalf("RuntimeStats failed: %v", err)
	}
	if got := stats.GetPhaseStatus()["contract_source"]; got != runtimeContractSourceRecoveryFixture {
		t.Fatalf("fixture runtime source = %q, want %q", got, runtimeContractSourceRecoveryFixture)
	}
	if got := stats.GetPhaseStatus()["contracts.required.skill.player_shield_rush"]; !strings.HasPrefix(got, "ready:") {
		t.Fatalf("fixture runtime should expose recovered player_shield_rush only by explicit opt-in: %q", got)
	}
	if got := stats.GetPhaseStatus()["contracts.required.base_movement_action.dodge"]; !strings.HasPrefix(got, "ready:") {
		t.Fatalf("fixture runtime should expose recovered dodge only by explicit opt-in: %q", got)
	}

	ready, err := runtime.Readiness(context.Background(), &gamev1.Empty{})
	if err != nil {
		t.Fatalf("Readiness failed: %v", err)
	}
	if !ready.GetReady() {
		t.Fatalf("explicit recovery fixture should remain usable for dev tests: %#v", ready.GetBlockers())
	}
}

func TestDBRuntimeSourcePromotesOnlyAfterStrictCompleteLoad(t *testing.T) {
	complete := LoadRuntimeContractsFromDB(context.Background(), fakeRuntimeContractSource{}, fakeRuntimeContractSource{})
	if complete.Source != runtimeContractSourceDB {
		t.Fatalf("complete DB load source = %q, want %q; issues=%#v", complete.Source, runtimeContractSourceDB, complete.LoadIssues)
	}
	if err := complete.ValidateRequiredCoverage(true); err != nil {
		t.Fatalf("complete fake DB coverage failed: %v", err)
	}

	incomplete := LoadRuntimeContractsFromDB(
		context.Background(),
		fakeRuntimeContractSource{missingSkills: map[string]bool{"player_shield_rush": true}},
		fakeRuntimeContractSource{missingSkills: map[string]bool{"player_shield_rush": true}},
	)
	if incomplete.Source == runtimeContractSourceDB {
		t.Fatalf("incomplete DB load promoted to complete source: %#v", incomplete.LoadIssues)
	}
	if incomplete.Source != runtimeContractSourceDBIncomplete {
		t.Fatalf("incomplete DB load source = %q, want %q", incomplete.Source, runtimeContractSourceDBIncomplete)
	}
	if len(incomplete.LoadIssues) == 0 {
		t.Fatal("incomplete DB load did not report load issues")
	}
	if got := incomplete.SkillContracts["player_shield_rush"]; got.Enabled || got.MovementAction.ID != "" {
		t.Fatalf("incomplete DB load leaked recovered player_shield_rush fallback: %#v", got)
	}
	if err := incomplete.ValidateRequiredCoverage(true); err == nil {
		t.Fatal("incomplete DB load passed strict coverage")
	}
}

func TestRuntimeStatsExposeFinalAuthoritySurfaceWhileDefaultContractsRemainBlocked(t *testing.T) {
	runtime := NewRuntime()
	stats, err := runtime.RuntimeStats(context.Background(), &gamev1.Empty{})
	if err != nil {
		t.Fatalf("RuntimeStats failed: %v", err)
	}
	if got := stats.GetPhaseStatus()["contracts.surface.movement_action_contract"]; !strings.Contains(got, "final_authority") || !strings.Contains(got, "runtime_authority") {
		t.Fatalf("movement action surface status = %q", got)
	}
	if got := stats.GetPhaseStatus()["contracts.surface.skill_movement_effect/GetSkillMovementEffect"]; !strings.Contains(got, "compat_runtime_required") || strings.Contains(got, "runtime_authority") {
		t.Fatalf("legacy skill movement surface status = %q", got)
	}
	if got := stats.GetPhaseStatus()["contracts.skill_runtime_contracts"]; got != "blocked" {
		t.Fatalf("default runtime contract coverage must stay blocked despite surface classification: %q", got)
	}
}

func stringSliceHasPrefix(values []string, prefix string) bool {
	for _, value := range values {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}
