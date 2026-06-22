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
	EntityType() EntityType
	Position() domainmath.Position
	Facing() domainmath.Vec3
	Radius() float64
	SetPosition(domainmath.Position)
	SetVelocity(domainmath.Vec3)
	Components() *Components
}

type Components struct {
	Skills   SkillComponent
	Movement MovementComponent
	Combat   CombatComponent
}

type SkillComponent struct {
	CurrentSkillID ids.SkillID
}

type MovementComponent struct {
	Velocity domainmath.Vec3
}

type CombatComponent struct {
	ControlImmuneUntil time.Time
	ActionLockedUntil  time.Time
}
