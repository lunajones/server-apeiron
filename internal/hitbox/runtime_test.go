package hitbox

import (
	"testing"
	"time"

	apeironv1 "db-apeiron/gen/apeiron/v1"
	domainentity "server-apeiron/internal/domain/entity"
	"server-apeiron/internal/domain/ids"
	domainmath "server-apeiron/internal/domain/math"
	"server-apeiron/internal/spatial"
)

type testEntity struct {
	id         ids.RuntimeEntityID
	entityType domainentity.EntityType
	position   domainmath.Position
	facing     domainmath.Vec3
	radius     float64
	components domainentity.Components
}

func newHitboxTestEntity(id ids.RuntimeEntityID, entityType domainentity.EntityType, position domainmath.Position) *testEntity {
	return &testEntity{
		id:         id,
		entityType: entityType,
		position:   position,
		facing:     domainmath.V3(1, 0, 0),
		radius:     35,
	}
}

func (e *testEntity) RuntimeID() ids.RuntimeEntityID { return e.id }
func (e *testEntity) Ref() domainentity.Ref {
	return domainentity.Ref{RuntimeID: e.id, Type: e.entityType}
}
func (e *testEntity) RegionID() ids.RegionID              { return ids.RegionID("test") }
func (e *testEntity) EntityType() domainentity.EntityType { return e.entityType }
func (e *testEntity) Position() domainmath.Position       { return e.position }
func (e *testEntity) Facing() domainmath.Vec3             { return e.facing }
func (e *testEntity) Radius() float64                     { return e.radius }
func (e *testEntity) SetPosition(position domainmath.Position) {
	e.position = position
	e.components.Transform.Position = position
}
func (e *testEntity) SetVelocity(velocity domainmath.Vec3) {
	e.components.Movement.Velocity = velocity
}
func (e *testEntity) Components() *domainentity.Components { return &e.components }

func TestEvaluateLimitsSingleTargetByForwardProgress(t *testing.T) {
	caster := newHitboxTestEntity(100, domainentity.EntityTypePlayer, domainmath.V3(0, 0, 0))
	near := newHitboxTestEntity(3, domainentity.EntityTypeCreature, domainmath.V3(80, 0, 0))
	far := newHitboxTestEntity(1, domainentity.EntityTypeCreature, domainmath.V3(160, 0, 0))
	side := newHitboxTestEntity(2, domainentity.EntityTypeCreature, domainmath.V3(120, 25, 0))

	index := spatial.NewLooseQuadtree(spatial.LooseQuadtreeConfig{})
	for _, entity := range []domainentity.Entity{caster, far, side, near} {
		if err := index.Insert(spatial.SpatialObjectFromEntity(entity)); err != nil {
			t.Fatalf("insert entity %d: %v", entity.RuntimeID(), err)
		}
	}

	now := time.UnixMilli(1000)
	one := int32(1)
	hits, err := NewRuntime().Evaluate(EvaluationContext{
		Caster:       caster,
		Skill:        &apeironv1.Skill{Id: "single_forward_hit", MaxTargets: 1},
		StartedAt:    now.Add(-50 * time.Millisecond),
		Now:          now,
		Origin:       caster.Position(),
		AimDirection: domainmath.V3(1, 0, 0),
		Spatial:      index,
		Hitboxes: []*apeironv1.SkillHitboxProfile{
			{
				Id:            "single_forward_hitbox",
				SkillId:       "single_forward_hit",
				HitboxShape:   "box",
				HitboxStartMs: 0,
				HitboxEndMs:   200,
				OffsetX:       120,
				Length:        240,
				SizeY:         120,
				SizeZ:         160,
				MaxTargets:    &one,
			},
		},
	})
	if err != nil {
		t.Fatalf("Evaluate returned error: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("hits = %d, want 1: %#v", len(hits), hits)
	}
	if hits[0].TargetID != near.RuntimeID() {
		t.Fatalf("target = %d, want nearest forward target %d", hits[0].TargetID, near.RuntimeID())
	}
	if hits[0].ForwardDistance <= 0 {
		t.Fatalf("forward distance = %.1f, want positive", hits[0].ForwardDistance)
	}
}
