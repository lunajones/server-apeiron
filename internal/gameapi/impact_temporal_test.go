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

func TestPlayerShieldRushUsesShortShieldFaceBeforeCarry(t *testing.T) {
	t.Parallel()

	runtime := NewRuntimeWithContracts(DevFixtureRuntimeContracts())
	skill := runtime.contracts.skillContract("player_shield_rush")
	start := vector{}
	dir := vector{x: 1}
	end := vector{x: skillRangeToCM(skill.Range)}
	closeTarget := vector{x: 36}
	futureTarget := vector{x: 120}

	if _, ok := skillRuntimeHitboxContainsAt(skill, start, end, dir, futureTarget, 160); ok {
		t.Fatal("shield rush hit a future target through an invisible long front wall at hitbox start")
	}
	if _, ok := skillRuntimeHitboxContainsAt(skill, start, end, dir, closeTarget, 160); !ok {
		t.Fatal("shield rush missed the close shield-face contact at hitbox start")
	}
	if _, ok := skillRuntimeHitboxContainsAt(skill, start, end, dir, vector{x: 60}, 880); !ok {
		t.Fatal("shield rush missed a target reached by the temporal shield face at the end of the hitbox")
	}
}

func TestPlayerShieldBashIsNarrowTemporalContact(t *testing.T) {
	t.Parallel()

	runtime := NewRuntimeWithContracts(DevFixtureRuntimeContracts())
	skill := runtime.contracts.skillContract("player_shield_bash")
	start := vector{}
	dir := vector{x: 1}
	end := vector{x: skillRangeToCM(skill.Range)}

	if _, ok := skillRuntimeHitboxContainsAt(skill, start, end, dir, vector{x: 100, y: 80}, 110); ok {
		t.Fatal("shield bash hit outside the narrowed frontal contact width")
	}
	if _, ok := skillRuntimeHitboxContainsAt(skill, start, end, dir, vector{x: 100, y: 60}, 110); !ok {
		t.Fatal("shield bash missed a target inside the narrowed frontal contact width")
	}
	if _, ok := skillRuntimeHitboxContainsAt(skill, start, end, dir, vector{x: 240}, 110); ok {
		t.Fatal("shield bash hit a target before the temporal contact reached it")
	}
	if _, ok := skillRuntimeHitboxContainsAt(skill, start, end, dir, vector{x: 220}, 280); !ok {
		t.Fatal("shield bash missed a target reached by the late temporal contact")
	}
}

func TestPlayerShieldDriveGrowsLengthWithoutGrowingWidth(t *testing.T) {
	t.Parallel()

	runtime := NewRuntimeWithContracts(DevFixtureRuntimeContracts())
	skill := runtime.contracts.skillContract("player_basic_attack_3")
	start := vector{}
	dir := vector{x: 1}
	end := vector{x: skillRangeToCM(skill.Range)}

	if _, ok := skillRuntimeHitboxContainsAt(skill, start, end, dir, vector{x: 110}, 180); ok {
		t.Fatal("shield drive initial contact length is too large")
	}
	if _, ok := skillRuntimeHitboxContainsAt(skill, start, end, dir, vector{x: 230}, 440); !ok {
		t.Fatal("shield drive final contact length did not reach the intended forward target")
	}
	if _, ok := skillRuntimeHitboxContainsAt(skill, start, end, dir, vector{x: 130, y: 55}, 440); ok {
		t.Fatal("shield drive widened laterally instead of only growing forward length")
	}
}
