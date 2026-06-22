package combat

import (
	domainentity "server-apeiron/internal/domain/entity"
	"server-apeiron/internal/hitbox"
)

func appendHitboxDebugEvents(out []HitboxDebugEvent, source domainentity.Entity, debug []hitbox.ActiveHitboxDebug, tick uint64) []HitboxDebugEvent {
	if source == nil || len(debug) == 0 {
		return out
	}
	for _, item := range debug {
		if item.MotionProfileID == "" || item.HitboxDebugShape == "" {
			continue
		}
		out = append(out, HitboxDebugEvent{
			Source:                 source.Ref(),
			RegionID:               source.RegionID(),
			SkillID:                item.SkillID,
			ActionInstanceID:       item.ActionInstanceID,
			HitboxID:               item.HitboxID,
			HitboxIndex:            item.HitboxIndex,
			Shape:                  string(item.Shape),
			MotionProfileID:        item.MotionProfileID,
			DamageGroupID:          item.DamageGroupID,
			MotionTStart:           item.MotionTStart,
			MotionTEnd:             item.MotionTEnd,
			MotionSampleStartIndex: item.MotionSampleStartIndex,
			MotionSampleEndIndex:   item.MotionSampleEndIndex,
			HitboxDebugShape:       item.HitboxDebugShape,
			HitboxDebugCenter:      item.HitboxDebugCenter,
			HitboxDebugExtent:      item.HitboxDebugExtent,
			HitboxDebugForward:     item.HitboxDebugForward,
			HitboxDebugRight:       item.HitboxDebugRight,
			HitboxDebugUp:          item.HitboxDebugUp,
			HitboxDebugSegmentA:    item.HitboxDebugSegmentA,
			HitboxDebugSegmentB:    item.HitboxDebugSegmentB,
			HitboxDebugSize:        item.HitboxDebugSize,
			HitboxDebugRadius:      item.HitboxDebugRadius,
			HitboxDebugLength:      item.HitboxDebugLength,
			HitboxDebugHeight:      item.HitboxDebugHeight,
			HitboxDebugMinAngleDeg: item.HitboxDebugMinAngleDeg,
			HitboxDebugMaxAngleDeg: item.HitboxDebugMaxAngleDeg,
			Tick:                   tick,
		})
	}
	return out
}
