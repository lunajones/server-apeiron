package gameapi

import (
	"math"

	domainmath "server-apeiron/internal/domain/math"
)

// defaultBlockArcDeg is the fallback frontal block arc: 180 degrees = the front half of
// the 360 around the defender. Restores the chat 10/14 rule: a block only neutralizes a
// hit inside the defender's frontal arc; a hit from behind bypasses the block.
const defaultBlockArcDeg = 180

// resolveDirectionalBlock decides whether a defender who is blocking can neutralize an
// incoming hit, based on where the hit comes from relative to the defender's facing.
//
//   - blocking: whether the defender is actively blocking.
//   - hitTravelDir: the direction the hit travels (attacker -> defender).
//   - defenderFacing: the defender's facing unit vector.
//   - frontalArcDeg: total block arc in degrees; <=0 falls back to 180.
//
// This is the first brick of the damage/defense pipeline. It is intentionally pure so it
// can be unit-tested and later wired into the live runtime once DB-sourced damage and the
// hitbox geometry resolve who/what was hit.
func resolveDirectionalBlock(blocking bool, hitTravelDir, defenderFacing domainmath.Vec3, frontalArcDeg float64) bool {
	if !blocking {
		return false
	}
	if frontalArcDeg <= 0 {
		frontalArcDeg = defaultBlockArcDeg
	}
	facing := defenderFacing.Normalize()
	// The threat direction (defender -> attacker) is the reverse of the hit travel.
	threat := hitTravelDir.Normalize().Scale(-1)
	if facing.IsZero() || threat.IsZero() {
		return false
	}
	dot := facing.Dot(threat)
	cosHalfArc := math.Cos((frontalArcDeg / 2) * math.Pi / 180)
	// Tolerance so an exactly-lateral hit on the 180-degree boundary counts as frontal
	// (cos(90deg) is a tiny positive float, not exactly 0).
	const blockArcEpsilon = 1e-9
	return dot >= cosHalfArc-blockArcEpsilon
}
