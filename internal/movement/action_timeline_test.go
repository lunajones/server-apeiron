package movement

import "testing"

func TestResolveActionTimelineOnTime(t *testing.T) {
	decision := ResolveActionTimeline(DefaultActionTimelinePolicy(), 100, 100)
	if decision.Rejected || decision.Classification != TimelineOnTime || decision.ActionStartTick != 100 {
		t.Fatalf("unexpected on-time decision: %+v", decision)
	}
}

func TestResolveActionTimelineLateButReplayable(t *testing.T) {
	decision := ResolveActionTimeline(ActionTimelinePolicy{MaxRetroactiveTicks: 3, MaxFutureTicks: 1}, 100, 98)
	if decision.Rejected || decision.Classification != TimelineLateButReplayable || decision.ActionStartTick != 98 {
		t.Fatalf("unexpected late replayable decision: %+v", decision)
	}
}

func TestResolveActionTimelineTooLateRejected(t *testing.T) {
	decision := ResolveActionTimeline(ActionTimelinePolicy{MaxRetroactiveTicks: 3, MaxFutureTicks: 1}, 100, 96)
	if !decision.Rejected || decision.Classification != TimelineTooLateRejected {
		t.Fatalf("expected too-late rejection, got %+v", decision)
	}
}

func TestResolveActionTimelineFutureClamped(t *testing.T) {
	decision := ResolveActionTimeline(ActionTimelinePolicy{MaxRetroactiveTicks: 3, MaxFutureTicks: 1}, 100, 105)
	if decision.Rejected || decision.Classification != TimelineFutureClamped || decision.ActionStartTick != 101 {
		t.Fatalf("unexpected future clamp decision: %+v", decision)
	}
}
