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
	if decision.Direction.X <= 0 {
		t.Fatalf("lunge should move toward target, got direction %#v", decision.Direction)
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

func TestBrainKeepsActiveSkillUntilRuntimeEnds(t *testing.T) {
	brain := NewBrain(testPolicy())
	decision := brain.Decide(Input{
		Tick:             30,
		CreaturePosition: domainmath.V3(0, 0, 0),
		TargetPosition:   domainmath.V3(400, 0, 0),
		ActiveSkillID:    "lunge",
		LineOfSight:      true,
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

func testPolicy() Policy {
	return Policy{
		DesiredRangeCM:          420,
		ChaseRangeCM:            760,
		RetreatRangeCM:          220,
		OrbitSpeedCMS:           220,
		ChaseSpeedCMS:           420,
		LungeSpeedCMS:           760,
		MaulSpeedCMS:            320,
		RetreatSpeedCMS:         360,
		DodgeSkillID:            "wolf_dodge",
		BiteRangeCM:             220,
		LungeMinRangeCM:         260,
		LungeMaxRangeCM:         760,
		MaulPressureThreshold:   0.65,
		MinOrbitDurationTicks:   90,
		SideSwitchCooldownTicks: 45,
		Bindings: []SkillBinding{
			{ID: "bind_lunge_circle", SkillID: "lunge", TacticalState: "circle", DecisionPhase: "reposition", MinRangeCM: 260, MaxRangeCM: 760, Priority: 90, UsageWeight: 1.1, Enabled: true, RequiresLineOfSight: true},
			{ID: "bind_maul_pressure", SkillID: "maul", TacticalState: "pressure", DecisionPhase: "counter", MinRangeCM: 0, MaxRangeCM: 260, Priority: 100, UsageWeight: 0.7, Enabled: true, RequiresLineOfSight: true},
			{ID: "bind_dodge_pressure", SkillID: "wolf_dodge", TacticalState: "pressure", DecisionPhase: "evade", MinRangeCM: 0, MaxRangeCM: 420, Priority: 110, UsageWeight: 1.2, Enabled: true},
		},
	}
}
