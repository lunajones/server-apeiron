package gameapi

import (
	"context"
	"strconv"
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
	components.Skills.State = runtimeEntityCombatPipelineStateAt(state, now)
	if state.skillRuntime != nil {
		components.Skills.StartedAtMS = state.skillRuntime.GetStartedAtMs()
		components.Skills.CooldownEndMS = state.skillRuntime.GetCooldownEndMs()
		components.Skills.LastResolvedAtMS = state.skillRuntime.GetLastResolvedAtMs()
	}
	components.Combat.ActionLockedUntil = state.actionLockedUntil
	if runtimeEntityHasIFrameStateAt(state, now) {
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
		Source:         sourceEntity,
		Target:         targetEntity,
		Hit:            runtimeCombatHitResult(skill, profile, target, start, dir),
		Skill:          runtimeCombatSkill(skill),
		Impact:         runtimeCombatImpactProfile(skill, profile),
		ControlEffects: skill.ControlEffects,
		SourceCore:     r.runtimeCombatCoreProfile(source),
		TargetCore:     r.runtimeCombatCoreProfile(target),
		Defense:        r.runtimeCombatDefenseContract(target),
		Now:            now,
		Tick:           r.tick,
		CurrentTick:    r.tick,
	})
	if err != nil {
		return runtimeSkillImpact{}, false
	}
	// Slice 5: Muscles scale a player's outgoing physical damage (additive over the resolved base).
	finalDamage := result.FinalDamage
	if source.entityType == "player" && source.progression != nil {
		finalDamage *= attributePhysicalDamageMultiplier(source.progression)
	}
	if target.entityType == "player" && apeironDodgeDebugEnabled() {
		r.logDodgeDebugStateLocked("impact_resolved_against_player", target, map[string]string{
			"source_id": strconv.FormatUint(source.id, 10),
			"skill_id":  skill.SkillID,
			"damage":    strconv.FormatFloat(finalDamage, 'f', 3, 64),
			"posture":   strconv.FormatFloat(result.PostureDamage, 'f', 3, 64),
			"blocked":   strconv.FormatBool(result.Blocked),
			"parried":   strconv.FormatBool(result.Parried),
			"evaded":    strconv.FormatBool(result.Evaded),
			"reason":    result.Reason,
		})
	}
	appliedControl := runtimeSkillAppliedControlEffect(skill.ControlEffects, result.StatusApplied)
	if appliedControl != nil {
		r.applyRuntimeImpactControlMotionLocked(source, target, dir, appliedControl, now)
	}
	controlType, releasePolicy := runtimeSkillImpactControlMetadata(appliedControl)
	controlDistance, controlSpeed, controlDirection := runtimeSkillImpactControlMotionMetadata(appliedControl)
	return runtimeSkillImpact{
		SourceID:               source.id,
		TargetID:               target.id,
		SkillID:                skill.SkillID,
		ImpactType:             runtimeImpactType(skill, profile),
		ImpactResponseProfile:  combat.ImpactResponseProfileForEntity(targetEntity),
		StatusApplied:          append([]string(nil), result.StatusApplied...),
		ControlType:            controlType,
		ControlReleasePolicy:   releasePolicy,
		ControlDistanceCM:      controlDistance,
		ControlSpeedCMS:        controlSpeed,
		ControlDirectionPolicy: controlDirection,
		DamageApplied:          finalDamage,
		DamageType:             result.DamageType,
		DamageFamily:           result.DamageFamily,
		PostureApplied:         result.PostureDamage,
		Blocked:                result.Blocked,
		Parried:                result.Parried,
		Evaded:                 result.Evaded,
		Reason:                 result.Reason,
		TargetPipelineState:    runtimeEntityCombatPipelineStateAt(target, now),
		TargetIFrame:           runtimeEntityHasIFrameStateAt(target, now),
	}, true
}

func runtimeImpactType(skill SkillRuntimeContract, profile *dbv1.SkillHitboxProfile) string {
	if skill.Impact != nil && strings.TrimSpace(skill.Impact.GetImpactType()) != "" {
		return strings.TrimSpace(skill.Impact.GetImpactType())
	}
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
	if skill.Impact != nil {
		return skill.Impact
	}
	impactType := "physical"
	if profile != nil && strings.TrimSpace(profile.GetHitboxShape()) != "" {
		impactType = strings.TrimSpace(profile.GetHitboxShape())
	}
	return &dbv1.SkillImpactProfile{
		SkillId:               skill.SkillID,
		ImpactType:            impactType,
		PoiseDamage:           skill.PostureDamage,
		GuardDamageMultiplier: 1,
	}
}

func runtimeSkillAppliedControlEffect(effects []*dbv1.SkillControlEffect, applied []string) *dbv1.SkillControlEffect {
	if len(effects) == 0 || len(applied) == 0 {
		return nil
	}
	appliedSet := make(map[string]struct{}, len(applied))
	for _, statusID := range applied {
		if statusID == "" {
			continue
		}
		appliedSet[statusID] = struct{}{}
	}
	for _, effect := range effects {
		if effect == nil {
			continue
		}
		if _, ok := appliedSet[effect.GetStatusEffectId()]; !ok {
			continue
		}
		return effect
	}
	return nil
}

func runtimeSkillImpactControlMetadata(effect *dbv1.SkillControlEffect) (string, string) {
	if effect == nil {
		return "", ""
	}
	return strings.TrimSpace(effect.GetControlType()), strings.TrimSpace(effect.GetReleasePolicyId())
}

func runtimeSkillImpactControlMotionMetadata(effect *dbv1.SkillControlEffect) (float64, float64, string) {
	if effect == nil {
		return 0, 0, ""
	}
	contract := runtimeImpactControlActionContract(effect)
	return contract.DistanceCM, contract.BaseSpeedCMS, strings.TrimSpace(effect.GetDirectionPolicy())
}

func (r *Runtime) runtimeCombatCoreProfile(entity *entityState) *dbv1.CombatCoreProfile {
	if r == nil {
		return nil
	}
	profile := r.contracts.combatCoreProfileForEntity(entity)
	if profile == nil {
		return nil
	}
	needsPostureCopy := entity != nil && entity.maxPosture > 0 && entity.maxPosture != profile.GetMaxPosture()
	// Slice 5: Resilience adds to a player's physical resistance rating (additive over the base profile).
	resistanceBonus := 0.0
	if entity != nil && entity.entityType == "player" && entity.progression != nil {
		resistanceBonus = attributePhysicalResistanceBonus(entity.progression)
	}
	if !needsPostureCopy && resistanceBonus == 0 {
		return profile
	}
	copy := *profile
	if needsPostureCopy {
		copy.MaxPosture = entity.maxPosture
	}
	if resistanceBonus != 0 {
		copy.PhysicalResistanceRating = profile.GetPhysicalResistanceRating() + resistanceBonus
	}
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
	return runtimeEntityCombatPipelineStateAt(entity, time.Now())
}

func runtimeEntityCombatPipelineStateAt(entity *entityState, now time.Time) string {
	if entity == nil {
		return ""
	}
	if runtimeEntityDodgeMotionActiveAt(entity, now) {
		return "dodge"
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
	return runtimeEntityHasIFrameStateAt(entity, time.Now())
}

func runtimeEntityHasIFrameStateAt(entity *entityState, now time.Time) bool {
	state := runtimeEntityCombatPipelineStateAt(entity, now)
	return state == "iframe" || state == "evade" || state == "dodge" || strings.Contains(state, "iframe")
}

func runtimeEntityDodgeMotionActiveAt(entity *entityState, now time.Time) bool {
	if entity == nil || entity.actionMotion == nil {
		return false
	}
	motion := entity.actionMotion
	if !strings.EqualFold(strings.TrimSpace(motion.MotionSource), "owned_locomotion") &&
		!strings.EqualFold(strings.TrimSpace(motion.MotionSource), "skill_root") {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(motion.Contract.ActionType), "dodge") &&
		!strings.EqualFold(strings.TrimSpace(motion.Contract.AbilityKey), "dodge") {
		return false
	}
	if motion.StartedAt.IsZero() {
		return false
	}
	duration := durationFromMS(motion.Contract.DurationMS)
	if duration <= 0 {
		duration = durationFromMS(motion.Contract.ActiveMS + motion.Contract.RecoveryMS)
	}
	if duration <= 0 {
		return false
	}
	elapsed := now.Sub(motion.StartedAt)
	return elapsed >= 0 && elapsed <= duration
}
