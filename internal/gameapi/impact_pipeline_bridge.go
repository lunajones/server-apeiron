package gameapi

import (
	"context"
	"strings"
	"time"

	dbv1 "db-apeiron/gen/apeiron/v1"
	"server-apeiron/internal/combat"
	domainentity "server-apeiron/internal/domain/entity"
	"server-apeiron/internal/domain/ids"
	domainmath "server-apeiron/internal/domain/math"
	"server-apeiron/internal/hitbox"
)

type runtimeEntityCombatAdapter struct {
	state      *entityState
	components domainentity.Components
}

func newRuntimeEntityCombatAdapter(state *entityState, now time.Time) *runtimeEntityCombatAdapter {
	if state == nil {
		return nil
	}
	facing := toDomainVector(yawVector(state.yaw))
	if facing.IsZero() {
		facing = domainmath.V3(1, 0, 0)
	}
	components := domainentity.Components{}
	components.Transform.Position = toDomainVector(state.position)
	components.Transform.Facing = facing
	components.Transform.RotationY = state.yaw
	components.Transform.Radius = runtimeEntityRadiusCM(state)
	components.Movement.Velocity = toDomainVector(state.velocity)
	components.Movement.Locomotion.AuthoritativeYaw = state.yaw
	components.Skills.CurrentSkillID = ids.SkillID(runtimeEntityCurrentSkillID(state))
	components.Skills.State = runtimeEntityCombatPipelineState(state)
	if state.skillRuntime != nil {
		components.Skills.StartedAtMS = state.skillRuntime.GetStartedAtMs()
		components.Skills.CooldownEndMS = state.skillRuntime.GetCooldownEndMs()
		components.Skills.LastResolvedAtMS = state.skillRuntime.GetLastResolvedAtMs()
	}
	components.Combat.ActionLockedUntil = state.actionLockedUntil
	if runtimeEntityHasIFrameState(state) {
		components.Status.Effects = map[string]time.Time{"iframe": now.Add(time.Second)}
		components.Combat.ControlImmuneUntil = now.Add(time.Second)
	}
	return &runtimeEntityCombatAdapter{state: state, components: components}
}

func (e *runtimeEntityCombatAdapter) RuntimeID() ids.RuntimeEntityID {
	if e == nil || e.state == nil {
		return 0
	}
	return ids.RuntimeEntityID(e.state.id)
}

func (e *runtimeEntityCombatAdapter) Ref() domainentity.Ref {
	return domainentity.Ref{RuntimeID: e.RuntimeID(), Type: e.EntityType()}
}

func (e *runtimeEntityCombatAdapter) RegionID() ids.RegionID {
	if e == nil || e.state == nil {
		return ""
	}
	return ids.RegionID(e.state.regionID)
}

func (e *runtimeEntityCombatAdapter) EntityType() domainentity.EntityType {
	if e == nil || e.state == nil {
		return ""
	}
	switch e.state.entityType {
	case "player":
		return domainentity.EntityTypePlayer
	case "creature":
		return domainentity.EntityTypeCreature
	default:
		return domainentity.EntityType(e.state.entityType)
	}
}

func (e *runtimeEntityCombatAdapter) ImpactResponseProfile() string {
	if e == nil || e.state == nil {
		return ""
	}
	return strings.TrimSpace(e.state.impactResponseProfile)
}

func (e *runtimeEntityCombatAdapter) Position() domainmath.Position {
	if e == nil || e.state == nil {
		return domainmath.Position{}
	}
	return toDomainVector(e.state.position)
}

func (e *runtimeEntityCombatAdapter) Facing() domainmath.Vec3 {
	if e == nil {
		return domainmath.Vec3{}
	}
	return e.components.Transform.Facing
}

func (e *runtimeEntityCombatAdapter) Radius() float64 {
	if e == nil || e.state == nil {
		return 0
	}
	return runtimeEntityRadiusCM(e.state)
}

func (e *runtimeEntityCombatAdapter) SetPosition(position domainmath.Position) {
	if e == nil || e.state == nil {
		return
	}
	e.state.position = fromDomainVector(position)
	e.components.Transform.Position = position
}

func (e *runtimeEntityCombatAdapter) SetVelocity(velocity domainmath.Vec3) {
	if e == nil || e.state == nil {
		return
	}
	e.state.velocity = fromDomainVector(velocity)
	e.components.Movement.Velocity = velocity
}

func (e *runtimeEntityCombatAdapter) Components() *domainentity.Components {
	if e == nil {
		return nil
	}
	return &e.components
}

func (r *Runtime) resolveRuntimeSkillImpact(source *entityState, target *entityState, skill SkillRuntimeContract, profile *dbv1.SkillHitboxProfile, start vector, dir vector) (runtimeSkillImpact, bool) {
	if r == nil || source == nil || target == nil || profile == nil {
		return runtimeSkillImpact{}, false
	}
	now := time.Now()
	pipeline := r.impact
	if pipeline == nil {
		pipeline = combat.NewImpactResolutionPipeline(nil, nil, nil, nil)
		r.impact = pipeline
	}

	sourceEntity := newRuntimeEntityCombatAdapter(source, now)
	targetEntity := newRuntimeEntityCombatAdapter(target, now)
	result, err := pipeline.Apply(context.Background(), combat.DamageContext{
		Source:      sourceEntity,
		Target:      targetEntity,
		Hit:         runtimeCombatHitResult(skill, profile, target, start, dir),
		Skill:       runtimeCombatSkill(skill),
		Impact:      runtimeCombatImpactProfile(skill, profile),
		SourceCore:  r.runtimeCombatCoreProfile(source),
		TargetCore:  r.runtimeCombatCoreProfile(target),
		Defense:     r.runtimeCombatDefenseContract(target),
		Now:         now,
		Tick:        r.tick,
		CurrentTick: r.tick,
	})
	if err != nil {
		return runtimeSkillImpact{}, false
	}
	return runtimeSkillImpact{
		SourceID:              source.id,
		TargetID:              target.id,
		SkillID:               skill.SkillID,
		ImpactType:            runtimeImpactType(skill, profile),
		ImpactResponseProfile: combat.ImpactResponseProfileForEntity(targetEntity),
		DamageApplied:         result.FinalDamage,
		PostureApplied:        result.PostureDamage,
		Blocked:               result.Blocked,
		Parried:               result.Parried,
	}, true
}

func runtimeImpactType(skill SkillRuntimeContract, profile *dbv1.SkillHitboxProfile) string {
	if profile != nil && strings.TrimSpace(profile.GetHitboxShape()) != "" {
		return strings.TrimSpace(profile.GetHitboxShape())
	}
	return "physical"
}

func runtimeCombatHitResult(skill SkillRuntimeContract, profile *dbv1.SkillHitboxProfile, target *entityState, start vector, dir vector) hitbox.HitResult {
	forwardDistance := 0.0
	if target != nil {
		rel := vector{x: target.position.x - start.x, y: target.position.y - start.y}
		forwardDistance = rel.x*dir.x + rel.y*dir.y
	}
	return hitbox.HitResult{
		SkillID:         ids.SkillID(skill.SkillID),
		HitboxID:        profile.GetId(),
		TargetID:        ids.RuntimeEntityID(target.id),
		ImpactPoint:     toDomainVector(target.position),
		ForwardDistance: forwardDistance,
		DamageGroupID:   profile.GetDamageGroupId(),
		MotionProfileID: func() string {
			if motion := profile.GetMotionProfile(); motion != nil {
				return motion.GetId()
			}
			return ""
		}(),
	}
}

func runtimeCombatSkill(skill SkillRuntimeContract) *dbv1.Skill {
	return &dbv1.Skill{
		Id:            skill.SkillID,
		BaseDamage:    skill.Damage,
		PostureDamage: skill.PostureDamage,
		IsBlockable:   skill.Blockable,
		IsParryable:   skill.Blockable,
		DamageType:    "physical",
	}
}

func runtimeCombatImpactProfile(skill SkillRuntimeContract, profile *dbv1.SkillHitboxProfile) *dbv1.SkillImpactProfile {
	return &dbv1.SkillImpactProfile{
		SkillId:               skill.SkillID,
		ImpactType:            profile.GetHitboxShape(),
		PoiseDamage:           skill.PostureDamage,
		GuardDamageMultiplier: 1,
	}
}

func (r *Runtime) runtimeCombatCoreProfile(entity *entityState) *dbv1.CombatCoreProfile {
	if r == nil {
		return nil
	}
	profile := r.contracts.combatCoreProfileForEntity(entity)
	if profile == nil {
		return nil
	}
	if entity == nil || entity.maxPosture <= 0 || entity.maxPosture == profile.GetMaxPosture() {
		return profile
	}
	copy := *profile
	copy.MaxPosture = entity.maxPosture
	return &copy
}

func (r *Runtime) runtimeCombatDefenseContract(target *entityState) *dbv1.CombatDefenseContract {
	if r == nil || target == nil {
		return nil
	}
	return r.contracts.defenseContractForEntity(target)
}

func runtimeEntityRadiusCM(entity *entityState) float64 {
	switch {
	case entity == nil:
		return 0
	case entity.entityType == "creature":
		return 55
	default:
		return 45
	}
}

func runtimeEntityCurrentSkillID(entity *entityState) string {
	if entity == nil || entity.skillRuntime == nil {
		return ""
	}
	return entity.skillRuntime.GetCurrentSkillId()
}

func runtimeEntityCombatPipelineState(entity *entityState) string {
	if entity == nil {
		return ""
	}
	state := strings.ToLower(strings.TrimSpace(entity.combatState))
	switch state {
	case "blocking", "block", "guard", "parry", "parry_active", "perfect_block", "iframe", "evade", "dodge":
		return state
	default:
		return strings.ToLower(strings.TrimSpace(entity.skillState))
	}
}

func runtimeEntityHasIFrameState(entity *entityState) bool {
	state := runtimeEntityCombatPipelineState(entity)
	return state == "iframe" || state == "evade" || state == "dodge" || strings.Contains(state, "iframe")
}
