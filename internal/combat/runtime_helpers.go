package combat

import (
	"math"
	"time"

	apeironv1 "db-apeiron/gen/apeiron/v1"
	"server-apeiron/internal/combat/actionruntime"
	domainentity "server-apeiron/internal/domain/entity"
	"server-apeiron/internal/domain/ids"
	domainmath "server-apeiron/internal/domain/math"
	"server-apeiron/internal/movement"
)

const (
	movementLockDuringWindup     = "windup"
	movementLockDuringActive     = "active"
	movementLockUntilRecoveryEnd = "recovery"
	movementLockFullAction       = "full_action"
)

const (
	ActionRuntimeEventInstanceCreated          = "instance_created"
	ActionRuntimeEventMovementContractResolved = "movement_contract_resolved"
	ActionRuntimeEventContractMissing          = "contract_missing"
	ActionRuntimeEventQueueAccepted            = "queue_accepted"
	ActionRuntimeEventQueueRejected            = "queue_rejected"
)

func attackProfileMissingRuntimeData(profile AttackProfile) bool {
	return profile.Skill == nil || (len(profile.Hitboxes) == 0 && profile.Projectile == nil && profile.Area == nil)
}

func skillMovementContractRequiredForProfile(profile AttackProfile) bool {
	if profile.Skill == nil {
		return false
	}
	for _, tag := range profile.Skill.Tags {
		if tag == "requires_movement_contract" || tag == "skill_movement" {
			return true
		}
	}
	return false
}

func ActionTimingFromProfile(profile AttackProfile) ActionTimingConfig {
	timing := profile.Timing
	if timing == nil {
		return ActionTimingConfig{
			Windup:         100 * time.Millisecond,
			ActiveStart:    100 * time.Millisecond,
			ActiveEnd:      180 * time.Millisecond,
			Recovery:       120 * time.Millisecond,
			ActionLock:     300 * time.Millisecond,
			GlobalCooldown: profile.Cooldown,
		}
	}
	return ActionTimingConfig{
		Windup:             msDuration(timing.GetWindupMs()),
		ActiveStart:        msDuration(timing.GetActiveStartMs()),
		ActiveEnd:          msDuration(timing.GetActiveEndMs()),
		Recovery:           msDuration(timing.GetRecoveryMs()),
		ActionLock:         msDuration(timing.GetActionLockMs()),
		GlobalCooldown:     msDuration(timing.GetGlobalCooldownMs()),
		MovementLockPolicy: timing.GetMovementLockPolicy(),
	}
}

func actionRuntimeTimingFromConfig(timing ActionTimingConfig) actionruntime.Timing {
	return actionruntime.Timing{
		Windup:         timing.Windup,
		Active:         timing.ActiveEnd - timing.ActiveStart,
		Recovery:       timing.Recovery,
		Cooldown:       timing.GlobalCooldown,
		ActionLock:     timing.ActionLock,
		GlobalCooldown: timing.GlobalCooldown,
	}
}

func actionTimingConfigFromRuntime(timing actionruntime.Timing) ActionTimingConfig {
	activeStart := timing.Windup
	activeEnd := timing.Windup + timing.Active
	return ActionTimingConfig{
		Windup:         timing.Windup,
		ActiveStart:    activeStart,
		ActiveEnd:      activeEnd,
		Recovery:       timing.Recovery,
		ActionLock:     firstPositiveDuration(timing.ActionLock, activeEnd+timing.Recovery),
		GlobalCooldown: firstPositiveDuration(timing.GlobalCooldown, timing.Cooldown),
	}
}

func (s *RegionPlayerSkillCombatSystem) recordActionRuntimeEvent(event ActionRuntimeEvent) {
	if s == nil {
		return
	}
	s.LastActionRuntimeEvents = append(s.LastActionRuntimeEvents, event)
	switch event.Kind {
	case ActionRuntimeEventContractMissing:
		s.ActionRuntimeCounters.MissingContracts++
	}
}

func (s *RegionPlayerSkillCombatSystem) updatePlayerActionPhase(entityID ids.RuntimeEntityID, now time.Time, reason string) {
	if s == nil || !entityID.Valid() {
		return
	}
	state, ok := s.actionStates[entityID]
	if !ok || state.Instance.StartedAt.IsZero() {
		return
	}
	phase := state.Instance.PhaseAt(now)
	s.recordActionRuntimeEvent(ActionRuntimeEvent{
		Kind:             "phase_update",
		ActorKind:        state.Instance.ActorKind,
		ActionKind:       state.Instance.ActionKind,
		EntityID:         entityID,
		SkillID:          state.Instance.SkillID,
		ActionInstanceID: state.Instance.InstanceID,
		At:               now,
		Reason:           string(phase) + ":" + reason,
	})
}

func actionRuntimeContractMissingEvent(actorKind actionruntime.ActorKind, entityID ids.RuntimeEntityID, skillID ids.SkillID, at time.Time, reason string) ActionRuntimeEvent {
	return ActionRuntimeEvent{Kind: ActionRuntimeEventContractMissing, ActorKind: actorKind, EntityID: entityID, SkillID: skillID, At: at, Reason: reason}
}

func actionRuntimeEventFromInstance(kind string, instance actionruntime.Instance, at time.Time, reason string) ActionRuntimeEvent {
	return ActionRuntimeEvent{
		Kind:             kind,
		ActorKind:        instance.ActorKind,
		ActionKind:       instance.ActionKind,
		EntityID:         instance.EntityID,
		SkillID:          instance.SkillID,
		ActionInstanceID: instance.InstanceID,
		At:               at,
		Reason:           reason,
	}
}

func combatOutcomeReason(result DamageResult) string {
	switch {
	case result.Parried:
		return "parried"
	case result.Blocked:
		return "blocked"
	case result.Evaded:
		return "evaded"
	case result.Killed:
		return "killed"
	default:
		return "hit"
	}
}

func ImpactResponseProfileForEntity(target any) string {
	if provider, ok := target.(interface {
		ImpactResponseProfile() string
	}); ok && provider != nil {
		if profile := provider.ImpactResponseProfile(); profile != "" {
			return profile
		}
	}
	entity, ok := target.(interface {
		EntityType() domainentity.EntityType
	})
	if !ok || entity == nil {
		return "default"
	}
	switch entity.EntityType() {
	case domainentity.EntityTypePlayer:
		return "flesh_blood_red"
	case domainentity.EntityTypeCreature:
		return "creature_flesh_blood_red"
	default:
		return "default"
	}
}

func statusEffectFromSkillControlEffect(control *apeironv1.SkillControlEffect) *apeironv1.StatusEffect {
	if control == nil || !control.GetEnabled() {
		return nil
	}
	duration := control.GetDurationMs()
	return &apeironv1.StatusEffect{Id: control.GetStatusEffectId(), DurationMs: duration, BlocksMovement: true, BlocksActions: true, BlocksSkills: true}
}

func movementPoisePolicyIgnoresStagger(policy string) bool {
	return policy == "ignore_stagger" || policy == "hyperarmor"
}

func skillInterruptPolicyIgnoresStagger(policy string) bool {
	return policy == "ignore_stagger" || policy == "hyperarmor"
}

func controlReleasePolicyHas(policyID string, token string) bool {
	if policyID == "" || token == "" {
		return false
	}
	return policyID == token || containsPolicyToken(policyID, token)
}

func containsPolicyToken(policyID string, token string) bool {
	for i := 0; i+len(token) <= len(policyID); i++ {
		if policyID[i:i+len(token)] == token {
			return true
		}
	}
	return false
}

func msDuration(value int32) time.Duration {
	return time.Duration(value) * time.Millisecond
}

func stringPointer(value string) *string {
	return &value
}

func int32Pointer(value int32) *int32 {
	return &value
}

func firstPositiveFloat(values ...float64) float64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func firstPositiveDuration(values ...time.Duration) time.Duration {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func yawToDirection(yawDeg float64) domainmath.Vec3 {
	rad := yawDeg * math.Pi / 180
	return domainmath.V3(math.Cos(rad), math.Sin(rad), 0).Normalize()
}

func clamp(value float64, min float64, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func sampleSkillMovementCurve(samples []movement.MovementActionCurvePoint, progress float64) float64 {
	return movement.MovementActionCurve{Samples: samples}.Sample(clamp(progress, 0, 1))
}

func integrateSkillMovementCurveProgress(samples []movement.MovementActionCurvePoint, progress float64) float64 {
	progress = clamp(progress, 0, 1)
	if progress <= 0 {
		return 0
	}
	if len(samples) == 0 {
		return progress
	}
	const steps = 16
	total := 0.0
	step := progress / steps
	prev := sampleSkillMovementCurve(samples, 0)
	for i := 1; i <= steps; i++ {
		t := float64(i) * step
		next := sampleSkillMovementCurve(samples, t)
		total += (prev + next) * 0.5 * step
		prev = next
	}
	return clamp(total, 0, 1)
}

func skillMovementDistanceAtElapsed(cfg skillMovementConfig, elapsed time.Duration, duration time.Duration, totalDistance float64) float64 {
	if duration <= 0 {
		return totalDistance
	}
	return totalDistance * clamp(float64(elapsed)/float64(duration), 0, 1)
}

func skillMovementSpeedScaleForProgress(cfg skillMovementConfig, progress float64) float64 {
	return 1
}

func skillMovementContractHorizontalSpeed(contract movement.MovementActionContract) float64 {
	return firstPositiveFloat(contract.BaseSpeedCMPerSec)
}

func skillMovementContactStopDistance(args ...any) float64 {
	cfg := skillMovementConfig{}
	radius := 0.0
	for _, arg := range args {
		switch value := arg.(type) {
		case skillMovementConfig:
			cfg = value
		case float64:
			radius = value
		}
	}
	return firstPositiveFloat(cfg.DesiredLandingDistance, cfg.MinLandingDistance, radius)
}
