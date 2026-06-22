package gameapi

import (
	"testing"

	domainmath "server-apeiron/internal/domain/math"
	"server-apeiron/internal/movement"
)

// TestCapBlockMotionHalvesWalkSpeed locks the chat-13 rule: while blocking, the player
// moves at most at half the walk speed, even when sprinting.
func TestCapBlockMotionHalvesWalkSpeed(t *testing.T) {
	profile := movement.SpeedProfile{MaxSpeed: 600, SprintSpeedMultiplier: 1.6}

	motion := movement.ResolveGroundedMove(movement.GroundedMoveInput{
		Direction:       domainmath.V3(1, 0, 0),
		FacingYawDeg:    0,
		AnalogMagnitude: 1,
		Sprint:          true,
		TickRate:        30,
		Profile:         profile,
	})
	if motion.SpeedCMPerSecond <= 300 {
		t.Fatalf("precondition: sprint speed should exceed the block cap, got %v", motion.SpeedCMPerSecond)
	}

	capped := capBlockMotion(motion, profile)
	want := profile.MaxSpeed * blockSpeedWalkFraction // 300
	if capped.SpeedCMPerSecond != want {
		t.Fatalf("blocked speed = %v, want %v", capped.SpeedCMPerSecond, want)
	}
	if capped.DistanceCM >= motion.DistanceCM {
		t.Fatalf("block did not reduce distance: %v -> %v", motion.DistanceCM, capped.DistanceCM)
	}
}

// TestCapBlockMotionLeavesSlowMovementUntouched: walking under the cap is not changed.
func TestCapBlockMotionLeavesSlowMovementUntouched(t *testing.T) {
	profile := movement.SpeedProfile{MaxSpeed: 600}
	motion := movement.ResolveConstantStep(movement.ConstantStepInput{
		Direction:        domainmath.V3(1, 0, 0),
		SpeedCMPerSecond: 200, // below 0.5*600=300
		TickRate:         30,
	})
	capped := capBlockMotion(motion, profile)
	if capped.SpeedCMPerSecond != 200 {
		t.Fatalf("slow movement should be untouched, got %v", capped.SpeedCMPerSecond)
	}
}
