package actionruntime

import (
	"fmt"
	"time"

	"server-apeiron/internal/domain/ids"
	domainmath "server-apeiron/internal/domain/math"
	"server-apeiron/internal/movement"
)

type ActorKind string
type ActionKind string
type Phase string

const (
	ActorKindPlayer ActorKind = "player"

	ActionKindWeaponBasic   ActionKind = "weapon_basic"
	ActionKindActiveSkill   ActionKind = "active_skill"
	ActionKindStatusControl ActionKind = "status_control"

	PhaseAccepted Phase = "accepted"
	PhaseWindup   Phase = "windup"
	PhaseActive   Phase = "active"
	PhaseRecovery Phase = "recovery"
	PhaseComplete Phase = "complete"
)

type Timing struct {
	Windup         time.Duration
	Active         time.Duration
	Recovery       time.Duration
	Cooldown       time.Duration
	ActionLock     time.Duration
	GlobalCooldown time.Duration
}

type Instance struct {
	InstanceID           string
	EntityID             ids.RuntimeEntityID
	ActorKind            ActorKind
	ActionKind           ActionKind
	SkillID              ids.SkillID
	CommandID            string
	CommandSequence      uint64
	ServerActionSequence uint64
	ClientTick           uint64
	StartedAt            time.Time
	Timing               Timing
	Cooldown             time.Duration
	MovementContract     movement.MovementActionContract
	HasMovementContract  bool
	ActionStartPosition  domainmath.Position
	MovementLockedUntil  time.Time
	GlobalLockedUntil    time.Time
	RecoveryEndsAt       time.Time
}

type NewInstanceSpec struct {
	InstanceID           string
	EntityID             ids.RuntimeEntityID
	ActorKind            ActorKind
	ActionKind           ActionKind
	SkillID              ids.SkillID
	CommandID            string
	CommandSequence      uint64
	ServerActionSequence uint64
	ClientTick           uint64
	StartedAt            time.Time
	Timing               Timing
	Cooldown             time.Duration
	MovementContract     movement.MovementActionContract
	HasMovementContract  bool
	ActionStartPosition  domainmath.Position
	MovementLockedUntil  time.Time
	GlobalLockedUntil    time.Time
	RecoveryEndsAt       time.Time
}

func NewInstance(spec NewInstanceSpec) Instance {
	return Instance{
		InstanceID:           spec.InstanceID,
		EntityID:             spec.EntityID,
		ActorKind:            spec.ActorKind,
		ActionKind:           spec.ActionKind,
		SkillID:              spec.SkillID,
		CommandID:            spec.CommandID,
		CommandSequence:      spec.CommandSequence,
		ServerActionSequence: spec.ServerActionSequence,
		ClientTick:           spec.ClientTick,
		StartedAt:            spec.StartedAt,
		Timing:               spec.Timing,
		Cooldown:             spec.Cooldown,
		MovementContract:     spec.MovementContract,
		HasMovementContract:  spec.HasMovementContract,
		ActionStartPosition:  spec.ActionStartPosition,
		MovementLockedUntil:  spec.MovementLockedUntil,
		GlobalLockedUntil:    spec.GlobalLockedUntil,
		RecoveryEndsAt:       spec.RecoveryEndsAt,
	}
}

func (i Instance) LockRemaining(now time.Time) time.Duration {
	until := i.StartedAt.Add(i.Timing.Windup + i.Timing.Active + i.Timing.Recovery)
	if i.RecoveryEndsAt.After(until) {
		until = i.RecoveryEndsAt
	}
	if until.After(now) {
		return until.Sub(now)
	}
	return 0
}

func (i Instance) GlobalCooldownRemaining(now time.Time) time.Duration {
	until := i.StartedAt.Add(i.Cooldown)
	if i.GlobalLockedUntil.After(until) {
		until = i.GlobalLockedUntil
	}
	if until.After(now) {
		return until.Sub(now)
	}
	return 0
}

func (i Instance) PhaseAt(now time.Time) Phase {
	if i.StartedAt.IsZero() || now.Before(i.StartedAt) {
		return PhaseAccepted
	}
	elapsed := now.Sub(i.StartedAt)
	if elapsed < i.Timing.Windup {
		return PhaseWindup
	}
	if elapsed < i.Timing.Windup+i.Timing.Active {
		return PhaseActive
	}
	if elapsed < i.Timing.Windup+i.Timing.Active+i.Timing.Recovery {
		return PhaseRecovery
	}
	return PhaseComplete
}

func NewInstanceID(entityID ids.RuntimeEntityID, skillID string, commandID string, clientSequence uint64, serverSequence uint64) string {
	return fmt.Sprintf("runtime:%d:%s:%s:%d:%d", entityID, skillID, commandID, clientSequence, serverSequence)
}

func CanQueueTiming(timing Timing, remaining time.Duration) bool {
	window := timing.Recovery
	if window <= 0 {
		window = 250 * time.Millisecond
	}
	return remaining <= window
}

func CanCancelIntoTiming(timing Timing, startedAt time.Time, now time.Time) bool {
	if startedAt.IsZero() || now.Before(startedAt) {
		return false
	}
	return now.Sub(startedAt) >= timing.Active
}
