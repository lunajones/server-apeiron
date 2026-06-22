package math

import stdmath "math"

const Epsilon = 0.000001

type Vec3 struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

type Position = Vec3

func V3(x, y, z float64) Vec3 {
	return Vec3{X: x, Y: y, Z: z}
}

func (v Vec3) Add(other Vec3) Vec3 {
	return Vec3{X: v.X + other.X, Y: v.Y + other.Y, Z: v.Z + other.Z}
}

func (v Vec3) Sub(other Vec3) Vec3 {
	return Vec3{X: v.X - other.X, Y: v.Y - other.Y, Z: v.Z - other.Z}
}

func (v Vec3) Scale(scale float64) Vec3 {
	return Vec3{X: v.X * scale, Y: v.Y * scale, Z: v.Z * scale}
}

func (v Vec3) Dot(other Vec3) float64 {
	return v.X*other.X + v.Y*other.Y + v.Z*other.Z
}

func (v Vec3) Length() float64 {
	return stdmath.Sqrt(v.Dot(v))
}

func (v Vec3) Normalize() Vec3 {
	length := v.Length()
	if length <= Epsilon {
		return Vec3{}
	}
	return v.Scale(1 / length)
}

func (v Vec3) Distance(other Vec3) float64 {
	return v.Sub(other).Length()
}

func Direction(from, to Position) Vec3 {
	return to.Sub(from).Normalize()
}

func RadToDeg(rad float64) float64 {
	return rad * 180 / stdmath.Pi
}

type Segment struct {
	A Position
	B Position
}

func NewSegment(a, b Position) Segment {
	return Segment{A: a, B: b}
}

func (s Segment) Bounds() AABB {
	min := V3(stdmath.Min(s.A.X, s.B.X), stdmath.Min(s.A.Y, s.B.Y), stdmath.Min(s.A.Z, s.B.Z))
	max := V3(stdmath.Max(s.A.X, s.B.X), stdmath.Max(s.A.Y, s.B.Y), stdmath.Max(s.A.Z, s.B.Z))
	return AABB{Min: min, Max: max}
}

func ClosestPointOnSegment(point, a, b Position) Position {
	ab := b.Sub(a)
	denom := ab.Dot(ab)
	if denom <= Epsilon {
		return a
	}
	t := point.Sub(a).Dot(ab) / denom
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return a.Add(ab.Scale(t))
}

type AABB struct {
	Min Position
	Max Position
}

func AABBFromCenterSize(center Position, size Vec3) AABB {
	half := size.Scale(0.5)
	return AABB{Min: center.Sub(half), Max: center.Add(half)}
}

func (a AABB) Expand(amount float64) AABB {
	delta := V3(amount, amount, amount)
	return AABB{Min: a.Min.Sub(delta), Max: a.Max.Add(delta)}
}

func (a AABB) Intersects(other AABB) bool {
	return a.Min.X <= other.Max.X && a.Max.X >= other.Min.X &&
		a.Min.Y <= other.Max.Y && a.Max.Y >= other.Min.Y &&
		a.Min.Z <= other.Max.Z && a.Max.Z >= other.Min.Z
}

func (a AABB) ContainsPoint(point Position) bool {
	return point.X >= a.Min.X && point.X <= a.Max.X &&
		point.Y >= a.Min.Y && point.Y <= a.Max.Y &&
		point.Z >= a.Min.Z && point.Z <= a.Max.Z
}

type Sphere struct {
	Center Position
	Radius float64
}

func NewSphere(center Position, radius float64) Sphere {
	return Sphere{Center: center, Radius: radius}
}

func (s Sphere) IntersectsAABB(box AABB) bool {
	x := stdmath.Max(box.Min.X, stdmath.Min(s.Center.X, box.Max.X))
	y := stdmath.Max(box.Min.Y, stdmath.Min(s.Center.Y, box.Max.Y))
	z := stdmath.Max(box.Min.Z, stdmath.Min(s.Center.Z, box.Max.Z))
	return s.Center.Distance(V3(x, y, z)) <= s.Radius
}

func SegmentIntersectsAABB(segment Segment, box AABB) bool {
	return segment.Bounds().Intersects(box)
}
