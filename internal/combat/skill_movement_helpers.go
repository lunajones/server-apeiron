package combat

import (
	"time"

	apeironv1 "db-apeiron/gen/apeiron/v1"
	"server-apeiron/internal/movement"
)

type skillMovementTimelineContext struct {
	Timing              ActionTimingConfig
	HitboxStart         time.Duration
	HasMovementContract bool
	MovementContract    movement.MovementActionContract
}

func skillMovementConfigForAttackProfile(profile AttackProfile) (skillMovementConfig, bool) {
	for _, hitbox := range profile.Hitboxes {
		if hitbox != nil && hitbox.GetMotionProfile() != nil && hitbox.GetMotionProfile().GetEnabled() {
			return skillMovementConfig{MovementType: "grounded", DurationMS: hitbox.GetHitboxEndMs() - hitbox.GetHitboxStartMs()}, true
		}
	}
	return skillMovementConfig{}, false
}

func skillMovementConfigForAttackProfileOrContract(profile AttackProfile, contract movement.MovementActionContract, hasContract bool) (skillMovementConfig, bool) {
	if hasContract {
		duration := contract.ActiveMS
		if duration <= 0 {
			duration = contract.DurationMS
		}
		return skillMovementConfig{
			MovementType:          contract.MovementAction,
			Distance:              contract.HorizontalDistanceCM,
			Speed:                 contract.BaseSpeedCMPerSec,
			DurationMS:            duration,
			MaxTurnDegPerSec:      contract.MaxTurnDegPerSec,
			MaxTotalRedirectAngle: contract.MaxRedirectDeg,
			MovementStartPhase:    contract.Phase,
		}, true
	}
	return skillMovementConfigForAttackProfile(profile)
}

func hitboxActiveWindow(hitboxes []*apeironv1.SkillHitboxProfile) (time.Duration, time.Duration) {
	if len(hitboxes) == 0 {
		return 0, 0
	}
	start := time.Duration(hitboxes[0].GetHitboxStartMs()) * time.Millisecond
	end := time.Duration(hitboxes[0].GetHitboxEndMs()) * time.Millisecond
	for _, hitbox := range hitboxes[1:] {
		if hitbox == nil {
			continue
		}
		nextStart := time.Duration(hitbox.GetHitboxStartMs()) * time.Millisecond
		nextEnd := time.Duration(hitbox.GetHitboxEndMs()) * time.Millisecond
		if nextStart < start {
			start = nextStart
		}
		if nextEnd > end {
			end = nextEnd
		}
	}
	return start, end
}

func skillMovementStart(ctx skillMovementTimelineContext, cfg skillMovementConfig) time.Duration {
	if ctx.HasMovementContract && ctx.MovementContract.StartupMS > 0 {
		return time.Duration(ctx.MovementContract.StartupMS) * time.Millisecond
	}
	if cfg.MovementStartOffsetMS > 0 {
		return time.Duration(cfg.MovementStartOffsetMS) * time.Millisecond
	}
	if cfg.MovementStartPhase == "active" && ctx.HitboxStart > 0 {
		return ctx.HitboxStart
	}
	return ctx.Timing.Windup
}

func skillMovementIntentCurveSamples(cfg skillMovementConfig) []movement.MovementActionCurvePoint {
	return nil
}
