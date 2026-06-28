package gameapi

import "testing"

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
