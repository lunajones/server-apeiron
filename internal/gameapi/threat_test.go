package gameapi

import (
	"math"
	"testing"
	"time"
)

// TestThreatCreditedOnHitWithPullerBonus locks Threat Slice 1 emission: a creature accrues threat
// from damage + posture applied by an attacker, and the first attacker on a fresh table earns the
// puller (first-aggro) bonus once.
func TestThreatCreditedOnHitWithPullerBonus(t *testing.T) {
	runtime := NewRuntimeWithContracts(DevFixtureRuntimeContracts())
	player := runtime.ensurePlayerLocked("local_player")
	wolf := runtime.ensureWolfLocked(player)
	prof := runtime.contracts.WolfPolicy.Threat

	runtime.creditThreatLocked(wolf, player, 10, 5)
	if wolf.threat == nil {
		t.Fatal("threat table not created on first hit")
	}
	want := 10*prof.DamageThreatPerPoint + 5*prof.PostureThreatPerPoint + prof.FirstAggroBonus
	if got := wolf.threat.Entries[player.id]; math.Abs(got-want) > 0.01 {
		t.Fatalf("threat after first hit = %.2f, want %.2f (incl. puller bonus)", got, want)
	}

	before := wolf.threat.Entries[player.id]
	runtime.creditThreatLocked(wolf, player, 10, 0)
	wantSecond := before + 10*prof.DamageThreatPerPoint
	if got := wolf.threat.Entries[player.id]; math.Abs(got-wantSecond) > 0.01 {
		t.Fatalf("threat after second hit = %.2f, want %.2f (no second puller bonus)", got, wantSecond)
	}
}

// TestThreatDecaysAndPrunes locks Slice 1 decay: threat falls by decay_per_sec and entries that
// reach zero are pruned so the table cannot grow unbounded.
func TestThreatDecaysAndPrunes(t *testing.T) {
	runtime := NewRuntimeWithContracts(DevFixtureRuntimeContracts())
	player := runtime.ensurePlayerLocked("local_player")
	wolf := runtime.ensureWolfLocked(player)
	prof := runtime.contracts.WolfPolicy.Threat

	// Disengaged: player well outside proximity range so threat decays (faster, out-of-range).
	wolf.position = vector{x: 0, y: 0, z: 0}
	player.position = vector{x: prof.ProximityRangeCM * 10, y: 0, z: 0}
	decayRate := prof.DecayPerSec * prof.OutOfRangeDecayMultiplier

	runtime.creditThreatLocked(wolf, player, 5, 0)
	start := wolf.threat.Entries[player.id]
	if start <= decayRate {
		t.Fatalf("test setup: starting threat %.2f must exceed one second of decay %.2f", start, decayRate)
	}

	runtime.decayCreatureThreatLocked(wolf, 1.0)
	if got := wolf.threat.Entries[player.id]; math.Abs(got-(start-decayRate)) > 0.01 {
		t.Fatalf("threat after 1s out-of-range decay = %.2f, want %.2f", got, start-decayRate)
	}

	runtime.decayCreatureThreatLocked(wolf, 100)
	if _, ok := wolf.threat.Entries[player.id]; ok {
		t.Fatalf("threat entry not pruned after heavy decay: %#v", wolf.threat.Entries)
	}
}

// TestThreatProximityEngagesLoiterer locks Slice 3: a target standing in the creature's face
// accrues threat from proximity and, while engaged (in range), does not decay away.
func TestThreatProximityEngagesLoiterer(t *testing.T) {
	runtime := NewRuntimeWithContracts(DevFixtureRuntimeContracts())
	player := runtime.ensurePlayerLocked("local_player")
	wolf := runtime.ensureWolfLocked(player)
	prof := runtime.contracts.WolfPolicy.Threat

	wolf.position = vector{x: 0, y: 0, z: 0}
	player.position = vector{x: prof.ProximityRangeCM * 0.5, y: 0, z: 0}

	runtime.accrueProximityThreatLocked(wolf, 1.0)
	if wolf.threat == nil || wolf.threat.Entries[player.id] <= 0 {
		t.Fatal("proximity did not accrue threat for an in-range loiterer")
	}

	before := wolf.threat.Entries[player.id]
	runtime.decayCreatureThreatLocked(wolf, 1.0)
	if got := wolf.threat.Entries[player.id]; got < before {
		t.Fatalf("in-range (engaged) threat decayed %.2f -> %.2f; should hold", before, got)
	}
}

// TestThreatOnlyCreaturesAccumulate locks that threat is creature-side only — a player struck by a
// wolf does not build a threat table (threat is "who is hurting me", kept by the creature).
func TestThreatOnlyCreaturesAccumulate(t *testing.T) {
	runtime := NewRuntimeWithContracts(DevFixtureRuntimeContracts())
	player := runtime.ensurePlayerLocked("local_player")
	wolf := runtime.ensureWolfLocked(player)

	runtime.creditThreatLocked(player, wolf, 10, 5)
	if player.threat != nil && len(player.threat.Entries) > 0 {
		t.Fatalf("player accumulated a threat table: %#v", player.threat)
	}
}

// TestThreatTargetSelectionSinglePlayerNoRegression locks the Slice 2 guarantee: with one
// candidate, target selection returns exactly that player, so behavior is unchanged.
func TestThreatTargetSelectionSinglePlayerNoRegression(t *testing.T) {
	runtime := NewRuntimeWithContracts(DevFixtureRuntimeContracts())
	player := runtime.ensurePlayerLocked("local_player")
	wolf := runtime.ensureWolfLocked(player)
	if got := runtime.resolveCreatureTargetLocked(wolf, time.Now()); got != player {
		t.Fatalf("single-player target = %v, want the only player", got)
	}
}

// TestThreatTargetSelectionPicksHigherThreatWithHysteresis locks Slice 2 multi-target selection:
// the wolf targets the higher-threat player, does not flip-flop within the switch cooldown, and
// switches once a challenger clearly out-threatens it and the cooldown has elapsed.
func TestThreatTargetSelectionPicksHigherThreatWithHysteresis(t *testing.T) {
	runtime := NewRuntimeWithContracts(DevFixtureRuntimeContracts())
	p1 := runtime.ensurePlayerLocked("p1")
	p2 := runtime.ensurePlayerLocked("p2")
	wolf := runtime.ensureWolfLocked(p1)
	prof := runtime.contracts.WolfPolicy.Threat
	now := time.Now()
	cooldownPast := now.Add(time.Duration(prof.SwitchCooldownMS+50) * time.Millisecond)

	runtime.creditThreatLocked(wolf, p1, 100, 0)
	runtime.creditThreatLocked(wolf, p2, 5, 0)
	if got := runtime.resolveCreatureTargetLocked(wolf, now); got != p1 {
		t.Fatalf("initial target = %v, want p1 (highest threat)", got)
	}

	// p2 surpasses p1 but the switch cooldown has not elapsed -> stick to p1.
	runtime.creditThreatLocked(wolf, p2, 1000, 0)
	if got := runtime.resolveCreatureTargetLocked(wolf, now); got != p1 {
		t.Fatalf("flip-flopped to %v within switch cooldown; want stay p1", got)
	}

	// Cooldown elapsed and p2 clearly out-threatens p1 -> switch to p2.
	if got := runtime.resolveCreatureTargetLocked(wolf, cooldownPast); got != p2 {
		t.Fatalf("did not switch to p2 after clear threat + cooldown; got %v", got)
	}
}

// TestThreatLeashResetsWhenPulledTooFar locks Slice 4: pulled beyond leash distance the creature
// wipes threat, walks home, and resets (health restored) once it arrives.
func TestThreatLeashResetsWhenPulledTooFar(t *testing.T) {
	runtime := NewRuntimeWithContracts(DevFixtureRuntimeContracts())
	player := runtime.ensurePlayerLocked("local_player")
	wolf := runtime.ensureWolfLocked(player)
	prof := runtime.contracts.WolfPolicy.Threat
	now := time.Now()

	// Anchor the wolf at origin (first call sets the anchor), give it threat + damage.
	wolf.position = vector{x: 0, y: 0, z: 0}
	if runtime.updateCreatureLeashLocked(wolf, now) {
		t.Fatal("wolf leashed at its own anchor")
	}
	runtime.creditThreatLocked(wolf, player, 100, 0)
	wolf.health = 10

	// Yank it well past the leash distance -> disengage + wipe + start returning.
	wolf.position = vector{x: prof.LeashDistanceCM + 600, y: 0, z: 0}
	if !runtime.updateCreatureLeashLocked(wolf, now) {
		t.Fatal("wolf did not leash when pulled beyond leash distance")
	}
	if !wolf.creatureLeashed {
		t.Fatal("wolf not marked leashed")
	}
	if len(wolf.threat.Entries) != 0 || wolf.threat.CurrentTarget != 0 {
		t.Fatalf("leash did not wipe threat: %#v", wolf.threat)
	}
	if wolf.position.x >= prof.LeashDistanceCM+600 {
		t.Fatal("leashed wolf did not step toward home")
	}

	// Arrive home -> reset clears the leash and restores health.
	wolf.position = vector{x: 0, y: 0, z: 0}
	if runtime.updateCreatureLeashLocked(wolf, now) {
		t.Fatal("wolf still leashing after reaching home")
	}
	if wolf.creatureLeashed {
		t.Fatal("leashed flag not cleared at home")
	}
	if wolf.health != wolf.maxHealth {
		t.Fatalf("leash reset did not restore health: %.1f/%.1f", wolf.health, wolf.maxHealth)
	}
}
