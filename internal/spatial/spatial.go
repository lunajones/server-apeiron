package spatial

import (
	"sort"

	domainentity "server-apeiron/internal/domain/entity"
	"server-apeiron/internal/domain/ids"
	domainmath "server-apeiron/internal/domain/math"
)

type SpatialObject struct {
	EntityID ids.RuntimeEntityID
	Type     domainentity.EntityType
	Position domainmath.Position
	Bounds   domainmath.AABB
	Entity   domainentity.Entity
	Object   ObjectRef
}

type ObjectRef struct {
	ID ids.RuntimeEntityID
}

func SpatialObjectFromEntity(entity domainentity.Entity) SpatialObject {
	position := entity.Position()
	radius := entity.Radius()
	return SpatialObject{
		EntityID: entity.RuntimeID(),
		Type:     entity.EntityType(),
		Position: position,
		Bounds:   domainmath.AABBFromCenterSize(position, domainmath.V3(radius*2, radius*2, radius*2)),
		Entity:   entity,
		Object:   ObjectRef{ID: entity.RuntimeID()},
	}
}

type QueryFilter struct {
	Types    []domainentity.EntityType
	Exclude  map[ids.RuntimeEntityID]struct{}
	MaxCount int
	RegionID ids.RegionID
}

type AABBQuery struct {
	Bounds domainmath.AABB
	Filter QueryFilter
}

type SpatialIndex interface {
	Insert(SpatialObject) error
	Update(SpatialObject) error
	QueryAABB(AABBQuery) []SpatialObject
}

type LooseQuadtreeConfig struct {
	Center   domainmath.Position
	HalfSize float64
	MaxDepth int
	Capacity int
	Bounds   domainmath.AABB
}

type LooseQuadtree struct {
	objects map[ids.RuntimeEntityID]SpatialObject
}

func NewLooseQuadtree(LooseQuadtreeConfig) *LooseQuadtree {
	return &LooseQuadtree{objects: make(map[ids.RuntimeEntityID]SpatialObject)}
}

func (q *LooseQuadtree) Insert(object SpatialObject) error {
	if q.objects == nil {
		q.objects = make(map[ids.RuntimeEntityID]SpatialObject)
	}
	q.objects[object.EntityID] = object
	return nil
}

func (q *LooseQuadtree) Update(object SpatialObject) error {
	return q.Insert(object)
}

func (q *LooseQuadtree) QueryAABB(query AABBQuery) []SpatialObject {
	if q == nil {
		return nil
	}
	out := make([]SpatialObject, 0)
	for _, object := range q.objects {
		if !object.Bounds.Intersects(query.Bounds) {
			continue
		}
		if !query.Filter.accepts(object) {
			continue
		}
		out = append(out, object)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].EntityID < out[j].EntityID
	})
	if query.Filter.MaxCount > 0 && len(out) > query.Filter.MaxCount {
		return out[:query.Filter.MaxCount]
	}
	return out
}

func (f QueryFilter) accepts(object SpatialObject) bool {
	if _, excluded := f.Exclude[object.EntityID]; excluded {
		return false
	}
	if len(f.Types) == 0 {
		return true
	}
	for _, entityType := range f.Types {
		if object.Type == entityType {
			return true
		}
	}
	return false
}
