package combat

import (
	"context"
	"testing"
	"time"

	apeironv1 "db-apeiron/gen/apeiron/v1"
	"server-apeiron/internal/hitbox"
)

func TestImpactResolutionBlocksOnlyInsideDefenseArc(t *testing.T) {
	now := time.UnixMilli(1000)
	target := combatEntity(2, "test", 0, 0)
	target.components.Skills.State = "blocking"

	pipeline := NewImpactResolutionPipeline(nil, nil, nil, nil)
	frontSource := combatEntity(1, "test", 100, 0)
	front, err := pipeline.Apply(context.Background(), DamageContext{
		Source:  frontSource,
		Target:  target,
		Skill:   &apeironv1.Skill{Id: "blockable", BaseDamage: 100, IsBlockable: true},
		Hit:     hitbox.HitResult{TargetID: target.RuntimeID()},
		Now:     now,
		Defense: shieldGuardContract(),
		TargetCore: &apeironv1.CombatCoreProfile{
			CanBlock:                true,
			BlockDamageReduction:    0.6,
			PostureDamageMultiplier: 1,
		},
		Impact: &apeironv1.SkillImpactProfile{SkillId: "blockable", PoiseDamage: 20, GuardDamageMultiplier: 1.5},
	})
	if err != nil {
		t.Fatalf("front Apply returned error: %v", err)
	}
	if !front.Blocked {
		t.Fatalf("front hit was not blocked: %#v", front)
	}
	if front.FinalDamage != 40 {
		t.Fatalf("front final damage = %.1f, want 40", front.FinalDamage)
	}
	if front.PostureDamage != 30 {
		t.Fatalf("front posture damage = %.1f, want 30", front.PostureDamage)
	}

	backSource := combatEntity(3, "test", -100, 0)
	back, err := pipeline.Apply(context.Background(), DamageContext{
		Source:     backSource,
		Target:     target,
		Skill:      &apeironv1.Skill{Id: "blockable", BaseDamage: 100, IsBlockable: true},
		Hit:        hitbox.HitResult{TargetID: target.RuntimeID()},
		Now:        now,
		Defense:    shieldGuardContract(),
		TargetCore: &apeironv1.CombatCoreProfile{CanBlock: true, BlockDamageReduction: 0.6},
	})
	if err != nil {
		t.Fatalf("back Apply returned error: %v", err)
	}
	if back.Blocked {
		t.Fatalf("back hit should bypass block: %#v", back)
	}
	if back.FinalDamage != 100 {
		t.Fatalf("back final damage = %.1f, want 100", back.FinalDamage)
	}
}

func TestImpactResolutionParryMarksRiposteVulnerability(t *testing.T) {
	now := time.UnixMilli(2000)
	source := combatEntity(10, "test", 100, 0)
	target := combatEntity(20, "test", 0, 0)
	target.components.Skills.State = "parry_active"

	defense := NewDefenseRuntime()
	pipeline := NewImpactResolutionPipeline(defense, nil, nil, nil)
	result, err := pipeline.Apply(context.Background(), DamageContext{
		Source:     source,
		Target:     target,
		Skill:      &apeironv1.Skill{Id: "parryable", BaseDamage: 100, IsParryable: true},
		Hit:        hitbox.HitResult{TargetID: target.RuntimeID()},
		Now:        now,
		Defense:    perfectGuardContract(),
		TargetCore: &apeironv1.CombatCoreProfile{CanBlock: true, CanParry: true},
	})
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if !result.Parried || result.FinalDamage != 0 {
		t.Fatalf("result = %#v, want parried zero-damage result", result)
	}
	if !defense.State(source.RuntimeID(), now).RiposteVulnerableUntil.After(now) {
		t.Fatalf("source %d was not marked riposte-vulnerable", source.RuntimeID())
	}
}

func TestImpactResponseProfileUsesTargetKind(t *testing.T) {
	player := combatEntity(30, "test", 0, 0)
	creature := combatEntity(31, "test", 0, 0)
	creature.entityType = "creature"

	if profile := ImpactResponseProfileForEntity(player); profile != "flesh_blood_red" {
		t.Fatalf("player impact response = %q, want flesh_blood_red", profile)
	}
	if profile := ImpactResponseProfileForEntity(creature); profile != "creature_flesh_blood_red" {
		t.Fatalf("creature impact response = %q, want creature_flesh_blood_red", profile)
	}
}

func shieldGuardContract() *apeironv1.CombatDefenseContract {
	return &apeironv1.CombatDefenseContract{
		Id:                         "player_shield_guard_v1",
		DefenseType:                "shield_block",
		FrontalArcDeg:              120,
		StaminaDamageOnlyOnBlock:   true,
		HealthDamageOnUnblockedHit: true,
		PostureDamageOnBlock:       true,
		GuardDamageMultiplier:      1,
	}
}

func perfectGuardContract() *apeironv1.CombatDefenseContract {
	contract := shieldGuardContract()
	contract.Id = "player_perfect_guard_v1"
	contract.DefenseType = "perfect_block"
	contract.PerfectBlockWindowMs = 120
	contract.ParryWindowMs = 90
	return contract
}
