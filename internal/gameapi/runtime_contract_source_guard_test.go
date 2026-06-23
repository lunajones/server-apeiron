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

func TestDefaultRuntimeRejectsMoveInsteadOfPublishingResolverDefaults(t *testing.T) {
	runtime := NewRuntime()
	session, err := runtime.OpenSession(context.Background(), &gamev1.OpenSessionRequest{
		Context: &gamev1.RequestContext{SessionId: "default_move_guard", AccountId: "default_move_guard"},
	})
	if err != nil {
		t.Fatalf("OpenSession failed: %v", err)
	}
	if _, err := runtime.AttachPlayer(context.Background(), &gamev1.AttachPlayerRequest{
		Context:  &gamev1.RequestContext{SessionId: session.GetSessionId(), AccountId: "default_move_guard"},
		PlayerId: "default_move_guard_player",
	}); err != nil {
		t.Fatalf("AttachPlayer failed: %v", err)
	}

	player := runtime.players["default_move_guard_player"]
	start := player.position
	ack, err := runtime.SubmitCommand(context.Background(), testRuntimeMoveCommand(session.GetSessionId(), 1, gamev1Vector(1, 0, 0), 1, true, nil))
	if err != nil {
		t.Fatalf("SubmitCommand failed: %v", err)
	}
	if ack.GetAccepted() {
		t.Fatalf("default runtime accepted move without DB movement contract: %#v", ack)
	}
	if ack.GetRejectionCode() != "missing_movement_contract" {
		t.Fatalf("default runtime move rejection = %q, want missing_movement_contract", ack.GetRejectionCode())
	}
	if player.position != start {
		t.Fatalf("default runtime moved player despite missing move contract: start=%#v current=%#v", start, player.position)
	}
	if player.locomotion != nil && player.locomotion.GetAction() == "move" {
		t.Fatalf("default runtime published resolver-default move locomotion: %#v", player.locomotion)
	}
}

func TestRecoveryFixtureRuntimeAcceptsMoveOnlyThroughExplicitMoveContract(t *testing.T) {
	runtime := NewRuntimeWithContracts(RecoveryFixtureRuntimeContracts())
	session, err := runtime.OpenSession(context.Background(), &gamev1.OpenSessionRequest{
		Context: &gamev1.RequestContext{SessionId: "fixture_move_guard", AccountId: "fixture_move_guard"},
	})
	if err != nil {
		t.Fatalf("OpenSession failed: %v", err)
	}
	if _, err := runtime.AttachPlayer(context.Background(), &gamev1.AttachPlayerRequest{
		Context:  &gamev1.RequestContext{SessionId: session.GetSessionId(), AccountId: "fixture_move_guard"},
		PlayerId: "fixture_move_guard_player",
	}); err != nil {
		t.Fatalf("AttachPlayer failed: %v", err)
	}

	ack, err := runtime.SubmitCommand(context.Background(), testRuntimeMoveCommand(session.GetSessionId(), 1, gamev1Vector(1, 0, 0), 1, true, nil))
	if err != nil {
		t.Fatalf("SubmitCommand failed: %v", err)
	}
	if !ack.GetAccepted() {
		t.Fatalf("fixture runtime rejected DB-equivalent move contract: %#v", ack)
	}
	player := runtime.players["fixture_move_guard_player"]
	if player.locomotion == nil || player.locomotion.GetAbilityKey() != "move" {
		t.Fatalf("fixture runtime did not publish move locomotion from explicit contract: %#v", player.locomotion)
	}
	if player.locomotion.GetActionContractId() != "grounded_move_v1" {
		t.Fatalf("move locomotion contract id = %q, want grounded_move_v1", player.locomotion.GetActionContractId())
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
