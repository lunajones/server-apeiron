package gameapi

import (
	"math"
	"testing"
	"time"
)

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

	// Pack of one is not slotted -> tactical movement is untouched (identity).
	runtime.assignPackRingSlotsLocked(time.Now())
	if w.packSlotKnown {
		t.Fatal("pack-of-one wolf was slotted; should stay unslotted for identity")
	}
}

// TestPackSlottingSpreadsMembersAroundTarget locks Pack Slice 2: a clustered pack is assigned
// distinct ring bearings so members surround the target instead of stacking on one arc.
func TestPackSlottingSpreadsMembersAroundTarget(t *testing.T) {
	runtime := NewRuntimeWithContracts(DevFixtureRuntimeContracts())
	player := runtime.ensurePlayerLocked("local_player")
	player.position = vector{x: 0, y: 0, z: 0}

	wolves := []*entityState{
		newFixtureWolf(301, vector{x: 500, y: 0}),
		newFixtureWolf(302, vector{x: 520, y: 30}),
		newFixtureWolf(303, vector{x: 480, y: -20}),
	}
	for _, w := range wolves {
		runtime.entities[w.id] = w
	}

	runtime.formCreaturePacksLocked()
	runtime.assignPackRingSlotsLocked(time.Now())

	seen := map[int]bool{}
	for _, w := range wolves {
		if !w.packSlotKnown {
			t.Fatalf("wolf %d not slotted", w.id)
		}
		key := int(math.Round(w.packRingSlotDeg))
		if seen[key] {
			t.Fatalf("duplicate slot bearing ~%d deg (members clumped on one slot)", key)
		}
		seen[key] = true
	}
}

// TestPackSlotSteerPointsAtSlot locks the steering math: a slotted member is steered toward the
// point on the target's ring at its assigned bearing.
func TestPackSlotSteerPointsAtSlot(t *testing.T) {
	wolf := newFixtureWolf(1, vector{x: 500, y: 0})
	wolf.packRingSlotDeg = 90
	wolf.packSlotKnown = true
	target := &entityState{id: 2, entityType: "player", position: vector{x: 0, y: 0}}

	steer := packSlotSteerDirection(wolf, target)
	slotPoint := add(target.position, scale(yawVector(wolf.packRingSlotDeg), 500))
	want := normalize(vector{x: slotPoint.x - wolf.position.x, y: slotPoint.y - wolf.position.y})
	if math.Abs(steer.x-want.x) > 0.01 || math.Abs(steer.y-want.y) > 0.01 {
		t.Fatalf("steer %v does not point toward slot point dir %v", steer, want)
	}
}
