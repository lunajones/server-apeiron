package ai

import (
	"testing"

	domainmath "server-apeiron/internal/domain/math"
)

func TestRegionBrainSystemKeepsIndependentAgentMemory(t *testing.T) {
	system := NewRegionBrainSystem()
	policy := testPolicy()
	policy.AllowSideSwitchWhenTargetFaces = true
	policy.LockSideDuringSetup = false
	policy.SideFlipChanceMultiplier = 1
	policy.MinOrbitDurationTicks = 1
	policy.SideSwitchCooldownTicks = 1

	input := Input{
		Tick:             10,
		CreaturePosition: domainmath.V3(0, 0, 0),
		TargetPosition:   domainmath.V3(520, 0, 0),
		LineOfSight:      true,
	}
	first := system.Decide("wolf:1", policy, input)
	second := system.Decide("wolf:1", policy, Input{
		Tick:             20,
		CreaturePosition: domainmath.V3(0, 0, 0),
		TargetPosition:   domainmath.V3(520, 0, 0),
		LineOfSight:      true,
	})
	other := system.Decide("wolf:2", policy, input)

	if first.OrbitSide == second.OrbitSide {
		t.Fatalf("wolf:1 did not persist and advance orbit memory: first=%s second=%s", first.OrbitSide, second.OrbitSide)
	}
	if other.OrbitSide != first.OrbitSide {
		t.Fatalf("wolf:2 inherited wolf:1 memory: other=%s first=%s", other.OrbitSide, first.OrbitSide)
	}
}
