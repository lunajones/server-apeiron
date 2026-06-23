package gameapi

import (
	"testing"

	dbv1 "db-apeiron/gen/apeiron/v1"
)

func TestTemporalCapsuleImpactDoesNotHitFutureMotionSample(t *testing.T) {
	t.Parallel()

	skill := SkillRuntimeContract{
		SkillID: "test_temporal_shield_punch",
		Range:   300,
		Hitboxes: []*dbv1.SkillHitboxProfile{{
			Id:            "hitbox_test_temporal_shield_punch",
			HitboxShape:   "temporal_sweep",
			HitboxStartMs: 100,
			HitboxEndMs:   300,
			Length:        40,
			Radius:        20,
			MotionProfile: &dbv1.SkillHitboxMotionProfile{
				Id:            "motion_test_temporal_shield_punch",
				Enabled:       true,
				MotionType:    "timeline_sweep",
				TimeBasis:     "hitbox_window_normalized",
				Interpolation: "linear",
				SweepShape:    "capsule_strip",
				Samples: []*dbv1.SkillHitboxMotionSample{
					{SampleIndex: 0, T: 0, OffsetX: 20, Length: 40, Radius: 20},
					{SampleIndex: 1, T: 1, OffsetX: 160, Length: 40, Radius: 20},
				},
			},
		}},
	}

	start := vector{}
	end := vector{x: 220}
	dir := vector{x: 1}
	futureTarget := vector{x: 180}

	if _, ok := skillRuntimeHitboxContainsAt(skill, start, end, dir, futureTarget, 100); ok {
		t.Fatal("temporal impact hit future motion sample at the start of the hitbox window")
	}
	if _, ok := skillRuntimeHitboxContainsAt(skill, start, end, dir, futureTarget, 300); !ok {
		t.Fatal("temporal impact missed target when the moving capsule reached its sample")
	}
}

func TestTemporalArcImpactUsesCurrentAngleSlice(t *testing.T) {
	t.Parallel()

	skill := SkillRuntimeContract{
		SkillID: "test_temporal_right_to_left_slash",
		Range:   200,
		Hitboxes: []*dbv1.SkillHitboxProfile{{
			Id:            "hitbox_test_temporal_right_to_left_slash",
			HitboxShape:   "temporal_sweep",
			HitboxStartMs: 50,
			HitboxEndMs:   250,
			Length:        140,
			Angle:         90,
			MotionProfile: &dbv1.SkillHitboxMotionProfile{
				Id:            "motion_test_temporal_right_to_left_slash",
				Enabled:       true,
				MotionType:    "timeline_sweep",
				TimeBasis:     "hitbox_window_normalized",
				Interpolation: "step",
				SweepShape:    "arc_slice",
				Samples: []*dbv1.SkillHitboxMotionSample{
					{SampleIndex: 0, T: 0, Length: 140, StartAngleDeg: -45, EndAngleDeg: -5},
					{SampleIndex: 1, T: 1, Length: 140, StartAngleDeg: 5, EndAngleDeg: 45},
				},
			},
		}},
	}

	start := vector{}
	end := vector{x: 140}
	dir := vector{x: 1}
	lateArcTarget := vector{x: 90, y: 70}

	if _, ok := skillRuntimeHitboxContainsAt(skill, start, end, dir, lateArcTarget, 50); ok {
		t.Fatal("temporal arc hit a target from the late angle slice at the first active sample")
	}
	if _, ok := skillRuntimeHitboxContainsAt(skill, start, end, dir, lateArcTarget, 250); !ok {
		t.Fatal("temporal arc missed a target from the active late angle slice")
	}
}
