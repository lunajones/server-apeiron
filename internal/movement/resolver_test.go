package movement

import (
	"testing"
	"time"
)

func TestResolverPhaseWindows(t *testing.T) {
	r := NewResolver()
	// startup 100ms, active 200ms, recovery 100ms
	cases := []struct {
		name      string
		elapsed   time.Duration
		wantPhase string
	}{
		{"startup", 50 * time.Millisecond, "startup"},
		{"active", 150 * time.Millisecond, "active"},
		{"recovery", 320 * time.Millisecond, "recovery"},
		{"past-end clamps to recovery", 500 * time.Millisecond, "recovery"},
	}
	for _, tc := range cases {
		phase, _, _ := r.ResolvePhase(tc.elapsed, 100, 200, 100)
		if phase != tc.wantPhase {
			t.Fatalf("%s: phase = %q, want %q", tc.name, phase, tc.wantPhase)
		}
	}
}

func TestResolvePhaseElapsedRemaining(t *testing.T) {
	r := NewResolver()
	phase, elapsed, remaining := r.ResolvePhase(150*time.Millisecond, 100, 200, 100)
	if phase != "active" {
		t.Fatalf("phase = %q, want active", phase)
	}
	if elapsed != 50 {
		t.Fatalf("active elapsed = %d, want 50", elapsed)
	}
	if remaining != 150 {
		t.Fatalf("active remaining = %d, want 150", remaining)
	}
}

// TestResolverSkillNormalParity is the anti-rubberband contract: skill movement and
// normal movement that share an authoritative contract must publish IDENTICAL
// locomotion policy. If any of these diverge, the client reconciles the skill
// differently than a walk and rubberbands.
func TestResolverSkillNormalParity(t *testing.T) {
	r := NewResolver()
	contract := MovementActionContract{
		ID:                   "player_shield_rush",
		MovementAction:       "shield_rush",
		MovementType:         "grounded_dash",
		ReconciliationMode:   "skill_grounded_action",
		DurationMS:           240,
		StartupMS:            40,
		ActiveMS:             140,
		RecoveryMS:           60,
		HorizontalDistanceCM: 320,
		BaseSpeedCMPerSec:    1333,
	}

	normal := r.Resolve(LocomotionInput{MovementMode: "grounded", Contract: contract, Phase: "active"})
	skill := r.Resolve(LocomotionInput{MovementMode: "skill", Contract: contract, Phase: "active"})

	if normal.ReconciliationMode != skill.ReconciliationMode {
		t.Fatalf("reconciliation diverged: normal=%q skill=%q", normal.ReconciliationMode, skill.ReconciliationMode)
	}
	if normal.ActionDistanceTraveled != skill.ActionDistanceTraveled {
		t.Fatalf("distance diverged: normal=%v skill=%v", normal.ActionDistanceTraveled, skill.ActionDistanceTraveled)
	}
	if normal.TargetSpeed != skill.TargetSpeed || normal.EffectiveSpeed != skill.EffectiveSpeed {
		t.Fatalf("speed diverged: normal=%v/%v skill=%v/%v", normal.TargetSpeed, normal.EffectiveSpeed, skill.TargetSpeed, skill.EffectiveSpeed)
	}
	if normal.DurationMS != skill.DurationMS || normal.ActiveMS != skill.ActiveMS || normal.RecoveryMS != skill.RecoveryMS {
		t.Fatalf("timing diverged")
	}
}

// TestResolveFullyPopulates guards that the resolver never emits a "blank" locomotion
// for a real contract — empty reconciliation/distance is exactly what the client reads
// as a generic correction (the rubberband).
func TestResolveFullyPopulates(t *testing.T) {
	r := NewResolver()
	got := r.Resolve(LocomotionInput{
		AbilityKey: "shield_rush",
		Contract: MovementActionContract{
			ID:                   "player_shield_rush",
			MovementAction:       "shield_rush",
			HorizontalDistanceCM: 320,
			BaseSpeedCMPerSec:    1333,
		},
		Elapsed: 10 * time.Millisecond,
	})
	if got.ReconciliationMode == "" {
		t.Fatal("reconciliation mode must default, never empty")
	}
	if got.ActionDistanceTraveled <= 0 {
		t.Fatal("distance must come from the contract")
	}
	if got.AbilityKey != "shield_rush" {
		t.Fatalf("ability key = %q, want shield_rush", got.AbilityKey)
	}
	if got.Phase == "" {
		t.Fatal("phase must be computed from elapsed when not provided")
	}
}

// TestResolveRuntimeMatchesHistoricalDefaults locks ResolveRuntime to the exact policy
// that gameapi.locomotionFromContract used inline before delegation, so wiring the
// runtime onto the resolver does not change published normal-movement locomotion.
func TestResolveRuntimeMatchesHistoricalDefaults(t *testing.T) {
	r := NewResolver()

	empty := r.ResolveRuntime(RuntimeActionContract{}, "active")
	if empty.ReconciliationMode != "grounded_move_reconciliation" {
		t.Fatalf("reconciliation default = %q", empty.ReconciliationMode)
	}
	if empty.DurationMS != 180 || empty.ActiveMS != 120 || empty.RecoveryMS != 60 {
		t.Fatalf("timing defaults = %d/%d/%d", empty.DurationMS, empty.ActiveMS, empty.RecoveryMS)
	}
	if empty.PhaseWindowPolicy != "server_authoritative" {
		t.Fatalf("phase window default = %q", empty.PhaseWindowPolicy)
	}
	if empty.PredictionErrorPolicy != "bounded_smooth_correction" {
		t.Fatalf("prediction default = %q", empty.PredictionErrorPolicy)
	}
	if empty.MovementMode != "grounded" {
		t.Fatalf("movement mode default = %q", empty.MovementMode)
	}

	dodge := r.ResolveRuntime(RuntimeActionContract{
		ID: "dodge_v1_full_iframe", ActionType: "dodge", AbilityKey: "dodge",
		DurationMS: 320, ActiveMS: 260, RecoveryMS: 60, DistanceCM: 260,
		BaseSpeedCMS: 812, ReconciliationCategory: "dodge_reconciliation",
	}, "active")
	if dodge.ReconciliationMode != "dodge_reconciliation" {
		t.Fatalf("reconciliation = %q", dodge.ReconciliationMode)
	}
	if dodge.ActionDistanceTraveled != 260 || dodge.TargetSpeed != 812 || dodge.EffectiveSpeed != 812 {
		t.Fatalf("distance/speed = %v/%v/%v", dodge.ActionDistanceTraveled, dodge.TargetSpeed, dodge.EffectiveSpeed)
	}
	if dodge.Action != "dodge" || dodge.AbilityKey != "dodge" || dodge.ActionContractID != "dodge_v1_full_iframe" {
		t.Fatalf("identity fields wrong: %q/%q/%q", dodge.Action, dodge.AbilityKey, dodge.ActionContractID)
	}
}
