package gameapi

import (
	"context"
	"testing"

	gamev1 "server-apeiron/gen/apeiron/game/v1"
)

// TestSkillRejectedDuringLeap restores the chat 6 #3 rule in the live runtime: a
// cast/basic during an owned movement action (leap/dodge) must be rejected as
// action_locked instead of being applied. The user reported being able to cast skills
// mid-jump/dodge, which should be forbidden.
func TestSkillRejectedDuringLeap(t *testing.T) {
	t.Parallel()

	runtime := NewRuntimeWithOptions(DevFixtureRuntimeContracts(), RuntimeOptions{MovementValidation: true})
	sessionID := "runtime-skill-gate"

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

	leap := &gamev1.PlayerCommand{
		Context:   &gamev1.RequestContext{SessionId: sessionID},
		CommandId: "leap-1",
		Sequence:  1,
		Type:      gamev1.CommandType_COMMAND_TYPE_LEAP,
		Payload: &gamev1.PlayerCommand_Leap{
			Leap: &gamev1.LeapCommand{Direction: gamev1Vector(1, 0, 0)},
		},
	}
	if _, err := runtime.SubmitCommand(context.Background(), leap); err != nil {
		t.Fatalf("leap submit failed: %v", err)
	}

	// Immediately try to cast while still inside the leap action window.
	ack, err := runtime.SubmitCommand(context.Background(),
		testRuntimeCastSkillCommand(sessionID, 2, "player_shield_rush", gamev1Vector(1, 0, 0)))
	if err != nil {
		t.Fatalf("cast submit failed: %v", err)
	}
	if ack.GetAccepted() {
		t.Fatal("skill during leap was accepted; expected action_locked rejection")
	}
	if ack.GetRejectionCode() != "action_locked" {
		t.Fatalf("rejection code = %q, want action_locked", ack.GetRejectionCode())
	}
}

func TestSkillRejectedDuringGroundedSkillRootMotion(t *testing.T) {
	t.Parallel()

	runtime := NewRuntimeWithOptions(DevFixtureRuntimeContracts(), RuntimeOptions{MovementValidation: true})
	sessionID := "runtime-grounded-skill-gate"

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

	first, err := runtime.SubmitCommand(context.Background(),
		testRuntimeCastSkillCommand(sessionID, 1, "player_shield_rush", gamev1Vector(1, 0, 0)))
	if err != nil {
		t.Fatalf("first cast submit failed: %v", err)
	}
	if !first.GetAccepted() {
		t.Fatalf("first cast rejected: %s %s", first.GetRejectionCode(), first.GetMessage())
	}

	second, err := runtime.SubmitCommand(context.Background(),
		testRuntimeCastSkillCommand(sessionID, 2, "player_basic_attack_1", gamev1Vector(1, 0, 0)))
	if err != nil {
		t.Fatalf("second cast submit failed: %v", err)
	}
	if second.GetAccepted() {
		t.Fatal("skill during grounded owned root was accepted; expected action_locked rejection")
	}
	if second.GetRejectionCode() != "action_locked" {
		t.Fatalf("rejection code = %q, want action_locked", second.GetRejectionCode())
	}
}
