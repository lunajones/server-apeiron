package gameapi

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	gamev1 "server-apeiron/gen/apeiron/game/v1"
)

func TestRuntimeLocomotionTransitionKeepsReconciliationFields(t *testing.T) {
	t.Parallel()

	runtime := NewRuntimeWithOptions(RecoveredRuntimeContracts(), RuntimeOptions{MovementValidation: true})
	sessionID := "runtime-integration-turn-skill"

	openCtx := &gamev1.OpenSessionRequest{Context: &gamev1.RequestContext{SessionId: sessionID}}
	_, err := runtime.OpenSession(context.Background(), openCtx)
	if err != nil {
		t.Fatalf("OpenSession failed: %v", err)
	}
	_, err = runtime.AttachPlayer(context.Background(), &gamev1.AttachPlayerRequest{
		Context:  &gamev1.RequestContext{SessionId: sessionID},
		PlayerId: "local_player",
	})
	if err != nil {
		t.Fatalf("AttachPlayer failed: %v", err)
	}

	player := runtime.ensurePlayerLocked("local_player")

	submit := func(cmd *gamev1.PlayerCommand) {
		if _, err := runtime.SubmitCommand(context.Background(), cmd); err != nil {
			t.Fatalf("SubmitCommand failed (%s): %v", commandTypeName(cmd.GetType()), err)
		}
	}

	submit(testRuntimeMoveCommand(sessionID, 1, gamev1Vector(1, 0, 0), 1, true, nil))
	if got := player.locomotion; got == nil {
		t.Fatal("locomotion is nil after sprint move")
	} else {
		if got.GetAction() != "move" {
			t.Fatalf("move action = %q, want move", got.GetAction())
		}
		if got.GetAbilityKey() != "move" {
			t.Fatalf("move ability = %q, want move", got.GetAbilityKey())
		}
		if got.GetReconciliationMode() != "grounded_move_reconciliation" {
			t.Fatalf("move reconciliation = %q, want grounded_move_reconciliation", got.GetReconciliationMode())
		}
		if got.GetActionDistanceTraveled() < 0 {
			t.Fatalf("move distance traveled = %v", got.GetActionDistanceTraveled())
		}
	}

	submit(testRuntimeTurnCommand(sessionID, 2, 45))
	if got := player.locomotion; got == nil {
		t.Fatal("locomotion is nil after turn")
	} else if got.GetAction() != "move" {
		t.Fatalf("turn should not stomp active move locomotion, action = %q", got.GetAction())
	}

	beforeSkillPosition := player.position
	submit(testRuntimeCastSkillCommand(sessionID, 3, "player_shield_rush", gamev1Vector(1, 0, 0)))
	if got := player.locomotion; got == nil {
		t.Fatal("locomotion is nil after cast skill")
	} else {
		if got.GetAction() != "grounded_skill" {
			t.Fatalf("skill action = %q, want grounded_skill", got.GetAction())
		}
		if got.GetAbilityKey() != "player_shield_rush" {
			t.Fatalf("skill ability = %q, want player_shield_rush", got.GetAbilityKey())
		}
		if got.GetReconciliationMode() != "grounded_skill_action" {
			t.Fatalf("skill reconciliation = %q, want grounded_skill_action (Unreal-recognized wire mode)", got.GetReconciliationMode())
		}
		if got.GetTargetSpeed() <= 0 || got.GetEffectiveSpeed() <= 0 {
			t.Fatalf("skill locomotion should publish motion speed: target=%v effective=%v", got.GetTargetSpeed(), got.GetEffectiveSpeed())
		}
	}
	if moved := distance(beforeSkillPosition, player.position); moved > 1 {
		t.Fatalf("skill command teleported player by %.2fcm; movement must progress through snapshots", moved)
	}
	forceCompleteRuntimeAction(t, runtime, sessionID, player)

	for i := uint64(0); i < 6; i++ {
		dir := gamev1Vector(0, 1, 0)
		if i%2 == 1 {
			dir = gamev1Vector(0, -1, 0)
		}
		yaw := 60 + float64(i*15)
		submit(testRuntimeTurnCommand(sessionID, 4+i, yaw))
		submit(testRuntimeMoveCommand(sessionID, 10+i, dir, 1, true, &yaw))
		g := player.locomotion
		if g == nil {
			t.Fatalf("iteration %d locomotion is nil", i)
		}
		if g.GetActionDistanceTraveled() <= 0 || g.GetTargetSpeed() <= 0 {
			t.Fatalf("iteration %d locomotion must remain moving: action=%v distance=%v target=%v", i, safeAction(g), safeDistance(g), safeSpeed(g))
		}
		if g.GetAction() != "move" {
			t.Fatalf("iteration %d action = %q, expected move", i, g.GetAction())
		}
	}
}

func TestRuntimeTurnWithMissingLocomotionSeedsTurnContract(t *testing.T) {
	t.Parallel()

	runtime := NewRuntimeWithOptions(RecoveredRuntimeContracts(), RuntimeOptions{MovementValidation: true})
	sessionID := "runtime-integration-turn-fallback"
	if _, err := runtime.OpenSession(context.Background(), &gamev1.OpenSessionRequest{
		Context: &gamev1.RequestContext{SessionId: sessionID},
	}); err != nil {
		t.Fatalf("OpenSession failed: %v", err)
	}
	if _, err := runtime.AttachPlayer(context.Background(), &gamev1.AttachPlayerRequest{
		Context:  &gamev1.RequestContext{SessionId: sessionID},
		PlayerId: "local_player",
	}); err != nil {
		t.Fatalf("AttachPlayer failed: %v", err)
	}

	player := runtime.ensurePlayerLocked("local_player")
	player.locomotion = nil

	if _, err := runtime.SubmitCommand(context.Background(), testRuntimeTurnCommand(sessionID, 1, 90)); err != nil {
		t.Fatalf("SubmitCommand failed: %v", err)
	}

	if player.locomotion == nil {
		t.Fatal("locomotion should be created by turn command")
	}
	if player.locomotion.GetAction() != "turn" {
		t.Fatalf("turn fallback action = %q", player.locomotion.GetAction())
	}
	if player.locomotion.GetAbilityKey() != "turn" {
		t.Fatalf("turn fallback ability = %q", player.locomotion.GetAbilityKey())
	}
	if player.locomotion.GetReconciliationMode() != "turn_reconciliation" {
		t.Fatalf("turn reconciliation = %q", player.locomotion.GetReconciliationMode())
	}
}

func TestGroundedMoveSpeedPreservesDirectionalCaps(t *testing.T) {
	t.Parallel()

	runtime := NewRuntimeWithContracts(RecoveredRuntimeContracts())
	player := runtime.ensurePlayerLocked("local_player")
	profile := runtime.contracts.MovementProfile
	if profile == nil {
		t.Fatal("movement profile missing")
	}

	forward := runtime.groundedMoveSpeed(true, 1, vector{x: 1}, 0)
	right := runtime.groundedMoveSpeed(true, 1, vector{x: 0, y: 1}, 0)
	backward := runtime.groundedMoveSpeed(true, 1, vector{x: -1}, 0)
	negAnalog := runtime.groundedMoveSpeed(true, -1, vector{x: 1}, 0)

	if forward <= 0 {
		t.Fatalf("forward speed=%v", forward)
	}
	if right <= 0 {
		t.Fatalf("right speed=%v", right)
	}
	if backward <= 0 {
		t.Fatalf("backward speed=%v", backward)
	}
	if backward >= forward {
		t.Fatalf("backward=%v should be lower than forward=%v", backward, forward)
	}
	if math.Abs(negAnalog-forward) > 0.0001 {
		t.Fatalf("negative analog should clamp to 1: %v != %v", negAnalog, forward)
	}

	// Keep the movement profile multiplier math explicit so unintended profile regressions
	// become obvious in tests instead of becoming rubberband feedback in playtests.
	expectedForward := profile.GetMaxSpeed() * profile.GetSprintSpeedMultiplier()
	if math.Abs(forward-expectedForward) > 0.0001 {
		t.Fatalf("forward speed drifted: got=%v want=%v", forward, expectedForward)
	}
	if right >= forward {
		t.Fatalf("strafe speed should be capped below forward %v: %v", forward, right)
	}
	_ = player
}

func TestRuntimeSprintStrafeYawInversionInterleavedWithSkills(t *testing.T) {
	t.Parallel()

	runtime := NewRuntimeWithOptions(RecoveredRuntimeContracts(), RuntimeOptions{MovementValidation: true})
	sessionID := "runtime-integration-sprint-strafe-skill"

	if _, err := runtime.OpenSession(context.Background(), &gamev1.OpenSessionRequest{
		Context: &gamev1.RequestContext{SessionId: sessionID},
	}); err != nil {
		t.Fatalf("OpenSession failed: %v", err)
	}
	if _, err := runtime.AttachPlayer(context.Background(), &gamev1.AttachPlayerRequest{
		Context:  &gamev1.RequestContext{SessionId: sessionID},
		PlayerId: "local_player",
	}); err != nil {
		t.Fatalf("AttachPlayer failed: %v", err)
	}

	player := runtime.ensurePlayerLocked("local_player")
	diag := 0.7071067811865476
	baseYaw := 90.0
	sequence := uint64(1)
	skills := []string{"player_shield_bash", "player_shield_rush"}

	submit := func(cmd *gamev1.PlayerCommand) {
		ack, err := runtime.SubmitCommand(context.Background(), cmd)
		if err != nil {
			t.Fatalf("SubmitCommand failed (%s): %v", commandTypeName(cmd.GetType()), err)
		}
		if ack == nil || !ack.GetAccepted() {
			t.Fatalf("command rejected: type=%s code=%s message=%s", commandTypeName(cmd.GetType()), ack.GetRejectionCode(), ack.GetMessage())
		}
	}

	// Baseline sprint move to establish momentum before aggressive yaw inversions.
	submit(testRuntimeMoveCommand(sessionID, sequence, gamev1Vector(1, 0, 0), 1, true, nil))
	sequence++
	if player.locomotion == nil {
		t.Fatal("baseline locomotion missing")
	}
	if player.locomotion.GetAction() != "move" {
		t.Fatalf("baseline action = %q", player.locomotion.GetAction())
	}
	baseSpeed := player.locomotion.GetTargetSpeed()
	if baseSpeed <= 0 {
		t.Fatalf("baseline speed = %v", baseSpeed)
	}

	// Stress:
	// alternate forward+strafe directions, invert yaw each iteration, and interleave
	// 2x2 basic combat inputs while keeping sprint active.
	for i := 0; i < 12; i++ {
		// 0/1 => W + A, 2/3 => W + D, repeated with abrupt yaw inversion.
		var dir *gamev1.Vector3
		if i%4 == 0 || i%4 == 1 {
			dir = gamev1Vector(diag, diag, 0)
		} else {
			dir = gamev1Vector(-diag, diag, 0)
		}
		if i%2 == 0 {
			baseYaw -= 55
		} else {
			baseYaw += 55
		}

		submit(testRuntimeTurnCommand(sessionID, sequence, baseYaw))
		sequence++

		submit(testRuntimeMoveCommand(sessionID, sequence, dir, 1, true, &baseYaw))
		sequence++

		expectedSpeed := runtime.groundedMoveSpeed(true, 1, vector{x: dir.GetX(), y: dir.GetY()}, baseYaw)
		if player.locomotion == nil {
			t.Fatalf("iteration %d locomotion missing", i)
		}
		if player.locomotion.GetAction() != "move" {
			t.Fatalf("iteration %d action should be move, got=%q", i, player.locomotion.GetAction())
		}
		if player.locomotion.GetAbilityKey() != "move" {
			t.Fatalf("iteration %d ability should be move, got=%q", i, player.locomotion.GetAbilityKey())
		}
		if player.locomotion.GetReconciliationMode() != "grounded_move_reconciliation" {
			t.Fatalf("iteration %d reconciliation should be grounded_move_reconciliation, got=%q", i, player.locomotion.GetReconciliationMode())
		}
		if player.locomotion.GetTargetSpeed() <= 0 {
			t.Fatalf("iteration %d target speed invalid: %v", i, player.locomotion.GetTargetSpeed())
		}
		if player.locomotion.GetTargetSpeed() < expectedSpeed*0.7 {
			t.Fatalf("iteration %d speed below expected profile floor: got=%v expected~%v", i, player.locomotion.GetTargetSpeed(), expectedSpeed)
		}
		if player.locomotion.GetTargetSpeed() > expectedSpeed*1.5 {
			t.Fatalf("iteration %d speed above expected profile ceiling: got=%v expected~%v", i, player.locomotion.GetTargetSpeed(), expectedSpeed)
		}

		if i%3 == 1 {
			skillID := skills[(i/3)%len(skills)]
			submit(testRuntimeCastSkillCommand(sessionID, sequence, skillID, gamev1Vector(1, 0, 0)))
			sequence++
			if player.locomotion == nil || player.locomotion.GetAction() != "grounded_skill" {
				t.Fatalf("iteration %d expected skill locomotion after cast", i)
			}
			forceCompleteRuntimeAction(t, runtime, sessionID, player)
		}

		if player.locomotion != nil && player.locomotion.GetTargetSpeed() > 0 && player.locomotion.GetTargetSpeed() < 20 {
			t.Fatalf("iteration %d suspiciously low speed after locomotion switch: %v", i, player.locomotion.GetTargetSpeed())
		}
	}

	// End state should still be stable locomotion and still reconciled as grounded move.
	if player.locomotion == nil {
		t.Fatal("final locomotion missing")
	}
	if player.locomotion.GetAction() != "move" && player.locomotion.GetAction() != "grounded_skill" {
		t.Fatalf("final action unexpected: %q", player.locomotion.GetAction())
	}
}

func TestRuntimeTurnOnlyDoesNotReplaceActiveMoveLocomotion(t *testing.T) {
	t.Parallel()

	runtime := NewRuntimeWithOptions(RecoveredRuntimeContracts(), RuntimeOptions{MovementValidation: true})
	sessionID := "runtime-integration-turn-only"

	if _, err := runtime.OpenSession(context.Background(), &gamev1.OpenSessionRequest{
		Context: &gamev1.RequestContext{SessionId: sessionID},
	}); err != nil {
		t.Fatalf("OpenSession failed: %v", err)
	}
	if _, err := runtime.AttachPlayer(context.Background(), &gamev1.AttachPlayerRequest{
		Context:  &gamev1.RequestContext{SessionId: sessionID},
		PlayerId: "local_player",
	}); err != nil {
		t.Fatalf("AttachPlayer failed: %v", err)
	}

	player := runtime.ensurePlayerLocked("local_player")
	submit := func(cmd *gamev1.PlayerCommand) {
		if _, err := runtime.SubmitCommand(context.Background(), cmd); err != nil {
			t.Fatalf("SubmitCommand failed (%s): %v", commandTypeName(cmd.GetType()), err)
		}
	}

	submit(testRuntimeMoveCommand(sessionID, 1, gamev1Vector(0, 1, 0), 1, true, nil))
	if player.locomotion == nil || player.locomotion.GetAction() != "move" {
		t.Fatalf("after baseline move action=%v", safeAction(player.locomotion))
	}

	for i := 0; i < 8; i++ {
		yaw := float64(i * 45)
		submit(testRuntimeTurnCommand(sessionID, uint64(i+2), yaw))
		// Keep move active across sharp turn inputs.
		submit(testRuntimeMoveCommand(sessionID, uint64(i+11), gamev1Vector(0, 1, 0), 1, true, &yaw))
		if player.locomotion == nil {
			t.Fatalf("iteration %d locomotion missing", i)
		}
		if player.locomotion.GetAction() != "move" {
			t.Fatalf("iteration %d action=%q expected move", i, player.locomotion.GetAction())
		}
		if player.locomotion.GetTargetSpeed() <= 0 {
			t.Fatalf("iteration %d target speed=%v", i, player.locomotion.GetTargetSpeed())
		}
		if player.locomotion.GetActionDistanceTraveled() <= 0 {
			t.Fatalf("iteration %d distance traveled should be >0", i)
		}
	}
}

func TestRuntimeShiftStrafeYawInversionKeepsMoveReconciled(t *testing.T) {
	t.Parallel()

	runtime := NewRuntimeWithOptions(RecoveredRuntimeContracts(), RuntimeOptions{MovementValidation: true})
	sessionID := "runtime-integration-shift-stride-yaw"

	if _, err := runtime.OpenSession(context.Background(), &gamev1.OpenSessionRequest{
		Context: &gamev1.RequestContext{SessionId: sessionID},
	}); err != nil {
		t.Fatalf("OpenSession failed: %v", err)
	}
	if _, err := runtime.AttachPlayer(context.Background(), &gamev1.AttachPlayerRequest{
		Context:  &gamev1.RequestContext{SessionId: sessionID},
		PlayerId: "local_player",
	}); err != nil {
		t.Fatalf("AttachPlayer failed: %v", err)
	}

	player := runtime.ensurePlayerLocked("local_player")
	submit := func(cmd *gamev1.PlayerCommand) {
		ack, err := runtime.SubmitCommand(context.Background(), cmd)
		if err != nil {
			t.Fatalf("SubmitCommand failed (%s): %v", commandTypeName(cmd.GetType()), err)
		}
		if ack == nil || !ack.GetAccepted() {
			t.Fatalf("command rejected: type=%s code=%s message=%s", commandTypeName(cmd.GetType()), ack.GetRejectionCode(), ack.GetMessage())
		}
	}

	sequence := uint64(1)
	baseYaw := 90.0
	left := gamev1Vector(-0.7071067811865476, 0.7071067811865476, 0)
	right := gamev1Vector(0.7071067811865476, 0.7071067811865476, 0)

	submit(testRuntimeMoveCommand(sessionID, sequence, gamev1Vector(0, 1, 0), 1, true, &baseYaw))
	sequence++
	if player.locomotion == nil || player.locomotion.GetAction() != "move" {
		t.Fatalf("baseline move missing: action=%s", safeAction(player.locomotion))
	}

	for i := 0; i < 16; i++ {
		if i%2 == 0 {
			baseYaw -= 85
		} else {
			baseYaw += 85
		}

		submit(testRuntimeTurnCommand(sessionID, sequence, baseYaw))
		sequence++

		var dir *gamev1.Vector3
		if i%2 == 0 {
			dir = left
		} else {
			dir = right
		}

		submit(testRuntimeMoveCommand(sessionID, sequence, dir, 1, true, &baseYaw))
		sequence++
		g := player.locomotion
		if g == nil {
			t.Fatalf("iteration %d locomotion nil", i)
		}
		if g.GetAction() != "move" {
			t.Fatalf("iteration %d action=%q expected move", i, g.GetAction())
		}
		if g.GetAbilityKey() != "move" {
			t.Fatalf("iteration %d ability=%q expected move", i, g.GetAbilityKey())
		}
		if g.GetReconciliationMode() != "grounded_move_reconciliation" {
			t.Fatalf("iteration %d reconciliation=%q expected grounded_move_reconciliation", i, g.GetReconciliationMode())
		}

		expected := runtime.groundedMoveSpeed(true, 1, vector{x: dir.GetX(), y: dir.GetY()}, baseYaw)
		if expected <= 0 {
			t.Fatalf("iteration %d expected speed must be >0", i)
		}
		if g.GetTargetSpeed() < expected*0.65 {
			t.Fatalf("iteration %d too-low speed: got=%v expected~%v", i, g.GetTargetSpeed(), expected)
		}
		if g.GetTargetSpeed() > expected*1.7 {
			t.Fatalf("iteration %d too-high speed: got=%v expected~%v", i, g.GetTargetSpeed(), expected)
		}
		if g.GetActionDistanceTraveled() <= 0 {
			t.Fatalf("iteration %d distance not advancing", i)
		}

		// Keep the engine in an aggressive curve while throwing short skill windows.
		if i%3 == 2 {
			submit(testRuntimeCastSkillCommand(sessionID, sequence, "player_shield_bash", dir))
			sequence++
			if player.locomotion == nil || player.locomotion.GetAction() != "grounded_skill" {
				t.Fatalf("iteration %d expected grounded_skill after shield bash", i)
			}
			if player.locomotion.GetReconciliationMode() != "grounded_skill_action" {
				t.Fatalf("iteration %d shield bash reconciliation=%q expected grounded_skill_action", i, player.locomotion.GetReconciliationMode())
			}
			forceCompleteRuntimeAction(t, runtime, sessionID, player)
		}
	}

	if player.locomotion == nil {
		t.Fatal("final locomotion missing")
	}
	if player.locomotion.GetAction() != "move" && player.locomotion.GetAction() != "grounded_skill" {
		t.Fatalf("final action unexpected: %q", player.locomotion.GetAction())
	}
}

func TestRuntimeShiftRunRepeatedBasicAttackPresses(t *testing.T) {
	t.Parallel()

	runtime := NewRuntimeWithOptions(RecoveredRuntimeContracts(), RuntimeOptions{MovementValidation: true})
	sessionID := "runtime-integration-shift-run-basic-presses"

	if _, err := runtime.OpenSession(context.Background(), &gamev1.OpenSessionRequest{
		Context: &gamev1.RequestContext{SessionId: sessionID},
	}); err != nil {
		t.Fatalf("OpenSession failed: %v", err)
	}
	if _, err := runtime.AttachPlayer(context.Background(), &gamev1.AttachPlayerRequest{
		Context:  &gamev1.RequestContext{SessionId: sessionID},
		PlayerId: "local_player",
	}); err != nil {
		t.Fatalf("AttachPlayer failed: %v", err)
	}

	player := runtime.ensurePlayerLocked("local_player")
	sequence := uint64(2)
	submit := func(cmd *gamev1.PlayerCommand) {
		ack, err := runtime.SubmitCommand(context.Background(), cmd)
		if err != nil {
			t.Fatalf("SubmitCommand failed (%s): %v", commandTypeName(cmd.GetType()), err)
		}
		if ack == nil {
			t.Fatalf("ack was nil for %s", commandTypeName(cmd.GetType()))
		}
		if !ack.GetAccepted() {
			t.Fatalf("command rejected: type=%s code=%s message=%s", commandTypeName(cmd.GetType()), ack.GetRejectionCode(), ack.GetMessage())
		}
	}

	// Baseline sprinted forward.
	submit(testRuntimeMoveCommand(sessionID, 1, gamev1Vector(0, 1, 0), 1, true, nil))
	if player.locomotion == nil || player.locomotion.GetAction() != "move" {
		t.Fatalf("baseline move failed: action=%s", safeAction(player.locomotion))
	}

	attackCycle := []string{"player_basic_attack_1", "player_basic_attack_2", "player_basic_attack_3"}
	baseYaw := 90.0

	for i := 0; i < 12; i++ {
		if i%2 == 0 {
			baseYaw -= 75
		} else {
			baseYaw += 75
		}

		// Keep strafing with sprint active during repeated basic attack presses.
		dir := gamev1Vector(-0.7071067811865476, 0.7071067811865476, 0)
		if i%3 == 0 {
			dir = gamev1Vector(0.7071067811865476, 0.7071067811865476, 0)
		}

		submit(testRuntimeTurnCommand(sessionID, sequence, baseYaw))
		sequence++
		submit(testRuntimeMoveCommand(sessionID, sequence, dir, 1, true, &baseYaw))
		sequence++

		g := player.locomotion
		if g == nil || g.GetAction() != "move" {
			t.Fatalf("iteration %d expected move before cast, got=%s", i, safeAction(g))
		}
		if g.GetTargetSpeed() <= 0 {
			t.Fatalf("iteration %d move speed not positive: %v", i, g.GetTargetSpeed())
		}
		if g.GetActionDistanceTraveled() <= 0 {
			t.Fatalf("iteration %d no movement progress", i)
		}

		submit(testRuntimeCastSkillCommand(sessionID, sequence, attackCycle[i%len(attackCycle)], dir))
		sequence++

		if player.locomotion == nil {
			t.Fatalf("iteration %d locomotion nil after cast", i)
		}
		if player.locomotion.GetAction() != "grounded_skill" {
			t.Fatalf("iteration %d expected grounded_skill after cast, got=%q", i, player.locomotion.GetAction())
		}
		if player.locomotion.GetReconciliationMode() != "grounded_skill_action" {
			t.Fatalf("iteration %d expected grounded_skill_action, got=%q", i, player.locomotion.GetReconciliationMode())
		}
		forceCompleteRuntimeAction(t, runtime, sessionID, player)
	}

	if player.locomotion == nil {
		t.Fatal("final locomotion missing")
	}
	if player.locomotion.GetAction() == "" {
		t.Fatalf("final action empty")
	}
	_ = player
}

func TestRuntimeCastPublishesActionInstanceAckMetadata(t *testing.T) {
	t.Parallel()

	runtime := NewRuntimeWithOptions(RecoveredRuntimeContracts(), RuntimeOptions{MovementValidation: true})
	sessionID := "runtime-integration-action-instance-ack"
	if _, err := runtime.OpenSession(context.Background(), &gamev1.OpenSessionRequest{
		Context: &gamev1.RequestContext{SessionId: sessionID},
	}); err != nil {
		t.Fatalf("OpenSession failed: %v", err)
	}
	if _, err := runtime.AttachPlayer(context.Background(), &gamev1.AttachPlayerRequest{
		Context:  &gamev1.RequestContext{SessionId: sessionID},
		PlayerId: "local_player",
	}); err != nil {
		t.Fatalf("AttachPlayer failed: %v", err)
	}

	ack, err := runtime.SubmitCommand(context.Background(), testRuntimeCastSkillCommand(sessionID, 1, "player_shield_rush", gamev1Vector(1, 0, 0)))
	if err != nil {
		t.Fatalf("SubmitCommand failed: %v", err)
	}
	if !ack.GetAccepted() {
		t.Fatalf("cast rejected: %s %s", ack.GetRejectionCode(), ack.GetMessage())
	}
	if ack.GetMetadata()["action_instance_id"] == "" {
		t.Fatalf("action_instance_id missing from ack metadata: %#v", ack.GetMetadata())
	}
	if ack.GetMetadata()["action_kind"] != "active_skill" {
		t.Fatalf("action_kind = %q", ack.GetMetadata()["action_kind"])
	}
	if ack.GetMetadata()["movement_action_contract_id"] == "" {
		t.Fatalf("movement_action_contract_id missing: %#v", ack.GetMetadata())
	}
	if ack.GetMetadata()["movement_action_contract_hash"] != "grounded_skill_action_reconciliation" {
		t.Fatalf("movement_action_contract_hash = %q", ack.GetMetadata()["movement_action_contract_hash"])
	}
}

func TestRuntimeSnapshotAdvancesAndCompletesActionInstance(t *testing.T) {
	t.Parallel()

	runtime := NewRuntimeWithOptions(RecoveredRuntimeContracts(), RuntimeOptions{MovementValidation: true})
	sessionID := "runtime-integration-action-instance-snapshot"
	if _, err := runtime.OpenSession(context.Background(), &gamev1.OpenSessionRequest{
		Context: &gamev1.RequestContext{SessionId: sessionID},
	}); err != nil {
		t.Fatalf("OpenSession failed: %v", err)
	}
	if _, err := runtime.AttachPlayer(context.Background(), &gamev1.AttachPlayerRequest{
		Context:  &gamev1.RequestContext{SessionId: sessionID},
		PlayerId: "local_player",
	}); err != nil {
		t.Fatalf("AttachPlayer failed: %v", err)
	}
	player := runtime.ensurePlayerLocked("local_player")

	if _, err := runtime.SubmitCommand(context.Background(), testRuntimeCastSkillCommand(sessionID, 1, "player_basic_attack_1", gamev1Vector(1, 0, 0))); err != nil {
		t.Fatalf("SubmitCommand failed: %v", err)
	}
	if player.actionInstance == nil {
		t.Fatal("action instance missing after cast")
	}

	player.actionInstance.StartedAt = time.Now().Add(-2 * time.Second)
	resp, err := runtime.GetSnapshot(context.Background(), &gamev1.SnapshotRequest{
		Context:          &gamev1.RequestContext{SessionId: sessionID},
		IncludeFullState: true,
	})
	if err != nil {
		t.Fatalf("GetSnapshot failed: %v", err)
	}
	if len(resp.GetEntities()) == 0 {
		t.Fatal("snapshot has no entities")
	}
	var playerEntity *gamev1.SnapshotEntity
	for _, entity := range resp.GetEntities() {
		if entity.GetRef().GetEntityType() == "player" {
			playerEntity = entity
			break
		}
	}
	if playerEntity == nil {
		t.Fatal("player entity missing")
	}
	if playerEntity.GetSkillRuntimeState().GetState() != "complete" {
		t.Fatalf("skill state = %q, want complete", playerEntity.GetSkillRuntimeState().GetState())
	}
	if playerEntity.GetSkillState() != "idle" {
		t.Fatalf("skill_state = %q, want idle", playerEntity.GetSkillState())
	}
	if playerEntity.GetCombatState() != "ready" {
		t.Fatalf("combat_state = %q, want ready", playerEntity.GetCombatState())
	}
}

func TestRuntimeGroundedSkillMotionProgressesBySnapshotAndOwnsRoot(t *testing.T) {
	t.Parallel()

	runtime := NewRuntimeWithOptions(RecoveredRuntimeContracts(), RuntimeOptions{MovementValidation: true})
	sessionID := "runtime-integration-skill-root-motion"
	if _, err := runtime.OpenSession(context.Background(), &gamev1.OpenSessionRequest{
		Context: &gamev1.RequestContext{SessionId: sessionID},
	}); err != nil {
		t.Fatalf("OpenSession failed: %v", err)
	}
	if _, err := runtime.AttachPlayer(context.Background(), &gamev1.AttachPlayerRequest{
		Context:  &gamev1.RequestContext{SessionId: sessionID},
		PlayerId: "local_player",
	}); err != nil {
		t.Fatalf("AttachPlayer failed: %v", err)
	}

	player := runtime.ensurePlayerLocked("local_player")
	start := player.position
	contract := runtime.contracts.skillContract("player_shield_rush").MovementAction

	ack, err := runtime.SubmitCommand(context.Background(), testRuntimeCastSkillCommand(sessionID, 1, "player_shield_rush", gamev1Vector(1, 0, 0)))
	if err != nil {
		t.Fatalf("SubmitCommand failed: %v", err)
	}
	if !ack.GetAccepted() {
		t.Fatalf("cast rejected: %s %s", ack.GetRejectionCode(), ack.GetMessage())
	}
	if player.actionMotion == nil {
		t.Fatal("skill with movement did not create an owned action motion")
	}
	if moved := distance(start, player.position); moved > 1 {
		t.Fatalf("cast applied final displacement immediately: moved %.2fcm", moved)
	}
	if player.locomotion == nil || player.locomotion.GetAction() != "grounded_skill" {
		t.Fatalf("skill locomotion missing: action=%s", safeAction(player.locomotion))
	}
	if player.locomotion.GetActionDistanceTraveled() != 0 {
		t.Fatalf("initial action distance = %v, want 0", player.locomotion.GetActionDistanceTraveled())
	}

	moveAck, err := runtime.SubmitCommand(context.Background(), testRuntimeMoveCommand(sessionID, 2, gamev1Vector(0, 1, 0), 1, true, nil))
	if err != nil {
		t.Fatalf("move during skill submit failed: %v", err)
	}
	if !moveAck.GetAccepted() {
		t.Fatalf("move during owned root should be buffered/accepted, not rejected: %s", moveAck.GetRejectionCode())
	}
	if player.actionMotion == nil {
		t.Fatal("move during owned root cleared the action motion")
	}
	if player.locomotion.GetAction() != "grounded_skill" {
		t.Fatalf("move stole locomotion ownership during skill: %q", player.locomotion.GetAction())
	}

	duration := durationFromMS(contract.DurationMS)
	if duration <= 0 {
		duration = time.Second
	}
	startedAt := time.Now().Add(-(duration / 2))
	player.actionMotion.StartedAt = startedAt
	player.actionInstance.StartedAt = startedAt
	if _, err := runtime.GetSnapshot(context.Background(), &gamev1.SnapshotRequest{
		Context:          &gamev1.RequestContext{SessionId: sessionID},
		IncludeFullState: true,
	}); err != nil {
		t.Fatalf("GetSnapshot mid-action failed: %v", err)
	}
	midDistance := distance(start, player.position)
	if midDistance <= 0 || midDistance >= contract.DistanceCM {
		t.Fatalf("mid-action distance = %.2f, want between 0 and %.2f", midDistance, contract.DistanceCM)
	}
	if player.locomotion == nil {
		t.Fatal("mid-action locomotion missing")
	}
	if player.locomotion.GetPhaseElapsedMs() <= 0 {
		t.Fatalf("mid-action phase elapsed = %d, want > 0", player.locomotion.GetPhaseElapsedMs())
	}
	if player.locomotion.GetPhaseRemainingMs() <= 0 {
		t.Fatalf("mid-action phase remaining = %d, want > 0", player.locomotion.GetPhaseRemainingMs())
	}
	if player.locomotion.GetStartupMs()+player.locomotion.GetActiveMs()+player.locomotion.GetRecoveryMs() != player.locomotion.GetDurationMs() {
		t.Fatalf("locomotion timing envelope mismatch: startup=%d active=%d recovery=%d duration=%d",
			player.locomotion.GetStartupMs(),
			player.locomotion.GetActiveMs(),
			player.locomotion.GetRecoveryMs(),
			player.locomotion.GetDurationMs())
	}

	forceCompleteRuntimeAction(t, runtime, sessionID, player)
	finalDistance := distance(start, player.position)
	if finalDistance < contract.DistanceCM-1 {
		t.Fatalf("final action distance = %.2f, want about %.2f", finalDistance, contract.DistanceCM)
	}
	if player.actionMotion != nil {
		t.Fatal("completed skill left actionMotion active")
	}
}

func TestRuntimePostSkillHandoffReturnsSprintStrafeForCurrentBulwarkSkills(t *testing.T) {
	t.Parallel()

	for _, skillID := range []string{
		"player_basic_attack_1",
		"player_basic_attack_2",
		"player_basic_attack_3",
		"player_shield_bash",
		"player_shield_rush",
	} {
		t.Run(skillID, func(t *testing.T) {
			t.Parallel()

			runtime := NewRuntimeWithOptions(RecoveredRuntimeContracts(), RuntimeOptions{MovementValidation: true})
			sessionID := "runtime-integration-post-skill-handoff-" + skillID
			if _, err := runtime.OpenSession(context.Background(), &gamev1.OpenSessionRequest{
				Context: &gamev1.RequestContext{SessionId: sessionID},
			}); err != nil {
				t.Fatalf("OpenSession failed: %v", err)
			}
			if _, err := runtime.AttachPlayer(context.Background(), &gamev1.AttachPlayerRequest{
				Context:  &gamev1.RequestContext{SessionId: sessionID},
				PlayerId: "local_player",
			}); err != nil {
				t.Fatalf("AttachPlayer failed: %v", err)
			}
			player := runtime.ensurePlayerLocked("local_player")
			dir := gamev1Vector(-0.7071067811865476, 0.7071067811865476, 0)
			yaw := 315.0

			castAck, err := runtime.SubmitCommand(context.Background(), testRuntimeCastSkillCommand(sessionID, 1, skillID, dir))
			if err != nil {
				t.Fatalf("cast submit failed: %v", err)
			}
			if !castAck.GetAccepted() {
				t.Fatalf("cast rejected: %s %s", castAck.GetRejectionCode(), castAck.GetMessage())
			}
			if player.locomotion == nil || player.locomotion.GetAction() != "grounded_skill" {
				t.Fatalf("cast did not own grounded skill root: %s", safeAction(player.locomotion))
			}

			moveDuringAck, err := runtime.SubmitCommand(context.Background(), testRuntimeMoveCommand(sessionID, 2, dir, 1, true, &yaw))
			if err != nil {
				t.Fatalf("move during skill failed: %v", err)
			}
			if !moveDuringAck.GetAccepted() {
				t.Fatalf("move during skill rejected: %s %s", moveDuringAck.GetRejectionCode(), moveDuringAck.GetMessage())
			}
			if player.locomotion.GetAction() != "grounded_skill" {
				t.Fatalf("move stole skill root before handoff: %q", player.locomotion.GetAction())
			}

			forceCompleteRuntimeAction(t, runtime, sessionID, player)
			if player.actionMotion != nil {
				t.Fatal("completed skill left action motion active")
			}
			moveAfterAck, err := runtime.SubmitCommand(context.Background(), testRuntimeMoveCommand(sessionID, 3, dir, 1, true, &yaw))
			if err != nil {
				t.Fatalf("move after skill failed: %v", err)
			}
			if !moveAfterAck.GetAccepted() {
				t.Fatalf("move after skill rejected: %s %s", moveAfterAck.GetRejectionCode(), moveAfterAck.GetMessage())
			}
			if player.locomotion == nil || player.locomotion.GetAction() != "move" {
				t.Fatalf("post-skill handoff did not return normal movement: %s", safeAction(player.locomotion))
			}
			if player.locomotion.GetTargetSpeed() <= 0 || player.locomotion.GetActionDistanceTraveled() <= 0 {
				t.Fatalf("post-skill movement did not progress: speed=%.2f distance=%.2f", safeSpeed(player.locomotion), safeDistance(player.locomotion))
			}
		})
	}
}

func testRuntimeMoveCommand(
	sessionID string,
	sequence uint64,
	direction *gamev1.Vector3,
	analog float64,
	sprint bool,
	targetYaw *float64,
) *gamev1.PlayerCommand {
	return &gamev1.PlayerCommand{
		Context:              &gamev1.RequestContext{SessionId: sessionID},
		CommandId:            fmt.Sprintf("move-%d", sequence),
		Sequence:             sequence,
		Type:                 gamev1.CommandType_COMMAND_TYPE_MOVE,
		ClientTick:           sequence,
		ClientActionSequence: sequence,
		Payload: &gamev1.PlayerCommand_Move{
			Move: &gamev1.MoveCommand{
				Direction:       direction,
				DesiredPosition: nil,
				AnalogMagnitude: analog,
				Sprint:          sprint,
				TargetYaw:       targetYaw,
			},
		},
	}
}

func testRuntimeTurnCommand(sessionID string, sequence uint64, targetYaw float64) *gamev1.PlayerCommand {
	return &gamev1.PlayerCommand{
		Context:              &gamev1.RequestContext{SessionId: sessionID},
		CommandId:            fmt.Sprintf("turn-%d", sequence),
		Sequence:             sequence,
		Type:                 gamev1.CommandType_COMMAND_TYPE_TURN,
		ClientTick:           sequence,
		ClientActionSequence: sequence,
		Payload: &gamev1.PlayerCommand_Turn{
			Turn: &gamev1.TurnCommand{
				TargetYaw:  targetYaw,
				CurrentYaw: targetYaw,
			},
		},
	}
}

func testRuntimeCastSkillCommand(sessionID string, sequence uint64, skillID string, aimDirection *gamev1.Vector3) *gamev1.PlayerCommand {
	return &gamev1.PlayerCommand{
		Context:              &gamev1.RequestContext{SessionId: sessionID},
		CommandId:            fmt.Sprintf("cast-%d", sequence),
		Sequence:             sequence,
		Type:                 gamev1.CommandType_COMMAND_TYPE_CAST_SKILL,
		ClientTick:           sequence,
		ClientActionSequence: sequence,
		Payload: &gamev1.PlayerCommand_CastSkill{
			CastSkill: &gamev1.CastSkillCommand{
				SkillId:      skillID,
				AimDirection: aimDirection,
			},
		},
	}
}

func gamev1Vector(x, y, z float64) *gamev1.Vector3 {
	return &gamev1.Vector3{X: x, Y: y, Z: z}
}

func forceCompleteRuntimeAction(t *testing.T, runtime *Runtime, sessionID string, player *entityState) {
	t.Helper()
	startedAt := time.Now().Add(-2 * time.Second)
	if player.actionMotion != nil {
		duration := durationFromMS(player.actionMotion.Contract.DurationMS)
		if duration <= 0 {
			duration = 2 * time.Second
		}
		startedAt = time.Now().Add(-duration - 100*time.Millisecond)
		player.actionMotion.StartedAt = startedAt
	}
	if player.actionInstance != nil {
		player.actionInstance.StartedAt = startedAt
	}
	if _, err := runtime.GetSnapshot(context.Background(), &gamev1.SnapshotRequest{
		Context:          &gamev1.RequestContext{SessionId: sessionID},
		IncludeFullState: true,
	}); err != nil {
		t.Fatalf("GetSnapshot force-complete failed: %v", err)
	}
}

func safeAction(loco *gamev1.LocomotionState) string {
	if loco == nil {
		return "<nil>"
	}
	return loco.GetAction()
}

func safeDistance(loco *gamev1.LocomotionState) float64 {
	if loco == nil {
		return -1
	}
	return loco.GetActionDistanceTraveled()
}

func safeSpeed(loco *gamev1.LocomotionState) float64 {
	if loco == nil {
		return -1
	}
	return loco.GetTargetSpeed()
}
