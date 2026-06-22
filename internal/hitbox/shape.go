package hitbox

import (
	"time"

	domainmath "server-apeiron/internal/domain/math"
	"server-apeiron/internal/spatial"
)

type ShapeType string

const (
	ShapeArcSlice     ShapeType = "arc_slice"
	ShapeBoxStrip     ShapeType = "box_strip"
	ShapeCapsuleStrip ShapeType = "capsule_strip"
)

type RuntimeShape interface {
	Bounds() domainmath.AABB
	Contains(spatial.SpatialObject) bool
	ImpactPoint(spatial.SpatialObject) domainmath.Position
}

type Basis struct {
	Origin  domainmath.Position
	Forward domainmath.Vec3
	Right   domainmath.Vec3
	Up      domainmath.Vec3
}

func NewBasis(origin domainmath.Position, forward domainmath.Vec3) Basis {
	f := forward.Normalize()
	if f.Length() <= domainmath.Epsilon {
		f = domainmath.V3(1, 0, 0)
	}
	right := domainmath.V3(-f.Y, f.X, 0).Normalize()
	if right.Length() <= domainmath.Epsilon {
		right = domainmath.V3(0, 1, 0)
	}
	return Basis{Origin: origin, Forward: f, Right: right, Up: domainmath.V3(0, 0, 1)}
}

func (b Basis) Offset(forward, right, up float64) domainmath.Position {
	return b.Origin.Add(b.Forward.Mul(forward)).Add(b.Right.Mul(right)).Add(b.Up.Mul(up))
}

func (b Basis) Local(point domainmath.Position, origin domainmath.Position) domainmath.Vec3 {
	delta := point.Sub(origin)
	return domainmath.V3(delta.Dot(b.Forward), delta.Dot(b.Right), delta.Dot(b.Up))
}

func ms(value int32) time.Duration {
	return time.Duration(value) * time.Millisecond
}

func firstPositive(values ...float64) float64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func aabbSamplePoints(box domainmath.AABB, center domainmath.Position) []domainmath.Position {
	return []domainmath.Position{
		center,
		box.Min,
		box.Max,
		domainmath.V3(box.Min.X, box.Min.Y, box.Max.Z),
		domainmath.V3(box.Min.X, box.Max.Y, box.Min.Z),
		domainmath.V3(box.Max.X, box.Min.Y, box.Min.Z),
		domainmath.V3(box.Min.X, box.Max.Y, box.Max.Z),
		domainmath.V3(box.Max.X, box.Min.Y, box.Max.Z),
		domainmath.V3(box.Max.X, box.Max.Y, box.Min.Z),
	}
}
