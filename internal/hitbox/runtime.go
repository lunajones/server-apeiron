package hitbox

import (
	"errors"
	"time"

	apeironv1 "db-apeiron/gen/apeiron/v1"
	domainentity "server-apeiron/internal/domain/entity"
	"server-apeiron/internal/domain/ids"
	domainmath "server-apeiron/internal/domain/math"
	"server-apeiron/internal/skill"
	"server-apeiron/internal/spatial"
)

type EntityResolver interface {
	Resolve(ids.RuntimeEntityID) (domainentity.Entity, bool)
}

type EntityResolverFunc func(ids.RuntimeEntityID) (domainentity.Entity, bool)

func (f EntityResolverFunc) Resolve(id ids.RuntimeEntityID) (domainentity.Entity, bool) {
	if f == nil {
		return nil, false
	}
	return f(id)
}

type LineOfSight interface {
	Clear(from domainmath.Position, to domainmath.Position) bool
}

type EvaluationContext struct {
	Caster         domainentity.Entity
	Skill          *apeironv1.Skill
	InstanceID     string
	StartedAt      time.Time
	PreviousNow    time.Time
	PreviousOrigin domainmath.Position
	Now            time.Time
	Origin         domainmath.Position
	AimDirection   domainmath.Vec3
	Target         skill.TargetRef
	HasTarget      bool
	Hitboxes       []*apeironv1.SkillHitboxProfile
	Projectile     any
	Area           any
	Spatial        spatial.SpatialIndex
	Resolver       EntityResolver
	LOS            LineOfSight
}

type HitResult struct {
	SkillID                ids.SkillID
	HitboxID               string
	HitboxIndex            int
	TargetID               ids.RuntimeEntityID
	ImpactPoint            domainmath.Position
	DamageGroupID          string
	MotionProfileID        string
	MotionTStart           float64
	MotionTEnd             float64
	MotionSampleStartIndex int32
	MotionSampleEndIndex   int32
	HitQuality             string
	HitQualitySpatialScore float64
	HitboxDebugShape       string
	HitboxDebugCenter      domainmath.Position
	HitboxDebugExtent      domainmath.Vec3
	HitboxDebugForward     domainmath.Vec3
	HitboxDebugRight       domainmath.Vec3
	HitboxDebugUp          domainmath.Vec3
	HitboxDebugSegmentA    domainmath.Position
	HitboxDebugSegmentB    domainmath.Position
	HitboxDebugSize        domainmath.Vec3
	HitboxDebugRadius      float64
	HitboxDebugLength      float64
	HitboxDebugHeight      float64
	HitboxDebugMinAngleDeg float64
	HitboxDebugMaxAngleDeg float64
}

type Runtime struct{}

func NewRuntime() *Runtime {
	return &Runtime{}
}

func (r *Runtime) Evaluate(ctx EvaluationContext) ([]HitResult, error) {
	if ctx.Caster == nil {
		return nil, errors.New("hitbox evaluation requires caster")
	}
	if ctx.Spatial == nil {
		return nil, errors.New("hitbox evaluation requires spatial index")
	}
	basis := NewBasis(ctx.Origin, ctx.AimDirection)
	previousBasis := basis
	if !ctx.PreviousNow.IsZero() {
		previousBasis = NewBasis(ctx.PreviousOrigin, ctx.AimDirection)
	}
	elapsed := ctx.Now.Sub(ctx.StartedAt)
	previousElapsed := elapsed
	if !ctx.PreviousNow.IsZero() {
		previousElapsed = ctx.PreviousNow.Sub(ctx.StartedAt)
	}

	hits := make([]HitResult, 0)
	for index, profile := range ctx.Hitboxes {
		shape, metadata, ok := activeShape(profile, basis, previousBasis, elapsed, previousElapsed)
		if !ok || shape == nil {
			continue
		}
		objects := ctx.Spatial.QueryAABB(spatial.AABBQuery{
			Bounds: shape.Bounds(),
			Filter: spatial.QueryFilter{
				Exclude: map[ids.RuntimeEntityID]struct{}{ctx.Caster.RuntimeID(): {}},
			},
		})
		for _, object := range objects {
			if !shape.Contains(object) {
				continue
			}
			impact := shape.ImpactPoint(object)
			if ctx.LOS != nil && !ctx.LOS.Clear(ctx.Origin, impact) {
				continue
			}
			debug := ActiveHitboxDebug{}
			fillDebugShape(&debug, shape, basis)
			hits = append(hits, HitResult{
				SkillID:                ids.SkillID(ctx.Skill.GetId()),
				HitboxID:               profile.GetId(),
				HitboxIndex:            index,
				TargetID:               object.EntityID,
				ImpactPoint:            impact,
				DamageGroupID:          metadata.DamageGroupID,
				MotionProfileID:        metadata.MotionProfileID,
				MotionTStart:           metadata.TStart,
				MotionTEnd:             metadata.TEnd,
				MotionSampleStartIndex: metadata.MotionSampleStartIndex,
				MotionSampleEndIndex:   metadata.MotionSampleEndIndex,
				HitQuality:             "clean",
				HitQualitySpatialScore: 1,
				HitboxDebugShape:       debug.HitboxDebugShape,
				HitboxDebugCenter:      debug.HitboxDebugCenter,
				HitboxDebugExtent:      debug.HitboxDebugExtent,
				HitboxDebugForward:     debug.HitboxDebugForward,
				HitboxDebugRight:       debug.HitboxDebugRight,
				HitboxDebugUp:          debug.HitboxDebugUp,
				HitboxDebugSegmentA:    debug.HitboxDebugSegmentA,
				HitboxDebugSegmentB:    debug.HitboxDebugSegmentB,
				HitboxDebugSize:        debug.HitboxDebugSize,
				HitboxDebugRadius:      debug.HitboxDebugRadius,
				HitboxDebugLength:      debug.HitboxDebugLength,
				HitboxDebugHeight:      debug.HitboxDebugHeight,
				HitboxDebugMinAngleDeg: debug.HitboxDebugMinAngleDeg,
				HitboxDebugMaxAngleDeg: debug.HitboxDebugMaxAngleDeg,
			})
		}
	}
	return hits, nil
}

type ActiveHitboxDebug struct {
	SkillID                ids.SkillID
	ActionInstanceID       string
	HitboxID               string
	HitboxIndex            int
	Shape                  ShapeType
	MotionProfileID        string
	DamageGroupID          string
	MotionTStart           float64
	MotionTEnd             float64
	MotionSampleStartIndex int32
	MotionSampleEndIndex   int32
	HitboxDebugShape       string
	HitboxDebugCenter      domainmath.Position
	HitboxDebugExtent      domainmath.Vec3
	HitboxDebugForward     domainmath.Vec3
	HitboxDebugRight       domainmath.Vec3
	HitboxDebugUp          domainmath.Vec3
	HitboxDebugSegmentA    domainmath.Position
	HitboxDebugSegmentB    domainmath.Position
	HitboxDebugSize        domainmath.Vec3
	HitboxDebugRadius      float64
	HitboxDebugLength      float64
	HitboxDebugHeight      float64
	HitboxDebugMinAngleDeg float64
	HitboxDebugMaxAngleDeg float64
}

func ActiveHitboxDebugs(ctx EvaluationContext) ([]ActiveHitboxDebug, error) {
	basis := NewBasis(ctx.Origin, ctx.AimDirection)
	previousBasis := basis
	if !ctx.PreviousNow.IsZero() {
		previousBasis = NewBasis(ctx.PreviousOrigin, ctx.AimDirection)
	}
	elapsed := ctx.Now.Sub(ctx.StartedAt)
	previousElapsed := elapsed
	if !ctx.PreviousNow.IsZero() {
		previousElapsed = ctx.PreviousNow.Sub(ctx.StartedAt)
	}
	out := make([]ActiveHitboxDebug, 0)
	for index, profile := range ctx.Hitboxes {
		shape, metadata, ok := activeShape(profile, basis, previousBasis, elapsed, previousElapsed)
		if !ok || shape == nil {
			continue
		}
		debug := ActiveHitboxDebug{
			SkillID:                ids.SkillID(ctx.Skill.GetId()),
			ActionInstanceID:       ctx.InstanceID,
			HitboxID:               profile.GetId(),
			HitboxIndex:            index,
			MotionProfileID:        metadata.MotionProfileID,
			DamageGroupID:          metadata.DamageGroupID,
			MotionTStart:           metadata.TStart,
			MotionTEnd:             metadata.TEnd,
			MotionSampleStartIndex: metadata.MotionSampleStartIndex,
			MotionSampleEndIndex:   metadata.MotionSampleEndIndex,
		}
		fillDebugShape(&debug, shape, basis)
		out = append(out, debug)
	}
	return out, nil
}

func activeShape(profile *apeironv1.SkillHitboxProfile, basis Basis, previousBasis Basis, elapsed time.Duration, previousElapsed time.Duration) (RuntimeShape, MotionHitboxShape, bool) {
	if profile == nil {
		return nil, MotionHitboxShape{}, false
	}
	start := ms(profile.GetHitboxStartMs())
	end := ms(profile.GetHitboxEndMs())
	if elapsed < start || elapsed > end {
		return nil, MotionHitboxShape{}, false
	}
	if motionShape, ok := ShapeFromMotionProfile(profile, basis, previousBasis, elapsed, previousElapsed); ok {
		return motionShape.Shape, motionShape, true
	}
	shape := staticShapeFromProfile(profile, basis)
	return shape, MotionHitboxShape{Shape: shape}, shape != nil
}

func staticShapeFromProfile(profile *apeironv1.SkillHitboxProfile, basis Basis) RuntimeShape {
	switch profile.GetHitboxShape() {
	case "arc", "asymmetric_arc":
		return ArcSliceShape{
			Center:       basis.Offset(profile.GetOffsetX(), profile.GetOffsetY(), profile.GetOffsetZ()),
			Length:       firstPositive(profile.GetLength(), profile.GetSizeX(), profile.GetRadius()),
			MinDeg:       -profile.GetSizeY(),
			MaxDeg:       profile.GetSizeX(),
			Height:       firstPositive(profile.GetSizeZ(), profile.GetRadius()*2, 180),
			QueryPadding: firstPositive(profile.GetRadius(), 0),
			Basis:        basis,
		}
	case "box", "rectangle":
		return BoxStripShape{
			Center: basis.Offset(profile.GetOffsetX(), profile.GetOffsetY(), profile.GetOffsetZ()),
			Size: domainmath.V3(
				firstPositive(profile.GetSizeX(), profile.GetLength(), 1),
				firstPositive(profile.GetSizeY(), profile.GetRadius()*2, 1),
				firstPositive(profile.GetSizeZ(), profile.GetRadius()*2, 180),
			),
			Basis: basis,
		}
	case "capsule", "capsule_strip":
		start := basis.Offset(profile.GetOffsetX(), profile.GetOffsetY(), profile.GetOffsetZ())
		end := start.Add(basis.Forward.Mul(firstPositive(profile.GetLength(), profile.GetSizeX(), 1)))
		return CapsuleStripShape{
			Segment: domainmath.NewSegment(start, end),
			Radius:  firstPositive(profile.GetRadius(), profile.GetSizeY()*0.5, 1),
		}
	default:
		return nil
	}
}

func fillDebugShape(debug *ActiveHitboxDebug, shape RuntimeShape, basis Basis) {
	switch s := shape.(type) {
	case ArcSliceShape:
		debug.Shape = ShapeArcSlice
		debug.HitboxDebugShape = string(ShapeArcSlice)
		debug.HitboxDebugCenter = s.Center
		debug.HitboxDebugForward = s.Basis.Forward
		debug.HitboxDebugRight = s.Basis.Right
		debug.HitboxDebugUp = s.Basis.Up
		debug.HitboxDebugLength = s.Length
		debug.HitboxDebugHeight = s.Height
		debug.HitboxDebugMinAngleDeg = s.MinDeg
		debug.HitboxDebugMaxAngleDeg = s.MaxDeg
	case BoxStripShape:
		debug.Shape = ShapeBoxStrip
		debug.HitboxDebugShape = string(ShapeBoxStrip)
		debug.HitboxDebugCenter = s.Center
		debug.HitboxDebugSize = s.Size
		debug.HitboxDebugExtent = s.Size.Mul(0.5)
		debug.HitboxDebugForward = s.Basis.Forward
		debug.HitboxDebugRight = s.Basis.Right
		debug.HitboxDebugUp = s.Basis.Up
	case CapsuleStripShape:
		debug.Shape = ShapeCapsuleStrip
		debug.HitboxDebugShape = string(ShapeCapsuleStrip)
		debug.HitboxDebugSegmentA = s.Segment.A
		debug.HitboxDebugSegmentB = s.Segment.B
		debug.HitboxDebugRadius = s.Radius
		debug.HitboxDebugLength = s.Segment.A.Distance(s.Segment.B)
		debug.HitboxDebugForward = basis.Forward
		debug.HitboxDebugRight = basis.Right
		debug.HitboxDebugUp = basis.Up
	}
}
