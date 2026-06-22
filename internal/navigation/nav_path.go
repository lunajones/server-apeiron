package navigation

import domainmath "server-apeiron/internal/domain/math"

type NavPath struct {
	Polygons []NavPolygonID
	Points   []domainmath.Vec3
	Cost     float64
	Partial  bool
}

func (p NavPath) Found() bool {
	return len(p.Polygons) > 0 && len(p.Points) > 0
}
