package hitbox

import (
	"testing"
	"time"

	apeironv1 "db-apeiron/gen/apeiron/v1"
	domainmath "server-apeiron/internal/domain/math"
)

func TestShapeFromMotionProfileBuildsTemporalCapsuleSweep(t *testing.T) {
	profile := &apeironv1.SkillHitboxProfile{
		Id:            "hitbox_player_basic_attack_1_0",
		SkillId:       "player_basic_attack_1",
		HitboxShape:   "temporal_sweep",
		HitboxStartMs: 100,
		HitboxEndMs:   300,
		Radius:        45,
		SizeZ:         150,
		MotionProfile: &apeironv1.SkillHitboxMotionProfile{
			Id:            "motion_player_basic_attack_1_forward_v1",
			Enabled:       true,
			MotionType:    "timeline_sweep",
			TimeBasis:     "hitbox_window_normalized",
			Interpolation: "linear",
			SweepShape:    "capsule_strip",
			DamageGroupId: "player_basic_attack_1_damage",
			Samples: []*apeironv1.SkillHitboxMotionSample{
				{SampleIndex: 0, T: 0.0, OffsetX: 35, Length: 70, Radius: 48},
				{SampleIndex: 1, T: 0.5, OffsetX: 85, Length: 150, Radius: 50},
				{SampleIndex: 2, T: 1.0, OffsetX: 130, Length: 210, Radius: 50},
			},
		},
	}

	basis := NewBasis(domainmath.V3(0, 0, 0), domainmath.V3(1, 0, 0))
	shape, ok := ShapeFromMotionProfile(profile, basis, basis, 200*time.Millisecond, 100*time.Millisecond)
	if !ok {
		t.Fatal("expected temporal sweep shape")
	}
	capsule, ok := shape.Shape.(CapsuleStripShape)
	if !ok {
		t.Fatalf("shape type = %T, want CapsuleStripShape", shape.Shape)
	}
	if shape.MotionProfileID != "motion_player_basic_attack_1_forward_v1" {
		t.Fatalf("motion profile id = %q", shape.MotionProfileID)
	}
	if shape.DamageGroupID != "player_basic_attack_1_damage" {
		t.Fatalf("damage group id = %q", shape.DamageGroupID)
	}
	if shape.TStart != 0 || shape.TEnd != 0.5 {
		t.Fatalf("motion t range = %.2f..%.2f, want 0..0.5", shape.TStart, shape.TEnd)
	}
	if capsule.Segment.B.X <= capsule.Segment.A.X {
		t.Fatalf("capsule did not advance forward: A=%v B=%v", capsule.Segment.A, capsule.Segment.B)
	}
	if capsule.Radius != 50 {
		t.Fatalf("capsule radius = %.1f, want 50", capsule.Radius)
	}
}
