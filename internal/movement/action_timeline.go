package movement

type TimelineClassification string

const (
	TimelineOnTime            TimelineClassification = "on_time"
	TimelineLateButReplayable TimelineClassification = "late_but_replayable"
	TimelineTooLateRejected   TimelineClassification = "too_late_rejected"
	TimelineFutureClamped     TimelineClassification = "future_clamped"
)

type ActionTimelinePolicy struct {
	MaxRetroactiveTicks uint64
	MaxFutureTicks      uint64
}

func DefaultActionTimelinePolicy() ActionTimelinePolicy {
	return ActionTimelinePolicy{MaxRetroactiveTicks: 3, MaxFutureTicks: 1}
}

type ActionTimelineDecision struct {
	ActionStartTick uint64
	Classification  TimelineClassification
	Rejected        bool
}

func ResolveActionTimeline(policy ActionTimelinePolicy, serverTick uint64, requestedTick uint64) ActionTimelineDecision {
	if requestedTick == serverTick {
		return ActionTimelineDecision{ActionStartTick: requestedTick, Classification: TimelineOnTime}
	}
	if requestedTick < serverTick {
		lateBy := serverTick - requestedTick
		if lateBy > policy.MaxRetroactiveTicks {
			return ActionTimelineDecision{ActionStartTick: serverTick, Classification: TimelineTooLateRejected, Rejected: true}
		}
		return ActionTimelineDecision{ActionStartTick: requestedTick, Classification: TimelineLateButReplayable}
	}
	futureBy := requestedTick - serverTick
	if futureBy > policy.MaxFutureTicks {
		return ActionTimelineDecision{ActionStartTick: serverTick + policy.MaxFutureTicks, Classification: TimelineFutureClamped}
	}
	return ActionTimelineDecision{ActionStartTick: requestedTick, Classification: TimelineFutureClamped}
}
