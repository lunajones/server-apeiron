package gameapi

import (
	"strings"
	"time"

	"server-apeiron/internal/combat/actionruntime"
	"server-apeiron/internal/movement"
)

type creatureActionMovementEnvelope struct {
	SkillID              string
	ContactPolicy        string
	MovementStartsAt     time.Time
	MovementEndsAt       time.Time
	AirborneStartsAt     time.Time
	AirborneEndsAt       time.Time
	LandingStartsAt      time.Time
	LandingEndsAt        time.Time
	HandoffAt            time.Time
	RootMotionActive     bool
	AirborneActive       bool
	LandingInertiaActive bool
	Complete             bool
	PreCommitActive      bool
	AllowsPassthrough    bool
	StopsAtContact       bool
}

type creatureSkillMovementPresentation struct {
	MovementStartMS   int32
	TakeoffMS         int32
	LandingLockMS     int32
	MovementDuration  int32
	MovementDistance  float64
	StopAtContactRate float64
}

func creatureActionMovementEnvelopeAt(instance actionruntime.Instance, contract SkillRuntimeContract, now time.Time) creatureActionMovementEnvelope {
	rootStartOffset := creatureSkillMovementStartOffset(instance.Timing, contract)
	movementDuration := movement.ActionDuration(contract.MovementAction)
	rootStart := instance.StartedAt.Add(rootStartOffset)
	rootEnd := rootStart.Add(movementDuration)
	preCommitDuration := creatureSkillPreCommitDuration(contract)
	airborneDuration := creatureSkillAirborneDuration(contract)
	airborneStart := rootStart.Add(preCommitDuration)
	if airborneStart.After(rootEnd) {
		airborneStart = rootEnd
	}
	airborneEnd := airborneStart.Add(airborneDuration)
	if airborneDuration <= 0 || airborneEnd.After(rootEnd) {
		airborneEnd = airborneStart
	}
	handoff := rootEnd
	if actionEnd := instance.StartedAt.Add(instance.Timing.Windup + instance.Timing.Active + instance.Timing.Recovery); actionEnd.After(handoff) {
		handoff = actionEnd
	}
	contact := creatureActionContactRuntimeFromContract(contract)
	envelope := creatureActionMovementEnvelope{
		SkillID:           contract.SkillID,
		ContactPolicy:     contact.Policy,
		MovementStartsAt:  rootStart,
		MovementEndsAt:    rootEnd,
		AirborneStartsAt:  airborneStart,
		AirborneEndsAt:    airborneEnd,
		LandingStartsAt:   airborneEnd,
		LandingEndsAt:     rootEnd,
		HandoffAt:         handoff,
		AllowsPassthrough: contact.AllowsPassthrough,
		StopsAtContact:    contact.StopsAtContact,
	}
	if movementDuration <= 0 {
		envelope.MovementEndsAt = rootStart
		envelope.LandingEndsAt = rootStart
		envelope.HandoffAt = instance.StartedAt.Add(instance.Timing.Windup + instance.Timing.Active + instance.Timing.Recovery)
	}
	envelope.RootMotionActive = !now.Before(envelope.MovementStartsAt) && now.Before(envelope.MovementEndsAt)
	envelope.PreCommitActive = preCommitDuration > 0 && !now.Before(envelope.MovementStartsAt) && now.Before(envelope.AirborneStartsAt)
	envelope.AirborneActive = airborneDuration > 0 && !now.Before(envelope.AirborneStartsAt) && now.Before(envelope.AirborneEndsAt)
	envelope.LandingInertiaActive = envelope.LandingEndsAt.After(envelope.LandingStartsAt) && !now.Before(envelope.LandingStartsAt) && now.Before(envelope.LandingEndsAt)
	envelope.Complete = !now.Before(envelope.HandoffAt)
	return envelope
}

func creatureSkillMovementPresentationFromContract(contract SkillRuntimeContract) creatureSkillMovementPresentation {
	timing := creatureActionTimingFromSkillContract(contract)
	startOffset := creatureSkillMovementStartOffset(timing, contract)
	duration := movement.ActionDuration(contract.MovementAction)
	preCommit := creatureSkillPreCommitDuration(contract)
	airborne := creatureSkillAirborneDuration(contract)
	landing := duration - preCommit - airborne
	if landing < 0 {
		landing = 0
	}
	return creatureSkillMovementPresentation{
		MovementStartMS:   durationMillis(startOffset),
		TakeoffMS:         durationMillis(startOffset + preCommit),
		LandingLockMS:     durationMillis(landing),
		MovementDuration:  durationMillis(duration),
		MovementDistance:  movement.ActionDistance(contract.MovementAction, 0),
		StopAtContactRate: creatureSkillMovementStopAtContactRate(contract),
	}
}

func creatureSkillPreCommitDuration(contract SkillRuntimeContract) time.Duration {
	if contract.Envelope != nil && contract.Envelope.GetPreCommitMs() > 0 {
		return time.Duration(contract.Envelope.GetPreCommitMs()) * time.Millisecond
	}
	return 0
}

func creatureSkillAirborneDuration(contract SkillRuntimeContract) time.Duration {
	if contract.Envelope != nil && contract.Envelope.GetAirborneMs() > 0 {
		return time.Duration(contract.Envelope.GetAirborneMs()) * time.Millisecond
	}
	if contract.MovementAction.AirborneDurationMS > 0 {
		return time.Duration(contract.MovementAction.AirborneDurationMS) * time.Millisecond
	}
	if strings.EqualFold(contract.MovementAction.ActionType, "leap") && contract.MovementAction.ActiveMS > 0 {
		return time.Duration(contract.MovementAction.ActiveMS) * time.Millisecond
	}
	return 0
}

func creatureSkillMovementStopAtContactRate(contract SkillRuntimeContract) float64 {
	policy := creatureActionContactRuntimeFromContract(contract)
	if policy.AllowsPassthrough {
		return 1
	}
	if policy.StopsAtContact {
		return 0
	}
	return 1
}
