package region

import (
	domainentity "server-apeiron/internal/domain/entity"
	"server-apeiron/internal/domain/ids"
	domainmath "server-apeiron/internal/domain/math"
	"server-apeiron/internal/domain/world"
)

type RegionRuntime struct {
	entities *EntityStore
	pkg      world.Package
	boundary Boundary
}

func NewRegionRuntime() *RegionRuntime {
	return &RegionRuntime{entities: NewEntityStore()}
}

func (r *RegionRuntime) Entities() *EntityStore {
	if r.entities == nil {
		r.entities = NewEntityStore()
	}
	return r.entities
}

func (r *RegionRuntime) Package() world.Package {
	return r.pkg
}

func (r *RegionRuntime) Boundary() Boundary {
	return r.boundary
}

type EntityStore struct {
	byID map[ids.RuntimeEntityID]domainentity.Entity
}

func NewEntityStore() *EntityStore {
	return &EntityStore{byID: make(map[ids.RuntimeEntityID]domainentity.Entity)}
}

func (s *EntityStore) All() []domainentity.Entity {
	out := make([]domainentity.Entity, 0, len(s.byID))
	for _, entity := range s.byID {
		out = append(out, entity)
	}
	return out
}

func (s *EntityStore) Get(id ids.RuntimeEntityID) (domainentity.Entity, bool) {
	entity, ok := s.byID[id]
	return entity, ok
}

type Boundary struct {
	Box domainmath.AABB
}

func (b Boundary) Bounds() domainmath.AABB {
	return b.Box
}
