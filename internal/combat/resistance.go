package combat

import (
	"os"
	"strconv"
	"strings"

	apeironv1 "db-apeiron/gen/apeiron/v1"
)

// Damage families. Each damage type maps to exactly one family, mitigated by that family's
// resistance rating. See server-apeiron/docs/roadmap/aaa-damage-types-resistances-weapons-roadmap.md.
const (
	familyPhysical   = "physical"
	familyChemical   = "chemical"
	familyBiological = "biological"
)

// damageFamilyOf maps a skill damage type to its mitigation family. Legacy `physical` and any
// unknown/unmapped type fall back to the Physical family (fail-safe: never zero damage or panic).
func damageFamilyOf(damageType string) string {
	switch strings.ToLower(strings.TrimSpace(damageType)) {
	case "fire", "corrosive":
		return familyChemical
	case "poison", "bleed", "trauma":
		return familyBiological
	case "slashing", "piercing", "blunt", "physical", "":
		return familyPhysical
	default:
		return familyPhysical
	}
}

func familyResistanceRating(core *apeironv1.CombatCoreProfile, family string) float64 {
	if core == nil {
		return 0
	}
	switch family {
	case familyChemical:
		return core.GetChemicalResistanceRating()
	case familyBiological:
		return core.GetBiologicalResistanceRating()
	default:
		return core.GetPhysicalResistanceRating()
	}
}

// defaultResistanceCap caps the mitigation curve below 100% so nothing is fully immune by stat
// alone, when a profile does not specify its own cap.
const defaultResistanceCap = 0.85

// mitigationKValue is the diminishing-returns curve constant (gear-treadmill knob). It scales per
// attacker tier later; for now it is one global value from MITIGATION_K (default 100).
var mitigationKValue = loadMitigationK()

func loadMitigationK() float64 {
	if v := strings.TrimSpace(os.Getenv("MITIGATION_K")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			return f
		}
	}
	return 100
}

// applyResistanceMitigation reduces a damage amount by the target's resistance for the skill's
// damage family, using the diminishing-returns curve rating/(rating+K), capped. (Armor penetration
// is wired in Slice 2 once it is surfaced on the skill proto.)
func applyResistanceMitigation(damageAmount float64, skill *apeironv1.Skill, targetCore *apeironv1.CombatCoreProfile) float64 {
	if damageAmount <= 0 || skill == nil || targetCore == nil {
		return damageAmount
	}
	rating := familyResistanceRating(targetCore, damageFamilyOf(skill.GetDamageType()))
	if rating <= 0 {
		return damageAmount
	}
	k := mitigationKValue
	if k <= 0 {
		k = 100
	}
	reduction := rating / (rating + k)
	cap := targetCore.GetResistanceCap()
	if cap <= 0 {
		cap = defaultResistanceCap
	}
	if reduction > cap {
		reduction = cap
	}
	return damageAmount * (1 - reduction)
}
