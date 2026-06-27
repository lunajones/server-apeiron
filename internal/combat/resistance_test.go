package combat

import (
	"math"
	"testing"

	apeironv1 "db-apeiron/gen/apeiron/v1"
)

// TestResistanceMitigatesByFamily locks Slice 1: a hit is reduced by the target's resistance for
// the damage's family (rating/(rating+K) curve), and a different family with no rating is not.
func TestResistanceMitigatesByFamily(t *testing.T) {
	mitigationKValue = 100 // deterministic K for the test

	target := &apeironv1.CombatCoreProfile{
		PhysicalResistanceRating: 100, // K=100 -> 100/(100+100) = 50% reduction
		ChemicalResistanceRating: 0,
		ResistanceCap:            0.85,
	}

	if got := applyResistanceMitigation(100, &apeironv1.Skill{DamageType: "blunt"}, target); math.Abs(got-50) > 0.5 {
		t.Fatalf("blunt (physical) mitigated = %.1f, want ~50", got)
	}
	if got := applyResistanceMitigation(100, &apeironv1.Skill{DamageType: "fire"}, target); math.Abs(got-100) > 0.01 {
		t.Fatalf("fire (chemical, 0 rating) mitigated = %.1f, want 100", got)
	}
}

// TestResistanceCapNeverFullImmunity locks that a huge rating is capped (no stat-only immunity).
func TestResistanceCapNeverFullImmunity(t *testing.T) {
	mitigationKValue = 100
	target := &apeironv1.CombatCoreProfile{PhysicalResistanceRating: 1_000_000, ResistanceCap: 0.85}
	got := applyResistanceMitigation(100, &apeironv1.Skill{DamageType: "blunt"}, target)
	if math.Abs(got-15) > 0.5 {
		t.Fatalf("capped mitigation = %.1f, want ~15 (cap 0.85)", got)
	}
}

// TestDamageFamilyMappingAndFallback locks the type->family map incl. legacy 'physical' and the
// unknown-type fallback to the Physical family (fail-safe, never zero/panic).
func TestDamageFamilyMappingAndFallback(t *testing.T) {
	cases := map[string]string{
		"slashing": familyPhysical, "piercing": familyPhysical, "blunt": familyPhysical,
		"fire": familyChemical, "corrosive": familyChemical,
		"poison": familyBiological, "bleed": familyBiological, "trauma": familyBiological,
		"physical": familyPhysical, // legacy default
		"":         familyPhysical, // empty
		"made_up":  familyPhysical, // unknown -> physical fallback
	}
	for damageType, want := range cases {
		if got := damageFamilyOf(damageType); got != want {
			t.Fatalf("damageFamilyOf(%q) = %q, want %q", damageType, got, want)
		}
	}
}
