package ai

import (
	"testing"

	domainmath "server-apeiron/internal/domain/math"
)

func TestBrainSelectsLungeFromBindingInCircleRange(t *testing.T) {
	brain := NewBrain(testPolicy())
	decision := brain.Decide(Input{
		Tick:             10,
		CreaturePosition: domainmath.V3(0, 0, 0),
		TargetPosition:   domainmath.V3(520, 0, 0),
		LineOfSight:      true,
	})
	if decision.SelectedSkill != "lunge" {
		t.Fatalf("selected skill = %q, want lunge", decision.SelectedSkill)
	}
	if decision.Action != "lunge" {
		t.Fatalf("action = %q, want lunge", decision.Action)
	}
	if decision.SetupPolicyID != "wolf_lunge_flank_windup_v1" {
		t.Fatalf("setup policy = %q, want flank windup", decision.SetupPolicyID)
	}
	if decision.MovementTactic != "circle_then_curve_to_target" || decision.Commitment != "preparing" {
		t.Fatalf("lunge setup decision = %#v", decision)
	}
	if decision.Direction.Y <= 0 || decision.Direction.X <= 0 {
		t.Fatalf("lunge setup should curve laterally toward target, got direction %#v", decision.Direction)
	}
}

func TestBrainKeepsLungeSetupMovementDuringWindup(t *testing.T) {
	brain := NewBrain(testPolicy())
	first := brain.Decide(Input{
		Tick:             10,
		CreaturePosition: domainmath.V3(0, 0, 0),
		TargetPosition:   domainmath.V3(520, 0, 0),
		LineOfSight:      true,
	})
	if first.SetupPolicyID == "" {
		t.Fatalf("first decision did not start setup: %#v", first)
	}

	active := brain.Decide(Input{
		Tick:                    12,
		CreaturePosition:        domainmath.V3(10, 15, 0),
		TargetPosition:          domainmath.V3(520, 0, 0),
		ActiveSkillID:           "lunge",
		ActiveSkillElapsedTicks: 20,
		LineOfSight:             true,
	})
	if active.DecisionPhase != "setup" || active.SetupPolicyID != "wolf_lunge_flank_windup_v1" {
		t.Fatalf("active setup decision = %#v", active)
	}
	if active.Direction.Y <= 0 {
		t.Fatalf("active lunge setup should keep the locked orbit side, got %#v", active.Direction)
	}
}

func TestBrainUsesRetreatEvasionUnderPressure(t *testing.T) {
	brain := NewBrain(testPolicy())
	decision := brain.Decide(Input{
		Tick:             10,
		CreaturePosition: domainmath.V3(0, 0, 0),
		TargetPosition:   domainmath.V3(120, 0, 0),
		LineOfSight:      true,
		Pressure:         0.2,
	})
	if decision.SelectedSkill != "wolf_dodge" {
		t.Fatalf("selected skill = %q, want wolf_dodge", decision.SelectedSkill)
	}
	if decision.Action != "retreat" {
		t.Fatalf("action = %q, want retreat", decision.Action)
	}
	if decision.Direction.X >= 0 {
		t.Fatalf("retreat should move away from target, got direction %#v", decision.Direction)
	}
}

func TestBrainUsesMaulCounterWhenPressureHigh(t *testing.T) {
	brain := NewBrain(testPolicy())
	decision := brain.Decide(Input{
		Tick:             10,
		CreaturePosition: domainmath.V3(0, 0, 0),
		TargetPosition:   domainmath.V3(150, 0, 0),
		LineOfSight:      true,
		Pressure:         0.9,
	})
	if decision.SelectedSkill != "maul" {
		t.Fatalf("selected skill = %q, want maul", decision.SelectedSkill)
	}
	if decision.Action != "maul" {
		t.Fatalf("action = %q, want maul", decision.Action)
	}
}

func TestThreatAssessmentPromotesCommittedTargetCounter(t *testing.T) {
	policy := testPolicy()
	threat := AssessThreat(policy, Input{
		Pressure: 0.50,
		Perception: Perception{
			TargetActionActive: true,
			TargetCombatState:  "committed",
		},
	}, domainmath.V3(1, 0, 0), 180)

	if threat.Pressure < policy.MaulPressureThreshold {
		t.Fatalf("pressure = %.2f, want >= %.2f", threat.Pressure, policy.MaulPressureThreshold)
	}
	if threat.PreferredCounter != "maul" {
		t.Fatalf("preferred counter = %q, want maul", threat.PreferredCounter)
	}
	if got := skillThreatMultiplier(threat, "maul"); got <= 1 {
		t.Fatalf("maul multiplier = %.2f, want > 1", got)
	}
	if !threat.TargetCommitted {
		t.Fatalf("threat did not mark target committed: %#v", threat)
	}
}

func TestBrainThreatCanPromoteCounterBindingWithoutWolfBranch(t *testing.T) {
	policy := testPolicy()
	policy.Bindings = append(policy.Bindings,
		SkillBinding{ID: "bind_maul_circle_read", SkillID: "maul", TacticalState: "circle", DecisionPhase: "reposition", SetupPolicyID: "wolf_maul_pressure_counter_v1", MinRangeCM: 260, MaxRangeCM: 760, Priority: 84, UsageWeight: 1, Enabled: true, RequiresLineOfSight: true},
	)
	brain := NewBrain(policy)
	decision := brain.Decide(Input{
		Tick:             10,
		CreaturePosition: domainmath.V3(0, 0, 0),
		TargetPosition:   domainmath.V3(520, 0, 0),
		LineOfSight:      true,
		Pressure:         0.38,
		Perception: Perception{
			TargetActionActive: true,
			TargetCombatState:  "committed",
		},
	})

	if decision.SelectedSkill != "maul" {
		t.Fatalf("threat multiplier should promote maul counter, got %#v", decision)
	}
	if decision.Threat.PreferredCounter != "maul" {
		t.Fatalf("decision threat = %#v, want maul counter", decision.Threat)
	}
	if decision.SetupPolicyID != "wolf_maul_pressure_counter_v1" {
		t.Fatalf("setup policy = %q, want maul counter setup", decision.SetupPolicyID)
	}
}

func TestBrainKeepsActiveSkillUntilRuntimeEnds(t *testing.T) {
	brain := NewBrain(testPolicy())
	decision := brain.Decide(Input{
		Tick:                    30,
		CreaturePosition:        domainmath.V3(0, 0, 0),
		TargetPosition:          domainmath.V3(400, 0, 0),
		ActiveSkillID:           "lunge",
		ActiveSkillElapsedTicks: 150,
		LineOfSight:             true,
	})
	if decision.SelectedSkill != "lunge" || decision.DecisionPhase != "active" {
		t.Fatalf("active skill decision = %#v", decision)
	}
}

func TestBrainSkipsUnavailableSkillBinding(t *testing.T) {
	brain := NewBrain(testPolicy())
	decision := brain.Decide(Input{
		Tick:             10,
		CreaturePosition: domainmath.V3(0, 0, 0),
		TargetPosition:   domainmath.V3(520, 0, 0),
		LineOfSight:      true,
		UnavailableSkill: map[string]string{"lunge": "cooldown"},
	})
	if decision.SelectedSkill == "lunge" {
		t.Fatalf("selected unavailable skill: %#v", decision)
	}
	if decision.Action != "orbit" {
		t.Fatalf("action = %q, want orbit while lunge cools down", decision.Action)
	}
}

func TestBrainSkipsSkillWhenResourceBudgetCannotPayCost(t *testing.T) {
	brain := NewBrain(testPolicy())
	decision := brain.Decide(Input{
		Tick:             10,
		CreaturePosition: domainmath.V3(0, 0, 0),
		TargetPosition:   domainmath.V3(520, 0, 0),
		LineOfSight:      true,
		ResourceCurrent:  8,
		ResourceMax:      100,
		SkillCosts:       map[string]float64{"lunge": 24},
	})
	if decision.SelectedSkill == "lunge" {
		t.Fatalf("selected unaffordable lunge: %#v", decision)
	}
	if decision.ResourceState != "available" {
		t.Fatalf("resource state = %q, want available", decision.ResourceState)
	}
}

func TestBrainRepeatPenaltyLetsAlternativeBindingWin(t *testing.T) {
	policy := testPolicy()
	policy.RepeatSkillPenaltyWindowTicks = 100
	policy.RepeatSkillPenaltyMultiplier = 0.1
	policy.Bindings = append(policy.Bindings,
		SkillBinding{ID: "bind_bite_circle", SkillID: "bite", TacticalState: "circle", DecisionPhase: "reposition", MinRangeCM: 260, MaxRangeCM: 760, Priority: 70, UsageWeight: 1, Enabled: true, RequiresLineOfSight: true},
	)
	brain := NewBrain(policy)
	first := brain.Decide(Input{
		Tick:             10,
		CreaturePosition: domainmath.V3(0, 0, 0),
		TargetPosition:   domainmath.V3(520, 0, 0),
		LineOfSight:      true,
		ResourceCurrent:  100,
		ResourceMax:      100,
	})
	if first.SelectedSkill != "lunge" {
		t.Fatalf("first selected skill = %q, want lunge", first.SelectedSkill)
	}
	second := brain.Decide(Input{
		Tick:             20,
		CreaturePosition: domainmath.V3(0, 0, 0),
		TargetPosition:   domainmath.V3(520, 0, 0),
		LineOfSight:      true,
		ResourceCurrent:  100,
		ResourceMax:      100,
	})
	if second.SelectedSkill != "bite" {
		t.Fatalf("repeat penalty should choose alternative binding, got %#v", second)
	}
}

func TestOrbitSideSwitchChanceMultiplierIsNotPeriodicAlwaysFlip(t *testing.T) {
	policy := testPolicy()
	policy.AllowSideSwitchWhenTargetFaces = true
	policy.LockSideDuringSetup = false
	policy.MinOrbitDurationTicks = 10
	policy.SideSwitchCooldownTicks = 10
	policy.SideFlipChanceMultiplier = 0.35

	side := "left"
	switches := 0
	windows := 20
	for bucket := 1; bucket <= windows; bucket++ {
		input := Input{Tick: uint64(bucket) * (policy.MinOrbitDurationTicks + policy.SideSwitchCooldownTicks)}
		if shouldSwitchOrbitSide(policy, input, side) {
			switches++
			if side == "left" {
				side = "right"
			} else {
				side = "left"
			}
		}
	}
	if switches == 0 {
		t.Fatal("partial side flip chance never allowed a side switch")
	}
	if switches >= windows {
		t.Fatalf("partial side flip chance behaved like periodic always-flip: switches=%d windows=%d", switches, windows)
	}
}

func TestOrbitSideSwitchFullChanceStillAllowsDeterministicFlip(t *testing.T) {
	policy := testPolicy()
	policy.MinOrbitDurationTicks = 10
	policy.SideSwitchCooldownTicks = 10
	policy.SideFlipChanceMultiplier = 1

	if !shouldSwitchOrbitSide(policy, Input{Tick: 20}, "left") {
		t.Fatal("full side flip chance should allow switch after the policy window")
	}
}

func testPolicy() Policy {
	return Policy{
		DesiredRangeCM:           420,
		ChaseRangeCM:             760,
		RetreatRangeCM:           220,
		OrbitSpeedCMS:            220,
		ChaseSpeedCMS:            420,
		LungeSpeedCMS:            760,
		MaulSpeedCMS:             320,
		RetreatSpeedCMS:          360,
		DodgeSkillID:             "wolf_dodge",
		BiteRangeCM:              220,
		LungeMinRangeCM:          260,
		LungeMaxRangeCM:          760,
		MaulPressureThreshold:    0.65,
		DodgeUnderPressure:       true,
		MaulCounterUnderPressure: true,
		MaulCounterChance:        0.22,
		DodgeRetreatMultiplier:   0.70,
		GlobalDodgeMultiplier:    0.85,
		MinOrbitDurationTicks:    90,
		SideSwitchCooldownTicks:  45,
		SetupPolicies: map[string]SkillSetupPolicy{
			"wolf_lunge_flank_windup_v1":    {ID: "wolf_lunge_flank_windup_v1", SkillID: "lunge", SetupType: "moving_windup", MinSetupTicks: 90, MaxSetupTicks: 126, CommitDistanceCM: 520, PreferredMinRangeCM: 180, PreferredMaxRangeCM: 700, MovementTactic: "circle_then_curve_to_target", LockSideDuringSetup: true, Enabled: true},
			"wolf_maul_pressure_counter_v1": {ID: "wolf_maul_pressure_counter_v1", SkillID: "maul", SetupType: "pressure_counter", MinSetupTicks: 4, MaxSetupTicks: 10, CommitDistanceCM: 160, PreferredMinRangeCM: 0, PreferredMaxRangeCM: 220, MovementTactic: "lateral_counter_dash", LockSideDuringSetup: true, Enabled: true},
		},
		Bindings: []SkillBinding{
			{ID: "bind_lunge_circle", SkillID: "lunge", TacticalState: "circle", DecisionPhase: "reposition", SetupPolicyID: "wolf_lunge_flank_windup_v1", MinRangeCM: 260, MaxRangeCM: 760, Priority: 90, UsageWeight: 1.1, Enabled: true, RequiresLineOfSight: true},
			{ID: "bind_maul_pressure", SkillID: "maul", TacticalState: "pressure", DecisionPhase: "counter", SetupPolicyID: "wolf_maul_pressure_counter_v1", MinRangeCM: 0, MaxRangeCM: 260, Priority: 100, UsageWeight: 0.7, Enabled: true, RequiresLineOfSight: true},
			{ID: "bind_dodge_pressure", SkillID: "wolf_dodge", TacticalState: "pressure", DecisionPhase: "evade", MinRangeCM: 0, MaxRangeCM: 420, Priority: 110, UsageWeight: 1.2, Enabled: true},
		},
	}
}
