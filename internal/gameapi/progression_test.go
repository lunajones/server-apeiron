package gameapi

import (
	"math"
	"testing"

	gamev1 "server-apeiron/gen/apeiron/game/v1"
)

// TestAllocateAttributeSpendsPointsAndScales locks the attribute-spend loop: spending points raises the
// attribute, recomputes derived stats (max health), and fails when there are no points left.
func TestAllocateAttributeSpendsPointsAndScales(t *testing.T) {
	runtime := NewRuntimeWithOptions(DevFixtureRuntimeContracts(), RuntimeOptions{DisableCreatures: true})
	player := runtime.ensurePlayerLocked("p1")
	player.progression.attributePoints = 5

	cmd := &gamev1.PlayerCommand{
		Type: gamev1.CommandType_COMMAND_TYPE_ALLOCATE_ATTRIBUTE,
		Payload: &gamev1.PlayerCommand_AllocateAttribute{
			AllocateAttribute: &gamev1.AllocateAttributeCommand{Attribute: "resilience", Amount: 5},
		},
	}
	if ok, code, _ := runtime.allocatePlayerAttributeLocked(player, cmd); !ok {
		t.Fatalf("allocate failed: %s", code)
	}
	if player.progression.resilience != 6 || player.progression.attributePoints != 0 {
		t.Fatalf("resilience/points = %.0f/%d, want 6/0", player.progression.resilience, player.progression.attributePoints)
	}
	if player.maxHealth != 150 {
		t.Fatalf("maxHealth = %.0f, want 150 (resilience 6)", player.maxHealth)
	}
	if ok, code, _ := runtime.allocatePlayerAttributeLocked(player, cmd); ok || code != "insufficient_attribute_points" {
		t.Fatalf("expected insufficient_attribute_points, got ok=%v code=%s", ok, code)
	}
}

// TestAttributesScaleDerivedCombatStats locks Progression Slice 5: Muscles add outgoing physical damage,
// Resilience adds max health + physical resistance, additively over the base (base attribute 1.0 = none).
func TestAttributesScaleDerivedCombatStats(t *testing.T) {
	runtime := NewRuntimeWithOptions(DevFixtureRuntimeContracts(), RuntimeOptions{DisableCreatures: true})
	player := runtime.ensurePlayerLocked("p1")

	if got := attributeMaxHealth(player.progression); got != 100 {
		t.Fatalf("base max health = %.0f, want 100", got)
	}
	if got := attributePhysicalDamageMultiplier(player.progression); math.Abs(got-1) > 1e-9 {
		t.Fatalf("base dmg mult = %.4f, want 1", got)
	}
	if got := attributePhysicalResistanceBonus(player.progression); got != 0 {
		t.Fatalf("base resist bonus = %.1f, want 0", got)
	}

	// Muscles 6 → +25% physical damage (0.05/pt). Resilience 6 → +50 HP (10/pt) + 10 physical resist (2/pt).
	player.progression.muscles = 6
	player.progression.resilience = 6
	runtime.applyAttributeDerivedStatsLocked(player)
	if player.maxHealth != 150 || player.health != 150 {
		t.Fatalf("resilience6 health = %.0f/%.0f, want 150/150", player.health, player.maxHealth)
	}
	if got := attributePhysicalDamageMultiplier(player.progression); math.Abs(got-1.25) > 1e-9 {
		t.Fatalf("muscles6 dmg mult = %.4f, want 1.25", got)
	}
	if got := attributePhysicalResistanceBonus(player.progression); got != 10 {
		t.Fatalf("resilience6 resist bonus = %.1f, want 10", got)
	}
	if cp := runtime.runtimeCombatCoreProfile(player); cp != nil {
		if base := runtime.contracts.combatCoreProfileForEntity(player); base != nil {
			if delta := cp.GetPhysicalResistanceRating() - base.GetPhysicalResistanceRating(); math.Abs(delta-10) > 1e-9 {
				t.Fatalf("resolved resistance bonus = %.1f, want 10", delta)
			}
		}
	}
}

// TestKillingCreatureAwardsLevelXP locks Progression Slice 2: killing a creature credits the killer
// level XP equal to the creature's experience_value, and despawns the creature.
func TestKillingCreatureAwardsLevelXP(t *testing.T) {
	runtime := NewRuntimeWithOptions(DevFixtureRuntimeContracts(), RuntimeOptions{DisableCreatures: true})
	runtime.contracts.WolfPolicy.Progression = CreatureProgressionProfile{ExperienceValue: 100}

	player := runtime.ensurePlayerLocked("p1")
	wolf := runtime.ensureWolfLocked(player)

	runtime.creditDamageLocked(wolf, player, 50)
	wolf.health = 0
	runtime.awardKillXPLocked(wolf)

	if player.progression.experience != 100 {
		t.Fatalf("xp = %d, want 100", player.progression.experience)
	}
	if _, alive := runtime.entities[wolf.id]; alive {
		t.Fatal("wolf should be despawned after death")
	}
}

// TestKillXPSplitsByDamageContribution locks the contribution split: the experience_value pool divides
// across damage contributors by their share, never multiplying it.
func TestKillXPSplitsByDamageContribution(t *testing.T) {
	runtime := NewRuntimeWithOptions(DevFixtureRuntimeContracts(), RuntimeOptions{DisableCreatures: true})
	runtime.contracts.WolfPolicy.Progression = CreatureProgressionProfile{ExperienceValue: 100}

	p1 := runtime.ensurePlayerLocked("p1")
	p2 := runtime.ensurePlayerLocked("p2")
	wolf := runtime.ensureWolfLocked(p1)

	runtime.creditDamageLocked(wolf, p1, 75)
	runtime.creditDamageLocked(wolf, p2, 25)
	wolf.health = 0
	runtime.awardKillXPLocked(wolf)

	if p1.progression.experience != 75 {
		t.Fatalf("p1 xp = %d, want 75", p1.progression.experience)
	}
	if p2.progression.experience != 25 {
		t.Fatalf("p2 xp = %d, want 25", p2.progression.experience)
	}
}

// TestSnapshotPublishesPlayerProgression locks Progression Slice 6: the player snapshot carries
// level/xp/attributes/points/coin plus the XP-bar fields for the HUD.
func TestSnapshotPublishesPlayerProgression(t *testing.T) {
	runtime := NewRuntimeWithOptions(DevFixtureRuntimeContracts(), RuntimeOptions{DisableCreatures: true})
	player := runtime.ensurePlayerLocked("p1")
	player.progression.level = 2
	player.progression.experience = 1500
	player.progression.attributePoints = 3
	player.progression.muscles = 5

	pp := player.snapshot(runtime.contracts).GetPlayerProgression()
	if pp == nil {
		t.Fatal("player progression missing from snapshot")
	}
	if pp.GetLevel() != 2 || pp.GetExperience() != 1500 || pp.GetAttributePoints() != 3 || pp.GetMuscles() != 5 {
		t.Fatalf("snapshot = lvl %d xp %d pts %d muscles %.0f", pp.GetLevel(), pp.GetExperience(), pp.GetAttributePoints(), pp.GetMuscles())
	}
	// Level 2 spans 1200..2800, so exp 1500 is 300 into a 1600 band.
	if pp.GetExperienceIntoLevel() != 300 || pp.GetExperienceForNextLevel() != 1600 {
		t.Fatalf("xp bar = %d/%d, want 300/1600", pp.GetExperienceIntoLevel(), pp.GetExperienceForNextLevel())
	}
	if pp.GetLevelCap() != 10 {
		t.Fatalf("cap = %d, want 10", pp.GetLevelCap())
	}
}

// TestCreatureSnapshotHasNoProgression ensures only players carry progression in the snapshot.
func TestCreatureSnapshotHasNoProgression(t *testing.T) {
	runtime := NewRuntimeWithOptions(DevFixtureRuntimeContracts(), RuntimeOptions{DisableCreatures: true})
	player := runtime.ensurePlayerLocked("p1")
	wolf := runtime.ensureWolfLocked(player)
	if wolf.snapshot(runtime.contracts).GetPlayerProgression() != nil {
		t.Fatal("creature snapshot should not carry player progression")
	}
}

// TestExperienceLevelsUpAndGrantsAttributePoints locks Progression Slice 4: cumulative experience
// crossing thresholds raises level (+3 attribute points each), up to the v1 cap of 10.
func TestExperienceLevelsUpAndGrantsAttributePoints(t *testing.T) {
	runtime := NewRuntimeWithOptions(DevFixtureRuntimeContracts(), RuntimeOptions{DisableCreatures: true})
	player := runtime.ensurePlayerLocked("p1")

	player.progression.experience = 1200
	runtime.applyLevelProgressionLocked(player)
	if player.progression.level != 2 || player.progression.attributePoints != 3 {
		t.Fatalf("level/points = %d/%d, want 2/3", player.progression.level, player.progression.attributePoints)
	}

	player.progression.experience = 33700
	runtime.applyLevelProgressionLocked(player)
	if player.progression.level != 10 || player.progression.attributePoints != 27 {
		t.Fatalf("level/points = %d/%d, want 10/27", player.progression.level, player.progression.attributePoints)
	}

	player.progression.experience = 10_000_000
	runtime.applyLevelProgressionLocked(player)
	if player.progression.level != 10 || player.progression.attributePoints != 27 {
		t.Fatalf("over-cap level/points = %d/%d, want 10/27 (capped)", player.progression.level, player.progression.attributePoints)
	}
}

// TestNoXPWithoutExperienceValue ensures a creature with no progression payout grants nothing (and
// still despawns), so unconfigured creatures never leak XP.
func TestNoXPWithoutExperienceValue(t *testing.T) {
	runtime := NewRuntimeWithOptions(DevFixtureRuntimeContracts(), RuntimeOptions{DisableCreatures: true})
	runtime.contracts.WolfPolicy.Progression = CreatureProgressionProfile{}

	player := runtime.ensurePlayerLocked("p1")
	wolf := runtime.ensureWolfLocked(player)

	runtime.creditDamageLocked(wolf, player, 50)
	wolf.health = 0
	runtime.awardKillXPLocked(wolf)

	if player.progression.experience != 0 {
		t.Fatalf("xp = %d, want 0", player.progression.experience)
	}
}
