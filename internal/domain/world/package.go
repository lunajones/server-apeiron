package world

import domainmath "server-apeiron/internal/domain/math"

type Package struct {
	ID        string
	Blockers  []BlockerDefinition
	SafeZones []SafeZoneDefinition
}

type BlockerDefinition struct {
	ID        string
	Center    domainmath.Position
	Size      domainmath.Vec3
	BlocksLOS bool
	BlocksNav bool
}

func (b BlockerDefinition) Bounds() domainmath.AABB {
	return domainmath.AABBFromCenterSize(b.Center, b.Size)
}

type SafeZoneDefinition struct {
	ID     string
	Center domainmath.Position
	Radius float64
}
