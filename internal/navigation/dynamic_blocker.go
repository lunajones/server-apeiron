package navigation

import (
	"server-apeiron/internal/domain/ids"
	domainmath "server-apeiron/internal/domain/math"
)

type DynamicBlocker struct {
	ID      ids.RuntimeEntityID
	Bounds  domainmath.AABB
	Enabled bool
}

func (b DynamicBlocker) Blocks(point domainmath.Vec3) bool {
	return b.Enabled && b.Bounds.ContainsPoint(point)
}
