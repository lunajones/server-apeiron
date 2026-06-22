package entity

import (
	"time"

	"server-apeiron/internal/domain/ids"
	domainmath "server-apeiron/internal/domain/math"
)

type EntityType string

const (
	EntityTypePlayer   EntityType = "player"
	EntityTypeCreature EntityType = "creature"
)

type Entity interface {
	RuntimeID() ids.RuntimeEntityID
	Ref() Ref
	RegionID() ids.RegionID
	EntityType() EntityType
	Position() domainmath.Position
	Facing() domainmath.Vec3
	Radius() float64
	SetPosition(domainmath.Position)
	SetVelocity(domainmath.Vec3)
	Components() *Components
}

type Ref struct {
	RuntimeID ids.RuntimeEntityID
	Type      EntityType
}

type Components struct {
	Skills    SkillComponent
	Movement  MovementComponent
	Combat    CombatComponent
	Transform TransformComponent
	Status    StatusComponent
}

type SkillComponent struct {
	CurrentSkillID   ids.SkillID
	State            string
	StartedAtMS      int64
	CooldownEndMS    int64
	LastResolvedAtMS int64
}

type MovementComponent struct {
	Velocity                domainmath.Vec3
	MovementLocked          bool
	Locomotion              LocomotionComponent
	LastProcessedSequence   uint64
	LastProcessedClientTick uint64
}

type LocomotionComponent struct {
	AuthoritativeYaw float64
}

type CombatComponent struct {
	ControlImmuneUntil time.Time
	ActionLockedUntil  time.Time
}

type TransformComponent struct {
	Position  domainmath.Position
	Facing    domainmath.Vec3
	RotationY float64
	Radius    float64
}

type StatusComponent struct {
	Effects     map[string]time.Time
	Stunned     bool
	Staggered   bool
	Silenced    bool
	Rooted      bool
	KnockedDown bool
	Attached    bool
	CCEndMS     int64
}
