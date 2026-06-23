package gameapi

import "testing"

func TestFixtureCreatureSkillsCarryTemporalHitboxesAndDamage(t *testing.T) {
	t.Parallel()

	contracts := DevFixtureRuntimeContracts()
	cases := map[string]struct {
		damage        float64
		posture       float64
		motionProfile string
	}{
		"bite":  {damage: 9, posture: 17.6, motionProfile: "motion_wolf_bite_melee_v1"},
		"lunge": {damage: 6.5, posture: 19.2, motionProfile: "motion_wolf_lunge_cross_v1"},
		"maul":  {damage: 6.5, posture: 19.2, motionProfile: "motion_wolf_maul_lateral_counter_v1"},
	}

	for skillID, want := range cases {
		contract := contracts.skillContract(skillID)
		if contract.SkillID != skillID {
			t.Fatalf("missing creature skill contract %q (got %q)", skillID, contract.SkillID)
		}
		if contract.Damage != want.damage || contract.PostureDamage != want.posture {
			t.Fatalf("%s damage/posture = %.1f/%.1f, want %.1f/%.1f", skillID, contract.Damage, contract.PostureDamage, want.damage, want.posture)
		}
		if len(contract.Hitboxes) != 1 {
			t.Fatalf("%s hitboxes = %d, want 1", skillID, len(contract.Hitboxes))
		}
		motion := contract.Hitboxes[0].GetMotionProfile()
		if motion == nil || !motion.GetEnabled() {
			t.Fatalf("%s motion profile missing or disabled: %#v", skillID, motion)
		}
		if motion.GetId() != want.motionProfile {
			t.Fatalf("%s motion profile = %q, want %q", skillID, motion.GetId(), want.motionProfile)
		}
		if len(motion.GetSamples()) < 3 {
			t.Fatalf("%s motion samples = %d, want at least 3", skillID, len(motion.GetSamples()))
		}
	}
}

func TestFixtureCreatureTemporalLungeDoesNotHitFutureSampleEarly(t *testing.T) {
	t.Parallel()

	contract := DevFixtureRuntimeContracts().skillContract("lunge")
	start := vector{}
	end := vector{x: 700}
	dir := vector{x: 1}
	futureTarget := vector{x: 520}

	if _, ok := skillRuntimeHitboxContainsAt(contract, start, end, dir, futureTarget, 3600); ok {
		t.Fatal("wolf lunge hit a future motion sample at the start of the hitbox window")
	}
	if _, ok := skillRuntimeHitboxContainsAt(contract, start, end, dir, futureTarget, 3980); !ok {
		t.Fatal("wolf lunge missed when the temporal capsule reached the target")
	}
}
