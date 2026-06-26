package gameapi

import (
	"math"
	"testing"
	"time"

	creatureai "server-apeiron/internal/ai"
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

// TestPackCommitBudgetGatesSimultaneousCommits locks Pack Slice 3: with max_committed_members=1,
// once one member is committing the other is denied a commit (and falls back to tactical), while
// the committing member keeps its token.
func TestPackCommitBudgetGatesSimultaneousCommits(t *testing.T) {
	runtime := NewRuntimeWithContracts(DevFixtureRuntimeContracts())
	player := runtime.ensurePlayerLocked("local_player")
	player.position = vector{x: 0, y: 0, z: player.position.z}
	w1 := newFixtureWolf(401, vector{x: 300, y: 0})
	w2 := newFixtureWolf(402, vector{x: 320, y: 20})
	runtime.entities[w1.id] = w1
	runtime.entities[w2.id] = w2
	runtime.formCreaturePacksLocked()
	if w1.packID != w2.packID {
		t.Fatal("wolves not grouped into one pack")
	}

	if !runtime.packMayCommitLocked(w1) || !runtime.packMayCommitLocked(w2) {
		t.Fatal("commit denied while the budget is free")
	}

	lunge := runtime.contracts.skillContract("lunge")
	inst := runtime.newCreatureActionInstance(w1, "lunge", lunge, w1.position, time.Now())
	w1.actionInstance = &inst
	if !runtime.isCommittingSkill("lunge") || !runtime.creatureIsCommittingLocked(w1) {
		t.Fatal("lunge/committing not detected")
	}
	if !runtime.packMayCommitLocked(w1) {
		t.Fatal("committing member denied its own commit")
	}
	if runtime.packMayCommitLocked(w2) {
		t.Fatal("budget exceeded: 2nd member allowed to commit while one is committing")
	}

	out := suppressCommitDecision(creatureai.Decision{Action: "lunge", SelectedSkill: "lunge", MovementTactic: "circle"})
	if creatureai.PublishesSkill(out.Action) || out.SelectedSkill != "" {
		t.Fatalf("suppressed commit still publishes a skill: %#v", out)
	}
}

// TestPackOfOneAlwaysMayCommit locks that a solo creature is never gated by the budget (identity).
func TestPackOfOneAlwaysMayCommit(t *testing.T) {
	runtime := NewRuntimeWithContracts(DevFixtureRuntimeContracts())
	runtime.ensurePlayerLocked("local_player")
	w := newFixtureWolf(501, vector{x: 0, y: 0})
	runtime.entities[w.id] = w
	runtime.formCreaturePacksLocked()
	if !runtime.packMayCommitLocked(w) {
		t.Fatal("pack-of-one wolf denied commit; identity broken")
	}
}

// TestPackCommitRotationYieldsTurn locks Pack Slice 4 rotation: a member that just committed
// yields its next turn while a fresh member is free, so commits rotate around the pack.
func TestPackCommitRotationYieldsTurn(t *testing.T) {
	runtime := NewRuntimeWithContracts(DevFixtureRuntimeContracts())
	runtime.ensurePlayerLocked("local_player")
	w1 := newFixtureWolf(601, vector{x: 300, y: 0})
	w2 := newFixtureWolf(602, vector{x: 320, y: 20})
	runtime.entities[w1.id] = w1
	runtime.entities[w2.id] = w2
	runtime.formCreaturePacksLocked()

	// w1 just finished a commit (cooldown active); w2 is fresh. Neither is committing now.
	w1.lastCommitAt = time.Now()
	if runtime.packMayCommitLocked(w1) {
		t.Fatal("w1 should yield its turn right after committing while w2 is free")
	}
	if !runtime.packMayCommitLocked(w2) {
		t.Fatal("w2 (fresh) should be able to take the rotated turn")
	}
}

// TestPackRolesReflectCommitState locks Slice 4 role labels: committing -> engager, recently
// committed -> recoverer, otherwise harasser.
func TestPackRolesReflectCommitState(t *testing.T) {
	runtime := NewRuntimeWithContracts(DevFixtureRuntimeContracts())
	runtime.ensurePlayerLocked("local_player")
	engager := newFixtureWolf(701, vector{x: 300, y: 0})
	recoverer := newFixtureWolf(702, vector{x: 320, y: 20})
	harasser := newFixtureWolf(703, vector{x: 310, y: -20})
	for _, w := range []*entityState{engager, recoverer, harasser} {
		runtime.entities[w.id] = w
	}
	runtime.formCreaturePacksLocked()

	lunge := runtime.contracts.skillContract("lunge")
	inst := runtime.newCreatureActionInstance(engager, "lunge", lunge, engager.position, time.Now())
	engager.actionInstance = &inst
	recoverer.lastCommitAt = time.Now() // recently committed, now idle

	runtime.assignPackRolesLocked()
	if engager.packRole != "engager" {
		t.Fatalf("committing member role = %q, want engager", engager.packRole)
	}
	if recoverer.packRole != "recoverer" {
		t.Fatalf("recently-committed member role = %q, want recoverer", recoverer.packRole)
	}
	if harasser.packRole != "harasser" {
		t.Fatalf("idle member role = %q, want harasser", harasser.packRole)
	}
}

// TestPackFocusDistributesByThreat locks Pack/Threat Slice 5: the pack aggregates member threat,
// focuses most members on the top-threat target, and (soft_focus) peels one member to the
// secondary; a focused member's combat target honors the assignment.
func TestPackFocusDistributesByThreat(t *testing.T) {
	runtime := NewRuntimeWithContracts(DevFixtureRuntimeContracts())
	p1 := runtime.ensurePlayerLocked("p1")
	p2 := runtime.ensurePlayerLocked("p2")
	wolves := []*entityState{
		newFixtureWolf(801, vector{x: 300, y: 0}),
		newFixtureWolf(802, vector{x: 320, y: 20}),
		newFixtureWolf(803, vector{x: 310, y: -20}),
	}
	for _, w := range wolves {
		runtime.entities[w.id] = w
	}
	runtime.formCreaturePacksLocked()

	for _, w := range wolves {
		runtime.creditThreatLocked(w, p1, 100, 0) // p1 is the clear aggregate threat
		runtime.creditThreatLocked(w, p2, 10, 0)  // p2 a weak secondary
	}
	runtime.assignPackFocusLocked()

	p1Count, p2Count := 0, 0
	for _, w := range wolves {
		switch w.packFocusTargetID {
		case p1.id:
			p1Count++
		case p2.id:
			p2Count++
		default:
			t.Fatalf("wolf %d focus = %d, want p1 or p2", w.id, w.packFocusTargetID)
		}
	}
	if p1Count < 1 {
		t.Fatal("no member focused the aggregate top-threat target p1")
	}
	if p2Count != 1 {
		t.Fatalf("soft_focus peel count = %d, want exactly 1 member on the secondary", p2Count)
	}
	for _, w := range wolves {
		if w.packFocusTargetID == p1.id {
			if runtime.resolveCreatureCombatTargetLocked(w, time.Now()) != p1 {
				t.Fatal("combat target did not honor pack focus")
			}
			return
		}
	}
}

// TestPackFocusSinglePlayerNoRegression locks that pack focus does not change targeting when there
// is one player: each member's combat target is that player.
func TestPackFocusSinglePlayerNoRegression(t *testing.T) {
	runtime := NewRuntimeWithContracts(DevFixtureRuntimeContracts())
	player := runtime.ensurePlayerLocked("local_player")
	wolves := []*entityState{
		newFixtureWolf(901, vector{x: 300, y: 0}),
		newFixtureWolf(902, vector{x: 320, y: 20}),
	}
	for _, w := range wolves {
		runtime.entities[w.id] = w
	}
	runtime.formCreaturePacksLocked()
	runtime.assignPackFocusLocked()
	for _, w := range wolves {
		if got := runtime.resolveCreatureCombatTargetLocked(w, time.Now()); got != player {
			t.Fatalf("single-player combat target = %v, want the only player", got)
		}
	}
}
