package movement

import (
	"math"
	"testing"
	"time"

	domainmath "server-apeiron/internal/domain/math"
)

func TestGroundedMoveSpeedUsesDirectionalProfile(t *testing.T) {
	profile := SpeedProfile{
		MaxSpeed:                       470,
		SprintSpeedMultiplier:          1.45,
		StrafeSpeedMultiplier:          0.92,
		BackpedalSpeedMultiplier:       0.65,
		StrafeSprintSpeedMultiplier:    0.75,
		BackpedalSprintSpeedMultiplier: 0.75,
	}

	forward := GroundedMoveSpeed(profile, true, 1, domainmath.V3(1, 0, 0), 0)
	strafe := GroundedMoveSpeed(profile, true, 1, domainmath.V3(0, 1, 0), 0)
	back := GroundedMoveSpeed(profile, true, 1, domainmath.V3(-1, 0, 0), 0)

	if forward <= 0 || strafe <= 0 || back <= 0 {
		t.Fatalf("speeds must be positive: forward=%v strafe=%v back=%v", forward, strafe, back)
	}
	if math.Abs(forward-681.5) > 0.0001 {
		t.Fatalf("forward speed = %v, want 681.5", forward)
	}
	if strafe >= forward {
		t.Fatalf("strafe speed should be capped below forward: strafe=%v forward=%v", strafe, forward)
	}
	if back >= forward {
		t.Fatalf("backpedal speed should be capped below forward: back=%v forward=%v", back, forward)
	}
	if math.Abs(strafe-(forward*0.75)) > 0.0001 {
		t.Fatalf("strafe sprint speed = %v, want exactly 75%% of forward sprint %v", strafe, forward)
	}
	if math.Abs(back-(forward*0.75)) > 0.0001 {
		t.Fatalf("backpedal sprint speed = %v, want exactly 75%% of forward sprint %v", back, forward)
	}
}

func TestResolveGroundedMovePublishesSingleTickMotion(t *testing.T) {
	got := ResolveGroundedMove(GroundedMoveInput{
		Position:        domainmath.V3(10, 20, 0),
		Direction:       domainmath.V3(1, 0, 0),
		FacingYawDeg:    0,
		AnalogMagnitude: 1,
		Sprint:          true,
		TickRate:        30,
		Profile: SpeedProfile{
			MaxSpeed:              470,
			SprintSpeedMultiplier: 1.45,
		},
	})

	if got.Stopped {
		t.Fatal("move should not be stopped")
	}
	if math.Abs(got.DistanceCM-(681.5/30)) > 0.0001 {
		t.Fatalf("distance = %v", got.DistanceCM)
	}
	if math.Abs(got.Projected.X-(10+681.5/30)) > 0.0001 || got.Projected.Y != 20 {
		t.Fatalf("projected position = %+v", got.Projected)
	}
	if math.Abs(got.Velocity.X-681.5) > 0.0001 {
		t.Fatalf("velocity = %+v", got.Velocity)
	}
}

func TestResolveActionMotionDerivesSpeedFromContractDuration(t *testing.T) {
	got := ResolveActionMotion(ActionMotionInput{
		Position:  domainmath.V3(0, 0, 0),
		Direction: domainmath.V3(1, 0, 0),
		Contract: RuntimeActionContract{
			ID:         "shield_rush_front_contact_v1",
			ActionType: "grounded_skill",
			DistanceCM: 960,
			DurationMS: 1100,
		},
	})

	if got.Stopped {
		t.Fatal("action should move")
	}
	if got.DistanceCM != 960 {
		t.Fatalf("distance = %v, want 960", got.DistanceCM)
	}
	if math.Abs(got.SpeedCMPerSecond-(960.0/1.1)) > 0.0001 {
		t.Fatalf("speed = %v, want derived canonical Shield Rush speed", got.SpeedCMPerSecond)
	}
	if got.Projected.X != 960 || got.Projected.Y != 0 {
		t.Fatalf("projected = %+v", got.Projected)
	}
}

func TestResolveActionMotionProgressUsesContractDurationNotActiveWindow(t *testing.T) {
	contract := RuntimeActionContract{
		ID:         "shield_rush_front_contact_v1",
		ActionType: "grounded_skill",
		DistanceCM: 960,
		DurationMS: 1100,
		ActiveMS:   720,
	}

	activeEnd := ResolveActionMotionProgress(ActionMotionProgressInput{
		Position:  domainmath.V3(0, 0, 0),
		Direction: domainmath.V3(1, 0, 0),
		Contract:  contract,
		Elapsed:   720 * time.Millisecond,
	})
	done := ResolveActionMotionProgress(ActionMotionProgressInput{
		Position:  domainmath.V3(0, 0, 0),
		Direction: domainmath.V3(1, 0, 0),
		Contract:  contract,
		Elapsed:   1100 * time.Millisecond,
	})

	if activeEnd.Complete {
		t.Fatal("action motion completed at ActiveMS; physical root must continue until DurationMS")
	}
	if math.Abs(activeEnd.DistanceCM-(960.0*(720.0/1100.0))) > 0.0001 {
		t.Fatalf("active-end distance = %v, want DurationMS-scaled distance", activeEnd.DistanceCM)
	}
	if !done.Complete || math.Abs(done.DistanceCM-960) > 0.0001 {
		t.Fatalf("done = complete:%v distance:%v, want complete/960", done.Complete, done.DistanceCM)
	}
}

func TestResolveActionMotionPrefersContractBaseSpeed(t *testing.T) {
	got := ResolveActionMotion(ActionMotionInput{
		Position:  domainmath.V3(0, 0, 0),
		Direction: domainmath.V3(0, 1, 0),
		Contract: RuntimeActionContract{
			ID:           "wolf_lunge_airborne_v1",
			ActionType:   "leap",
			DistanceCM:   620,
			DurationMS:   980,
			BaseSpeedCMS: 760,
		},
	})

	if got.SpeedCMPerSecond != 760 {
		t.Fatalf("speed = %v, want base speed 760", got.SpeedCMPerSecond)
	}
	if got.Velocity.Y != 760 {
		t.Fatalf("velocity = %+v", got.Velocity)
	}
}

func TestResolveActionMotionProgressIntegratesSpeedCurve(t *testing.T) {
	contract := RuntimeActionContract{
		ID:         "curved_action_v1",
		ActionType: "grounded_skill",
		DistanceCM: 100,
		DurationMS: 1000,
		SpeedCurveSamples: []MovementActionCurvePoint{
			{T: 0, Value: 0},
			{T: 0.5, Value: 1},
			{T: 1, Value: 0},
		},
	}

	quarter := ResolveActionMotionProgress(ActionMotionProgressInput{
		Position:  domainmath.V3(0, 0, 0),
		Direction: domainmath.V3(1, 0, 0),
		Contract:  contract,
		Elapsed:   250 * time.Millisecond,
	})
	half := ResolveActionMotionProgress(ActionMotionProgressInput{
		Position:  domainmath.V3(0, 0, 0),
		Direction: domainmath.V3(1, 0, 0),
		Contract:  contract,
		Elapsed:   500 * time.Millisecond,
	})
	done := ResolveActionMotionProgress(ActionMotionProgressInput{
		Position:  domainmath.V3(0, 0, 0),
		Direction: domainmath.V3(1, 0, 0),
		Contract:  contract,
		Elapsed:   1200 * time.Millisecond,
	})

	if math.Abs(quarter.DistanceCM-12.5) > 0.0001 {
		t.Fatalf("quarter distance = %v, want 12.5 from integrated curve", quarter.DistanceCM)
	}
	if math.Abs(half.DistanceCM-50) > 0.0001 {
		t.Fatalf("half distance = %v, want 50", half.DistanceCM)
	}
	if !done.Complete || math.Abs(done.DistanceCM-100) > 0.0001 {
		t.Fatalf("done = complete:%v distance:%v, want complete/100", done.Complete, done.DistanceCM)
	}
}

func TestResolveConstantStepKeepsCreatureMovementInSameKinematicLayer(t *testing.T) {
	got := ResolveConstantStep(ConstantStepInput{
		Position:         domainmath.V3(100, 100, 0),
		Direction:        domainmath.V3(0, -2, 0),
		SpeedCMPerSecond: 360,
		TickRate:         30,
	})

	if got.DistanceCM != 12 {
		t.Fatalf("distance = %v, want 12", got.DistanceCM)
	}
	if got.Projected.X != 100 || got.Projected.Y != 88 {
		t.Fatalf("projected = %+v", got.Projected)
	}
	if got.Velocity.Y != -360 {
		t.Fatalf("velocity = %+v", got.Velocity)
	}
}
