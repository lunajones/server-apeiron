package gameapi

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	gamev1 "server-apeiron/gen/apeiron/game/v1"
	"server-apeiron/internal/combat/actionruntime"
)

type runtimeGuardHarness struct {
	t         *testing.T
	runtime   *Runtime
	sessionID string
	player    *entityState
	sequence  uint64
}

func newRuntimeGuardHarness(t *testing.T, sessionID string) *runtimeGuardHarness {
	t.Helper()
	runtime := NewRuntimeWithOptions(RecoveryFixtureRuntimeContracts(), RuntimeOptions{MovementValidation: true})
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
	return &runtimeGuardHarness{
		t:         t,
		runtime:   runtime,
		sessionID: sessionID,
		player:    runtime.ensurePlayerLocked("local_player"),
		sequence:  1,
	}
}

func (h *runtimeGuardHarness) nextSequence() uint64 {
	h.sequence++
	return h.sequence - 1
}

func (h *runtimeGuardHarness) submit(cmd *gamev1.PlayerCommand) *gamev1.CommandAck {
	h.t.Helper()
	ack, err := h.runtime.SubmitCommand(context.Background(), cmd)
	if err != nil {
		h.t.Fatalf("SubmitCommand failed (%s seq=%d): %v", commandTypeName(cmd.GetType()), cmd.GetSequence(), err)
	}
	if ack == nil {
		h.t.Fatalf("nil ack for %s seq=%d", commandTypeName(cmd.GetType()), cmd.GetSequence())
	}
	if !ack.GetAccepted() {
		h.t.Fatalf("command rejected: type=%s seq=%d code=%s message=%s metadata=%v", commandTypeName(cmd.GetType()), cmd.GetSequence(), ack.GetRejectionCode(), ack.GetMessage(), ack.GetMetadata())
	}
	return ack
}

func (h *runtimeGuardHarness) move(direction *gamev1.Vector3, sprint bool, yaw *float64) *gamev1.CommandAck {
	h.t.Helper()
	return h.submit(testRuntimeMoveCommand(h.sessionID, h.nextSequence(), direction, 1, sprint, yaw))
}

func (h *runtimeGuardHarness) turn(yaw float64) *gamev1.CommandAck {
	h.t.Helper()
	return h.submit(testRuntimeTurnCommand(h.sessionID, h.nextSequence(), yaw))
}

func (h *runtimeGuardHarness) cast(skillID string, aim *gamev1.Vector3) *gamev1.CommandAck {
	h.t.Helper()
	return h.submit(testRuntimeCastSkillCommand(h.sessionID, h.nextSequence(), skillID, aim))
}

func (h *runtimeGuardHarness) dodge(direction *gamev1.Vector3) *gamev1.CommandAck {
	h.t.Helper()
	return h.submit(testRuntimeDodgeCommand(h.sessionID, h.nextSequence(), direction))
}

func (h *runtimeGuardHarness) leap(direction *gamev1.Vector3) *gamev1.CommandAck {
	h.t.Helper()
	return h.submit(testRuntimeLeapCommand(h.sessionID, h.nextSequence(), direction))
}

func (h *runtimeGuardHarness) forceComplete() {
	h.t.Helper()
	forceCompleteRuntimeAction(h.t, h.runtime, h.sessionID, h.player)
}

func (h *runtimeGuardHarness) snapshotAtActionElapsed(elapsed time.Duration) {
	h.t.Helper()
	startedAt := time.Now().Add(-elapsed)
	if h.player.actionMotion != nil {
		h.player.actionMotion.StartedAt = startedAt
	}
	if h.player.actionInstance != nil {
		h.player.actionInstance.StartedAt = startedAt
	}
	if _, err := h.runtime.GetSnapshot(context.Background(), &gamev1.SnapshotRequest{
		Context:          &gamev1.RequestContext{SessionId: h.sessionID},
		IncludeFullState: true,
	}); err != nil {
		h.t.Fatalf("GetSnapshot failed: %v", err)
	}
}

func TestRubberbandGuardStationaryPlayerSkillsAreActionOwnedAndHandoffCleanly(t *testing.T) {
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
			h := newRuntimeGuardHarness(t, "rubber-guard-stationary-"+skillID)
			start := h.player.position
			h.cast(skillID, gamev1Vector(1, 0, 0))

			assertSkillRootLocomotion(t, h.player, skillID)
			assertNoImmediateTeleport(t, start, h.player.position, skillID)
			assertActionMotionEnvelope(t, h.player, skillID)

			contract := h.runtime.contracts.skillContract(skillID).MovementAction
			h.snapshotAtActionElapsed(durationFromMS(contract.DurationMS) / 2)
			mid := h.player.position
			if movementDistance := distance(start, mid); contract.DistanceCM > 0 && (movementDistance <= 0 || movementDistance >= contract.DistanceCM) {
				t.Fatalf("%s mid-action distance %.2f outside expected action envelope %.2f", skillID, movementDistance, contract.DistanceCM)
			}
			assertSkillRootLocomotion(t, h.player, skillID)

			h.forceComplete()
			if h.player.actionMotion != nil {
				t.Fatalf("%s left action motion after forced completion", skillID)
			}
			h.move(gamev1Vector(0, 1, 0), true, floatPtr(90))
			assertGroundedMoveLocomotion(t, h.player, "post-stationary-"+skillID)
		})
	}
}

func TestRubberbandGuardSprintForwardBasicChainDoesNotLoseGroundedHandoff(t *testing.T) {
	t.Parallel()

	h := newRuntimeGuardHarness(t, "rubber-guard-sprint-forward-basic-chain")
	forward := gamev1Vector(1, 0, 0)
	yaw := 0.0
	chain := []string{"player_basic_attack_1", "player_basic_attack_2", "player_basic_attack_3"}

	for i := 0; i < 10; i++ {
		h.move(forward, true, &yaw)
		assertGroundedMoveLocomotion(t, h.player, fmt.Sprintf("pre-basic-%d", i))
		skillID := chain[i%len(chain)]
		start := h.player.position
		h.cast(skillID, forward)
		assertSkillRootLocomotion(t, h.player, skillID)
		assertNoImmediateTeleport(t, start, h.player.position, skillID)
		h.forceComplete()
		h.move(forward, true, &yaw)
		assertGroundedMoveLocomotion(t, h.player, fmt.Sprintf("post-basic-%d", i))
	}
}

func TestRubberbandGuardAggressiveYawSprintStrafeDoesNotEnterSkillOrTurnReconciliation(t *testing.T) {
	t.Parallel()

	h := newRuntimeGuardHarness(t, "rubber-guard-aggressive-yaw-strafe")
	left := gamev1Vector(-0.7071067811865476, 0.7071067811865476, 0)
	right := gamev1Vector(0.7071067811865476, 0.7071067811865476, 0)
	yaw := 90.0

	for i := 0; i < 24; i++ {
		var dir *gamev1.Vector3
		if i%2 == 0 {
			yaw -= 115
			h.turn(yaw)
			dir = left
			h.move(dir, true, &yaw)
		} else {
			yaw += 115
			h.turn(yaw)
			dir = right
			h.move(dir, true, &yaw)
		}
		assertGroundedMoveLocomotion(t, h.player, fmt.Sprintf("strafe-yaw-%d", i))
		expected := h.runtime.groundedMoveSpeed(true, 1, vector{x: dir.GetX(), y: dir.GetY()}, yaw)
		if expected > 0 && h.player.locomotion.GetTargetSpeed() < expected*0.60 {
			t.Fatalf("iteration %d target speed collapsed: got %.2f expected around %.2f", i, h.player.locomotion.GetTargetSpeed(), expected)
		}
	}
}

func TestRubberbandGuardLateralSprintDuringSkillCannotStealRoot(t *testing.T) {
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
			h := newRuntimeGuardHarness(t, "rubber-guard-lateral-during-skill-"+skillID)
			start := h.player.position
			h.cast(skillID, gamev1Vector(1, 0, 0))
			assertSkillRootLocomotion(t, h.player, skillID)

			yaw := 90.0
			h.move(gamev1Vector(0, 1, 0), true, &yaw)
			assertSkillRootLocomotion(t, h.player, skillID)
			if math.Abs(h.player.position.y-start.y) > 0.001 {
				t.Fatalf("%s lateral sprint changed Y during owned root: start=%.3f got=%.3f", skillID, start.y, h.player.position.y)
			}

			h.turn(180)
			assertSkillRootLocomotion(t, h.player, skillID)
			h.forceComplete()
			h.move(gamev1Vector(0, 1, 0), true, &yaw)
			assertGroundedMoveLocomotion(t, h.player, "post-lateral-root-"+skillID)
		})
	}
}

func TestRubberbandGuardLeapDodgeTurnBaselineSurvivesSkillPressure(t *testing.T) {
	t.Parallel()

	for _, scenario := range []struct {
		name       string
		command    func(h *runtimeGuardHarness)
		abilityKey string
		action     string
	}{
		{
			name:       "dodge",
			abilityKey: "dodge",
			action:     "dodge",
			command: func(h *runtimeGuardHarness) {
				h.dodge(gamev1Vector(0, 1, 0))
			},
		},
		{
			name:       "leap",
			abilityKey: "jump",
			action:     "leap",
			command: func(h *runtimeGuardHarness) {
				h.leap(gamev1Vector(1, 0, 0))
			},
		},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			h := newRuntimeGuardHarness(t, "rubber-guard-baseline-"+scenario.name)
			scenario.command(h)
			if h.player.actionMotion == nil {
				t.Fatalf("%s did not create owned action motion", scenario.name)
			}
			if h.player.actionMotion.Contract.AbilityKey != scenario.abilityKey {
				t.Fatalf("%s action ability = %q, want %q", scenario.name, h.player.actionMotion.Contract.AbilityKey, scenario.abilityKey)
			}
			if h.player.locomotion == nil || h.player.locomotion.GetAbilityKey() != scenario.abilityKey {
				t.Fatalf("%s locomotion ability = %q, want %q", scenario.name, safeAbility(h.player.locomotion), scenario.abilityKey)
			}

			ack, err := h.runtime.SubmitCommand(context.Background(), testRuntimeCastSkillCommand(h.sessionID, h.nextSequence(), "player_shield_rush", gamev1Vector(1, 0, 0)))
			if err != nil {
				t.Fatalf("skill during %s submit failed: %v", scenario.name, err)
			}
			if ack.GetAccepted() || ack.GetRejectionCode() != "action_locked" {
				t.Fatalf("skill during %s should be action_locked, ack=%#v", scenario.name, ack)
			}
			if h.player.actionMotion == nil {
				t.Fatalf("rejected skill cleared %s action motion", scenario.name)
			}

			h.forceComplete()
			h.move(gamev1Vector(1, 0, 0), true, floatPtr(0))
			assertGroundedMoveLocomotion(t, h.player, "post-"+scenario.name)
		})
	}
}

func TestRubberbandGuardRepeatedShieldSkillsWhileSprintingForward(t *testing.T) {
	t.Parallel()

	h := newRuntimeGuardHarness(t, "rubber-guard-repeated-shield-skills-forward")
	forward := gamev1Vector(1, 0, 0)
	yaw := 0.0
	skills := []string{"player_shield_bash", "player_shield_rush"}

	for i := 0; i < 8; i++ {
		h.move(forward, true, &yaw)
		assertGroundedMoveLocomotion(t, h.player, fmt.Sprintf("pre-shield-%d", i))
		skillID := skills[i%len(skills)]
		start := h.player.position
		h.cast(skillID, forward)
		assertSkillRootLocomotion(t, h.player, skillID)
		assertNoImmediateTeleport(t, start, h.player.position, skillID)
		h.forceComplete()
		h.move(forward, true, &yaw)
		assertGroundedMoveLocomotion(t, h.player, fmt.Sprintf("post-shield-%d", i))
	}
}

func assertGroundedMoveLocomotion(t *testing.T, player *entityState, label string) {
	t.Helper()
	if player == nil || player.locomotion == nil {
		t.Fatalf("%s: locomotion missing", label)
	}
	if player.locomotion.GetAction() != "move" {
		t.Fatalf("%s: action=%q want move", label, player.locomotion.GetAction())
	}
	if player.locomotion.GetAbilityKey() != "move" {
		t.Fatalf("%s: ability=%q want move", label, player.locomotion.GetAbilityKey())
	}
	if player.locomotion.GetReconciliationMode() != "grounded_move_reconciliation" {
		t.Fatalf("%s: reconciliation=%q want grounded_move_reconciliation", label, player.locomotion.GetReconciliationMode())
	}
	if player.locomotion.GetTargetSpeed() <= 0 || player.locomotion.GetEffectiveSpeed() <= 0 {
		t.Fatalf("%s: invalid speed target=%.2f effective=%.2f", label, player.locomotion.GetTargetSpeed(), player.locomotion.GetEffectiveSpeed())
	}
	if player.locomotion.GetActionDistanceTraveled() <= 0 {
		t.Fatalf("%s: no action distance", label)
	}
}

func assertSkillRootLocomotion(t *testing.T, player *entityState, skillID string) {
	t.Helper()
	if player == nil || player.locomotion == nil {
		t.Fatalf("%s: skill locomotion missing", skillID)
	}
	if player.locomotion.GetAction() != "grounded_skill" {
		t.Fatalf("%s: action=%q want grounded_skill", skillID, player.locomotion.GetAction())
	}
	if player.locomotion.GetAbilityKey() != skillID {
		t.Fatalf("%s: ability=%q want %q", skillID, player.locomotion.GetAbilityKey(), skillID)
	}
	if player.locomotion.GetReconciliationMode() != "grounded_skill_action" {
		t.Fatalf("%s: reconciliation=%q want grounded_skill_action", skillID, player.locomotion.GetReconciliationMode())
	}
	if player.actionMotion == nil {
		t.Fatalf("%s: actionMotion missing", skillID)
	}
	if player.actionMotion.SkillID != skillID {
		t.Fatalf("%s: actionMotion skill=%q", skillID, player.actionMotion.SkillID)
	}
}

func assertActionMotionEnvelope(t *testing.T, player *entityState, skillID string) {
	t.Helper()
	if player == nil || player.actionMotion == nil || player.actionInstance == nil {
		t.Fatalf("%s: missing action motion/instance", skillID)
	}
	phase := player.actionInstance.PhaseAt(time.Now())
	if phase == actionruntime.PhaseComplete {
		t.Fatalf("%s: action instance completed immediately", skillID)
	}
	if player.actionMotion.Contract.ID == "" {
		t.Fatalf("%s: missing movement action contract id", skillID)
	}
	if player.actionMotion.NormalInputPolicy == "" {
		t.Fatalf("%s: missing normal input policy", skillID)
	}
	if !blocksNormalInputDuringOwnedRoot(player.actionMotion.NormalInputPolicy) {
		t.Fatalf("%s: normal input policy does not protect owned root: %q", skillID, player.actionMotion.NormalInputPolicy)
	}
}

func assertNoImmediateTeleport(t *testing.T, before vector, after vector, label string) {
	t.Helper()
	if moved := distance(before, after); moved > 1 {
		t.Fatalf("%s moved %.2fcm on submit; action movement must advance through snapshots", label, moved)
	}
}

func safeAbility(loco *gamev1.LocomotionState) string {
	if loco == nil {
		return "<nil>"
	}
	return loco.GetAbilityKey()
}

func floatPtr(value float64) *float64 {
	return &value
}

func testRuntimeDodgeCommand(sessionID string, sequence uint64, direction *gamev1.Vector3) *gamev1.PlayerCommand {
	return &gamev1.PlayerCommand{
		Context:              &gamev1.RequestContext{SessionId: sessionID},
		CommandId:            fmt.Sprintf("dodge-guard-%d", sequence),
		Sequence:             sequence,
		Type:                 gamev1.CommandType_COMMAND_TYPE_DODGE,
		ClientTick:           sequence,
		ClientActionSequence: sequence,
		Payload: &gamev1.PlayerCommand_Dodge{
			Dodge: &gamev1.DodgeCommand{Direction: direction},
		},
	}
}

func testRuntimeLeapCommand(sessionID string, sequence uint64, direction *gamev1.Vector3) *gamev1.PlayerCommand {
	return &gamev1.PlayerCommand{
		Context:              &gamev1.RequestContext{SessionId: sessionID},
		CommandId:            fmt.Sprintf("leap-guard-%d", sequence),
		Sequence:             sequence,
		Type:                 gamev1.CommandType_COMMAND_TYPE_LEAP,
		ClientTick:           sequence,
		ClientActionSequence: sequence,
		Payload: &gamev1.PlayerCommand_Leap{
			Leap: &gamev1.LeapCommand{Direction: direction},
		},
	}
}
