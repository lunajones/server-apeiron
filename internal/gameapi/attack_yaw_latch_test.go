package gameapi

import (
	"math"
	"testing"
	"time"

	dbv1 "db-apeiron/gen/apeiron/v1"
	gamev1 "server-apeiron/gen/apeiron/game/v1"
	creatureai "server-apeiron/internal/ai"
)

// TestCreatureLungeAttackYawLatchesAtTakeoffAndDrivesHitbox locks roadmap orientation
// rules 3-5: once the lunge takes off, attack yaw freezes to the committed line, stops
// tracking the moving target, the hitbox sweeps that latched line, and presentation keeps
// focus yaw (head/attention) separated from the latched attack yaw.
func TestCreatureLungeAttackYawLatchesAtTakeoffAndDrivesHitbox(t *testing.T) {
	runtime := NewRuntimeWithContracts(DevFixtureRuntimeContracts())
	player := runtime.ensurePlayerLocked("local_player")
	wolf := runtime.ensureWolfLocked(player)

	// Wolf at origin; player due east so the committed lunge line points east (+X).
	wolf.position = vector{x: 0, y: 0, z: player.position.z}
	player.position = vector{x: 600, y: 0, z: player.position.z}

	contract := runtime.contracts.skillContract("lunge")
	contract.Orientation = &dbv1.ActionOrientationPolicy{
		Id:                      "orientation_lunge_flank_commit_v1",
		AttackYawLatchPolicy:    "latch_at_takeoff",
		AllowBodySideOnMovement: true,
		BodyTurnRateDegS:        420,
	}
	contract.Envelope = &dbv1.ActionEnvelopePolicy{
		Id:               "envelope_lunge_low_raking_100_520_200_v1",
		PreCommitMs:      100,
		AirborneMs:       520,
		LandingInertiaMs: 200,
	}

	startedAt := time.Now()
	instance := runtime.newCreatureActionInstance(wolf, "lunge", contract, wolf.position, startedAt)
	wolf.actionInstance = &instance
	wolf.skillRuntime = &gamev1.SkillRuntimeState{
		CurrentSkillId: "lunge",
		State:          "active",
		StartedAtMs:    startedAt.UnixMilli(),
	}
	// Committed physical root pointing east — the latched attack yaw must match this.
	committedDir := vector{x: 1}
	wolf.actionMotion = &actionMotionState{
		SkillID:      "lunge",
		CommandID:    instance.InstanceID,
		MotionSource: "skill_root",
		StartedAt:    startedAt,
		Direction:    committedDir,
	}

	env := creatureActionMovementEnvelopeAt(instance, contract, startedAt)
	takeoff := env.AirborneStartsAt
	committedYaw := normalizeYaw(vectorYaw(committedDir))

	// Before takeoff: still tracking, not latched.
	runtime.updateCreatureActionOrientationLatchLocked(wolf, player, contract, &instance, takeoff.Add(-20*time.Millisecond))
	if wolf.actionOrientationLatch == nil || wolf.actionOrientationLatch.Latched {
		t.Fatalf("attack yaw latched before takeoff: %#v", wolf.actionOrientationLatch)
	}

	// At/after takeoff: latched to the committed line.
	runtime.updateCreatureActionOrientationLatchLocked(wolf, player, contract, &instance, takeoff.Add(20*time.Millisecond))
	if wolf.actionOrientationLatch == nil || !wolf.actionOrientationLatch.Latched {
		t.Fatalf("attack yaw did not latch at takeoff: %#v", wolf.actionOrientationLatch)
	}
	if d := math.Abs(normalizeYawDelta(wolf.actionOrientationLatch.AttackYawDeg - committedYaw)); d > 0.5 {
		t.Fatalf("latched attack yaw %.1f != committed %.1f", wolf.actionOrientationLatch.AttackYawDeg, committedYaw)
	}

	// Target jumps north: the latch must stay frozen on the committed (east) line.
	player.position = vector{x: 0, y: 600, z: player.position.z}
	runtime.updateCreatureActionOrientationLatchLocked(wolf, player, contract, &instance, takeoff.Add(120*time.Millisecond))
	if d := math.Abs(normalizeYawDelta(wolf.actionOrientationLatch.AttackYawDeg - committedYaw)); d > 0.5 {
		t.Fatalf("latched attack yaw drifted to moving target: %.1f", wolf.actionOrientationLatch.AttackYawDeg)
	}

	// Hitbox sweep follows the latched line, not the new (north) target bearing.
	schedule, ok := runtime.creatureSkillImpactScheduleLocked(wolf, player, contract, takeoff.Add(120*time.Millisecond))
	if !ok {
		t.Fatal("expected lunge impact schedule")
	}
	hitboxYaw := normalizeYaw(vectorYaw(schedule.Direction))
	if d := math.Abs(normalizeYawDelta(hitboxYaw - committedYaw)); d > 0.5 {
		t.Fatalf("hitbox swept toward moving target (%.1f) instead of latched line (%.1f)", hitboxYaw, committedYaw)
	}

	// Presentation: focus yaw tracks the target (north) while attack yaw stays latched (east).
	decision := creatureai.Decision{Action: "lunge", SelectedSkill: "lunge", MovementTactic: "lunge"}
	orientation := resolveCreatureActionOrientation(wolf, player, decision, contract, env, takeoff.Add(120*time.Millisecond))
	if !orientation.AttackYawLatched {
		t.Fatal("orientation did not report attack yaw latched")
	}
	if d := math.Abs(normalizeYawDelta(orientation.AttackYawDeg - committedYaw)); d > 0.5 {
		t.Fatalf("presented attack yaw %.1f != latched %.1f", orientation.AttackYawDeg, committedYaw)
	}
	focusYaw := normalizeYaw(vectorYaw(vector{x: player.position.x - wolf.position.x, y: player.position.y - wolf.position.y}))
	if d := math.Abs(normalizeYawDelta(orientation.FocusYawDeg - focusYaw)); d > 0.5 {
		t.Fatalf("focus yaw %.1f did not track target %.1f", orientation.FocusYawDeg, focusYaw)
	}
	if math.Abs(normalizeYawDelta(orientation.AttackYawDeg-orientation.FocusYawDeg)) < 1 {
		t.Fatal("attack yaw and focus yaw not separated after target moved")
	}
}

// TestCreatureActionLatchResetsPerInstance ensures a new action instance gets a fresh
// latch instead of inheriting the previous action's committed direction.
func TestCreatureActionLatchResetsPerInstance(t *testing.T) {
	runtime := NewRuntimeWithContracts(DevFixtureRuntimeContracts())
	player := runtime.ensurePlayerLocked("local_player")
	wolf := runtime.ensureWolfLocked(player)
	wolf.position = vector{x: 0, y: 0, z: player.position.z}
	player.position = vector{x: 600, y: 0, z: player.position.z}

	contract := runtime.contracts.skillContract("lunge")
	contract.Orientation = &dbv1.ActionOrientationPolicy{
		Id:                   "orientation_lunge_flank_commit_v1",
		AttackYawLatchPolicy: "latch_at_takeoff",
	}
	contract.Envelope = &dbv1.ActionEnvelopePolicy{PreCommitMs: 100, AirborneMs: 520, LandingInertiaMs: 200}

	startedAt := time.Now()
	first := runtime.newCreatureActionInstance(wolf, "lunge", contract, wolf.position, startedAt)
	wolf.actionInstance = &first
	wolf.actionMotion = &actionMotionState{SkillID: "lunge", CommandID: first.InstanceID, MotionSource: "skill_root", Direction: vector{x: 1}}
	takeoff := creatureActionMovementEnvelopeAt(first, contract, startedAt).AirborneStartsAt
	runtime.updateCreatureActionOrientationLatchLocked(wolf, player, contract, &first, takeoff.Add(20*time.Millisecond))
	if wolf.actionOrientationLatch == nil || !wolf.actionOrientationLatch.Latched {
		t.Fatal("first instance did not latch")
	}

	// A new instance (different InstanceID) must reset the latch back to unlatched.
	runtime.tick++
	second := runtime.newCreatureActionInstance(wolf, "lunge", contract, wolf.position, startedAt.Add(2*time.Second))
	if second.InstanceID == first.InstanceID {
		t.Fatal("test setup: instances share an id")
	}
	wolf.actionInstance = &second
	wolf.actionMotion = &actionMotionState{SkillID: "lunge", CommandID: second.InstanceID, MotionSource: "skill_root", Direction: vector{y: 1}}
	runtime.updateCreatureActionOrientationLatchLocked(wolf, player, contract, &second, startedAt.Add(2*time.Second))
	if wolf.actionOrientationLatch == nil {
		t.Fatal("second instance latch missing")
	}
	if wolf.actionOrientationLatch.InstanceID != second.InstanceID {
		t.Fatalf("latch kept previous instance id %q", wolf.actionOrientationLatch.InstanceID)
	}
	if wolf.actionOrientationLatch.Latched {
		t.Fatal("new instance inherited a latched attack yaw instead of resetting")
	}
}

// TestCreatureOrientationFocusAndAttackEaseAtContractTurnRates locks that focus and
// pre-latch attack yaw ease toward the target at their own policy turn rates (the head
// leads faster than the strike winds up) instead of snapping.
func TestCreatureOrientationFocusAndAttackEaseAtContractTurnRates(t *testing.T) {
	runtime := NewRuntimeWithContracts(DevFixtureRuntimeContracts())
	player := runtime.ensurePlayerLocked("local_player")
	wolf := runtime.ensureWolfLocked(player)
	wolf.position = vector{x: 0, y: 0, z: player.position.z}
	player.position = vector{x: 0, y: 600, z: player.position.z} // due north

	contract := runtime.contracts.skillContract("lunge")
	contract.Orientation = &dbv1.ActionOrientationPolicy{
		Id:                   "orientation_test_rates",
		FocusTurnRateDegS:    300,
		AttackTurnRateDegS:   150,
		AttackYawLatchPolicy: "none", // keep attack tracking so its turn rate is observable
	}

	targetYaw := normalizeYaw(vectorYaw(vector{x: player.position.x - wolf.position.x, y: player.position.y - wolf.position.y}))
	// Seed both yaws 90 deg away from the target so easing is visible (not a first-seen snap).
	startYaw := normalizeYaw(targetYaw + 90)
	wolf.orientationFocusYaw, wolf.orientationFocusYawKnown = startYaw, true
	wolf.orientationAttackYaw, wolf.orientationAttackYawKnown = startYaw, true

	orientation := resolveCreatureActionOrientation(wolf, player, creatureai.Decision{}, contract, creatureActionMovementEnvelope{}, time.Now())

	focusMoved := math.Abs(normalizeYawDelta(orientation.FocusYawDeg - startYaw))
	attackMoved := math.Abs(normalizeYawDelta(orientation.AttackYawDeg - startYaw))
	if focusMoved <= 0 || focusMoved >= 90 {
		t.Fatalf("focus did not ease toward target (moved %.1f of 90)", focusMoved)
	}
	if attackMoved <= 0 || attackMoved >= 90 {
		t.Fatalf("attack did not ease toward target (moved %.1f of 90)", attackMoved)
	}
	if focusMoved <= attackMoved {
		t.Fatalf("focus rate (300) should ease faster than attack rate (150): focus %.1f attack %.1f", focusMoved, attackMoved)
	}
	if math.Abs(normalizeYawDelta(orientation.FocusYawDeg-targetYaw)) >= 90 {
		t.Fatal("focus eased away from the target instead of toward it")
	}
}
