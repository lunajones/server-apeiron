package gameapi

import "testing"

func newFixtureWolf(id uint64, pos vector) *entityState {
	return &entityState{
		id:         id,
		entityType: "creature",
		templateID: "steppe_wolf",
		position:   pos,
		health:     160,
		maxHealth:  160,
	}
}

// TestPackFormationGroupsNearbyAndSolosIdentity locks Pack Slice 1: creatures within join radius
// share one pack id; a far creature forms its own pack of one. Membership only — no behavior
// change, so a lone creature is identical to today.
func TestPackFormationGroupsNearbyAndSolosIdentity(t *testing.T) {
	runtime := NewRuntimeWithContracts(DevFixtureRuntimeContracts())
	join := runtime.contracts.WolfPolicy.Pack.JoinRadiusCM
	if join <= 0 {
		t.Fatal("fixture pack join radius not set")
	}

	w1 := newFixtureWolf(101, vector{x: 0, y: 0})
	w2 := newFixtureWolf(102, vector{x: join * 0.5, y: 0}) // within join radius of w1
	w3 := newFixtureWolf(103, vector{x: join * 20, y: 0})  // far away -> its own pack
	runtime.entities[w1.id] = w1
	runtime.entities[w2.id] = w2
	runtime.entities[w3.id] = w3

	runtime.formCreaturePacksLocked()

	if w1.packID == "" || w2.packID == "" || w3.packID == "" {
		t.Fatalf("pack ids not assigned: w1=%q w2=%q w3=%q", w1.packID, w2.packID, w3.packID)
	}
	if w1.packID != w2.packID {
		t.Fatalf("nearby wolves not grouped: w1=%q w2=%q", w1.packID, w2.packID)
	}
	if w3.packID == w1.packID {
		t.Fatalf("far wolf joined the near pack: w3=%q", w3.packID)
	}

	near := runtime.packs[w1.packID]
	if near == nil || len(near.MemberIDs) != 2 {
		t.Fatalf("near pack membership = %#v, want 2 members", near)
	}
	solo := runtime.packs[w3.packID]
	if solo == nil || len(solo.MemberIDs) != 1 || solo.MemberIDs[0] != w3.id {
		t.Fatalf("solo pack-of-one membership = %#v, want [%d]", solo, w3.id)
	}
}

// TestPackOfOneIsIdentity locks that a single creature forms a pack of one keyed on itself.
func TestPackOfOneIsIdentity(t *testing.T) {
	runtime := NewRuntimeWithContracts(DevFixtureRuntimeContracts())
	w := newFixtureWolf(201, vector{x: 0, y: 0})
	runtime.entities[w.id] = w

	runtime.formCreaturePacksLocked()

	if w.packID == "" {
		t.Fatal("lone wolf got no pack id")
	}
	pack := runtime.packs[w.packID]
	if pack == nil || len(pack.MemberIDs) != 1 || pack.MemberIDs[0] != w.id {
		t.Fatalf("pack-of-one = %#v, want a single self-membership", pack)
	}
}
