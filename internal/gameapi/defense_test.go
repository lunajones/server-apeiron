package gameapi

import (
	"testing"

	domainmath "server-apeiron/internal/domain/math"
)

// TestResolveDirectionalBlock locks the chat 10/14 rule: with the default 180-degree
// arc, a block neutralizes hits from the front half and lets back hits through.
func TestResolveDirectionalBlock(t *testing.T) {
	facing := domainmath.V3(1, 0, 0) // defender looks +X

	cases := []struct {
		name         string
		blocking     bool
		hitTravelDir domainmath.Vec3 // attacker -> defender
		want         bool
	}{
		// Attacker in front (+X), hit travels -X toward the defender: blocked.
		{"front hit blocked", true, domainmath.V3(-1, 0, 0), true},
		// Attacker behind (-X), hit travels +X (same as facing): bypasses block.
		{"back hit bypasses", true, domainmath.V3(1, 0, 0), false},
		// Pure lateral hit sits on the 180 boundary -> counts as frontal (>= 0).
		{"side hit on boundary blocks", true, domainmath.V3(0, -1, 0), true},
		// Not blocking: never blocked.
		{"not blocking", false, domainmath.V3(-1, 0, 0), false},
	}

	for _, tc := range cases {
		got := resolveDirectionalBlock(tc.blocking, tc.hitTravelDir, facing, 0)
		if got != tc.want {
			t.Fatalf("%s: blocked = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestResolveDirectionalBlockNarrowArc: a tighter arc rejects a hit that a 180 arc would
// have blocked, proving the arc is honored (forward-looking for DB-driven arcs).
func TestResolveDirectionalBlockNarrowArc(t *testing.T) {
	facing := domainmath.V3(1, 0, 0)
	// Hit coming from ~60 degrees off-front. Threat dir ~ (cos60, sin60). With a 90-deg
	// arc (half = 45), cos45 ~ 0.707; dot ~ 0.5 -> not blocked.
	hitTravel := domainmath.V3(-0.5, -0.866, 0) // threat ~ (0.5, 0.866)
	if resolveDirectionalBlock(true, hitTravel, facing, 90) {
		t.Fatal("hit at ~60deg off-front should NOT be blocked by a 90deg arc")
	}
	if !resolveDirectionalBlock(true, hitTravel, facing, 180) {
		t.Fatal("same hit SHOULD be blocked by a 180deg arc")
	}
}
