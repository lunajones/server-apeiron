package gameapi

import (
	"math"
	"testing"
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

	runtime.creditThreatLocked(wolf, player, 5, 0)
	start := wolf.threat.Entries[player.id]
	if start <= prof.DecayPerSec {
		t.Fatalf("test setup: starting threat %.2f must exceed one second of decay %.2f", start, prof.DecayPerSec)
	}

	runtime.decayCreatureThreatLocked(wolf, 1.0)
	if got := wolf.threat.Entries[player.id]; math.Abs(got-(start-prof.DecayPerSec)) > 0.01 {
		t.Fatalf("threat after 1s decay = %.2f, want %.2f", got, start-prof.DecayPerSec)
	}

	runtime.decayCreatureThreatLocked(wolf, 100)
	if _, ok := wolf.threat.Entries[player.id]; ok {
		t.Fatalf("threat entry not pruned after heavy decay: %#v", wolf.threat.Entries)
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
