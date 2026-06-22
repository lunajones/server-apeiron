package combat

import (
	domainentity "server-apeiron/internal/domain/entity"
	"server-apeiron/internal/domain/ids"
	domainmath "server-apeiron/internal/domain/math"
)

type testCombatEntity struct {
	id         ids.RuntimeEntityID
	regionID   ids.RegionID
	entityType domainentity.EntityType
	position   domainmath.Position
	facing     domainmath.Vec3
	radius     float64
	components domainentity.Components
}

func combatEntity(id ids.RuntimeEntityID, regionID ids.RegionID, x float64, y float64) *testCombatEntity {
	return &testCombatEntity{
		id:         id,
		regionID:   regionID,
		entityType: domainentity.EntityTypePlayer,
		position:   domainmath.V3(x, y, 0),
		facing:     domainmath.V3(1, 0, 0),
		radius:     45,
	}
}

func (e *testCombatEntity) RuntimeID() ids.RuntimeEntityID { return e.id }
func (e *testCombatEntity) Ref() domainentity.Ref {
	return domainentity.Ref{RuntimeID: e.id, Type: e.entityType}
}
func (e *testCombatEntity) RegionID() ids.RegionID              { return e.regionID }
func (e *testCombatEntity) EntityType() domainentity.EntityType { return e.entityType }
func (e *testCombatEntity) Position() domainmath.Position       { return e.position }
func (e *testCombatEntity) Facing() domainmath.Vec3             { return e.facing }
func (e *testCombatEntity) Radius() float64                     { return e.radius }
func (e *testCombatEntity) SetPosition(position domainmath.Position) {
	e.position = position
	e.components.Transform.Position = position
}
func (e *testCombatEntity) SetVelocity(velocity domainmath.Vec3) {
	e.components.Movement.Velocity = velocity
}
func (e *testCombatEntity) Components() *domainentity.Components {
	return &e.components
}
