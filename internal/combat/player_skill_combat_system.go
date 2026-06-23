package combat

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	apeironv1 "db-apeiron/gen/apeiron/v1"
	"server-apeiron/internal/combat/actionruntime"
	domainentity "server-apeiron/internal/domain/entity"
	"server-apeiron/internal/domain/ids"
	domainmath "server-apeiron/internal/domain/math"
	"server-apeiron/internal/domain/world"
	"server-apeiron/internal/hitbox"
	"server-apeiron/internal/logging"
	"server-apeiron/internal/movement"
	"server-apeiron/internal/pvp"
	regionruntime "server-apeiron/internal/runtime/region"
	"server-apeiron/internal/skill"
	"server-apeiron/internal/spatial"
)

type PlayerSkillIntentSource interface {
	Consume(ids.RuntimeEntityID) []skill.Intent
}

type CreatureActionInterrupter interface {
	InterruptCreatureAction(domainentity.Entity, time.Time, string) bool
}

type PlayerSkillMovementContractProvider interface {
	MovementActionContract(context.Context, domainentity.Entity, ids.SkillID) (movement.MovementActionContract, bool)
}

type RegionPlayerSkillCombatSystem struct {
	Regions                 RegionSource
	Intents                 PlayerSkillIntentSource
	Profiles                SkillAttackProfileProvider
	Defense                 *DefenseRuntime
	Hitboxes                *hitbox.Runtime
	Damage                  *ImpactResolutionPipeline
	CreatureInterrupts      CreatureActionInterrupter
	MovementContracts       PlayerSkillMovementContractProvider
	CastPipeline            skill.CastPipeline
	SpatialConfig           spatial.LooseQuadtreeConfig
	LastResults             []DamageResult
	LastOutcomes            []AttackOutcome
	LastHitboxDebugEvents   []HitboxDebugEvent
	LastAttempts            int
	LastMisses              []SkillMiss
	LastActionRuntimeEvents []ActionRuntimeEvent
	ActionRuntimeCounters   ActionRuntimeCounters
	actionStates            map[ids.RuntimeEntityID]playerActionRuntimeState
}

type PlayerIncomingThreatSnapshot struct {
	SourceID     ids.RuntimeEntityID
	RegionID     ids.RegionID
	SkillID      ids.SkillID
	Phase        string
	Position     domainmath.Position
	Forward      domainmath.Vec3
	RangeCM      float64
	ThreatScore  float64
	StartedAt    time.Time
	ActiveAt     time.Time
	ExpiresAt    time.Time
	Tick         uint64
	TargetLocked bool
}

type PlayerSkillCommandValidation struct {
	Accepted         bool
	Code             string
	Message          string
	SkillID          ids.SkillID
	Remaining        time.Duration
	Cooldown         time.Duration
	Scope            string
	Timing           ActionTimingConfig
	Queued           bool
	QueueIn          time.Duration
	ComboIndex       int64
	ComboWindow      time.Duration
	ActionInstanceID string
	ActionKind       string
	Phase            string

	MovementContract    movement.MovementActionContract
	HasMovementContract bool
}

type SkillMiss struct {
	SkillID        ids.SkillID
	SourceID       ids.RuntimeEntityID
	Reason         string
	Origin         domainmath.Position
	AimDirection   domainmath.Vec3
	TargetID       ids.RuntimeEntityID
	TargetPosition domainmath.Position
	HasTarget      bool
	HasPosition    bool
	HitboxCount    int
	Tick           uint64
}

type queuedPlayerSkillAction struct {
	Intent    skill.Intent
	ExecuteAt time.Time
	QueuedAt  time.Time
}

type playerActionRuntimeState struct {
	Instance            actionruntime.Instance
	RangeCM             float64
	MovementLockedUntil time.Time
	Queued              queuedPlayerSkillAction
	HasQueued           bool
	Execution           playerActionExecutionState
	HasExecution        bool
}

type playerActionExecutionState struct {
	Profile                           AttackProfile
	Intent                            skill.Intent
	StartedAt                         time.Time
	ResolveAt                         time.Time
	HitboxStart                       time.Duration
	HitboxEnd                         time.Duration
	Timing                            ActionTimingConfig
	Tick                              uint64
	InstanceID                        string
	CommittedMoveDirection            domainmath.Vec3
	CommittedTargetPosition           domainmath.Position
	ActionStartPosition               domainmath.Position
	MovementTargetCommitted           bool
	ContactTargetID                   ids.RuntimeEntityID
	ContactControlApplied             bool
	ContactInterruptApplied           bool
	ContactTargetReleased             bool
	ContactControlAppliedByTarget     map[ids.RuntimeEntityID]bool
	ContactInterruptAppliedByTarget   map[ids.RuntimeEntityID]bool
	AppliedTargetPushDistanceByTarget map[ids.RuntimeEntityID]float64
	AppliedMovementDistance           float64
	AppliedTargetPushDistance         float64
	PreMovementContactPushTick        uint64
	LastHitboxEvaluationAt            time.Time
	LastHitboxEvaluationOrigin        domainmath.Position
	ImpactResolved                    bool
	ReleaseStartedAt                  time.Time
	ReleaseTargetID                   ids.RuntimeEntityID
	ReleaseDirection                  domainmath.Vec3
	ReleaseDistance                   float64
	ReleaseAppliedDistance            float64
	ReleaseSpeed                      float64
	ReleaseStunMS                     int32
	MovementContract                  movement.MovementActionContract
	HasMovementContract               bool
}

type pendingPlayerSkillAction = playerActionExecutionState

type playerSkillContactCandidate struct {
	target    domainentity.Entity
	projected float64
	distance  float64
	priority  int
}

const playerBasicAttackComboGroup = "old_china_basic_attack"

var playerBasicAttackComboSkillIDs = []ids.SkillID{
	"player_basic_attack_1",
	"player_basic_attack_2",
	"player_basic_attack_3",
}

func NewRegionPlayerSkillCombatSystem(regions RegionSource, intents PlayerSkillIntentSource, defense *DefenseRuntime, profiles SkillAttackProfileProvider) *RegionPlayerSkillCombatSystem {
	if defense == nil {
		defense = NewDefenseRuntime()
	}
	return &RegionPlayerSkillCombatSystem{
		Regions:      regions,
		Intents:      intents,
		Profiles:     profiles,
		Defense:      defense,
		Hitboxes:     hitbox.NewRuntime(),
		Damage:       NewImpactResolutionPipeline(defense, nil, pvp.NewValidator(), nil),
		CastPipeline: skill.NewCastPipeline(nil, nil, skill.PipelineConfig{}),
		actionStates: make(map[ids.RuntimeEntityID]playerActionRuntimeState),
	}
}

func (s *RegionPlayerSkillCombatSystem) Tick(ctx context.Context, now time.Time, tick uint64) error {
	if s == nil || s.Regions == nil || s.Intents == nil {
		return nil
	}
	if now.IsZero() {
		now = time.Now()
	}
	if s.Defense == nil {
		s.Defense = NewDefenseRuntime()
	}
	if s.Hitboxes == nil {
		s.Hitboxes = hitbox.NewRuntime()
	}
	if s.Damage == nil {
		s.Damage = NewImpactResolutionPipeline(s.Defense, nil, pvp.NewValidator(), nil)
	}
	if s.CastPipeline.Cooldowns == nil {
		s.CastPipeline = skill.NewCastPipeline(nil, nil, skill.PipelineConfig{})
	}
	s.ensurePlayerActionStates()
	s.LastResults = s.LastResults[:0]
	s.LastOutcomes = s.LastOutcomes[:0]
	s.LastHitboxDebugEvents = s.LastHitboxDebugEvents[:0]
	s.LastAttempts = 0
	s.LastMisses = s.LastMisses[:0]
	for _, region := range s.Regions.Active() {
		if err := ctx.Err(); err != nil {
			return err
		}
		if region == nil || region.Entities() == nil {
			continue
		}
		if err := s.tickRegion(ctx, region, now, tick); err != nil {
			return err
		}
	}
	return nil
}

func (s *RegionPlayerSkillCombatSystem) ValidateIntent(ctx context.Context, source domainentity.Entity, intent skill.Intent, now time.Time) PlayerSkillCommandValidation {
	if now.IsZero() {
		now = time.Now()
	}
	if source == nil {
		return PlayerSkillCommandValidation{Code: "player_entity_not_found", Message: "attached player entity was not found"}
	}
	if intent.Action != skill.IntentCast || intent.SkillID == "" {
		return PlayerSkillCommandValidation{Code: "invalid_combat_command", Message: "combat command payload is invalid"}
	}
	if s.CastPipeline.Cooldowns == nil {
		s.CastPipeline = skill.NewCastPipeline(nil, nil, skill.PipelineConfig{})
	}
	intent = s.resolveBasicAttackComboIntent(ctx, source, intent, now)
	profile := s.playerAttackProfileForSkill(ctx, source, intent.SkillID, now, "validate_intent")
	if attackProfileMissingRuntimeData(profile) {
		skillID := intent.SkillID
		if profile.Skill != nil {
			skillID = ids.SkillID(profile.Skill.GetId())
		}
		return PlayerSkillCommandValidation{
			Code:             "action_profile_missing",
			Message:          "combat action profile is missing runtime hitbox/projectile/area data",
			SkillID:          skillID,
			Scope:            "profile",
			ActionInstanceID: playerActionInstanceID(source, intent, skillID, 0),
			ActionKind:       string(playerActionKindForSkill(intent, skillID)),
			Phase:            string(actionruntime.PhaseAccepted),
		}
	}
	castSkill := actionCombatCastSkill(profile, intent)
	skillID := ids.SkillID(profile.Skill.GetId())
	movementContract, hasMovementContract := s.movementContractForSkill(ctx, source, skillID)
	timing := ActionTimingFromProfile(profile)
	timing = playerActionTimingForRuntimeWithContract(profile, timing, movementContract, hasMovementContract)
	cooldown := playerSkillCooldown(profile, timing)
	s.clearNonCooldownPlayerSkill(source, profile)
	comboIndex := int64(0)
	comboWindow := time.Duration(0)
	if profile.Skill != nil {
		comboIndex = profile.Skill.GetComboIndex()
		if comboWindowMS := profile.Skill.GetComboWindowMs(); comboWindowMS > 0 {
			comboWindow = time.Duration(comboWindowMS) * time.Millisecond
		}
	}
	withProfileMetadata := func(validation PlayerSkillCommandValidation) PlayerSkillCommandValidation {
		validation.ActionInstanceID = playerActionInstanceID(source, intent, skillID, 0)
		validation.ActionKind = string(playerActionKindForSkill(intent, skillID))
		if validation.Phase == "" {
			validation.Phase = string(actionruntime.PhaseAccepted)
		}
		if comboIndex > 0 {
			validation.ComboIndex = comboIndex
		}
		if comboWindow > 0 {
			validation.ComboWindow = comboWindow
		}
		if hasMovementContract {
			validation.MovementContract = movementContract
			validation.HasMovementContract = true
		}
		return validation
	}
	if skillMovementContractRequiredForProfile(profile) && !hasMovementContract {
		s.recordActionRuntimeEvent(actionRuntimeContractMissingEvent(actionruntime.ActorKindPlayer, source.RuntimeID(), skillID, now, "validate_intent"))
		return withProfileMetadata(PlayerSkillCommandValidation{
			Code:    "movement_contract_missing",
			Message: "skill movement action contract is required for this migrated action",
			SkillID: skillID,
			Scope:   "movement_contract",
			Timing:  timing,
		})
	}
	if remaining := s.remainingActionLock(source.RuntimeID(), now); remaining > 0 {
		if s.canQueueDuringCurrentAction(source.RuntimeID(), timing, now, remaining) {
			return withProfileMetadata(PlayerSkillCommandValidation{
				Accepted:  true,
				SkillID:   skillID,
				Remaining: remaining,
				Cooldown:  cooldown,
				Scope:     "action",
				Timing:    timing,
				Queued:    true,
				QueueIn:   remaining,
			})
		}
		return withProfileMetadata(PlayerSkillCommandValidation{
			Code:      "action_locked",
			Message:   "combat action is locked by current action timing",
			SkillID:   skillID,
			Remaining: remaining,
			Cooldown:  cooldown,
			Scope:     "action",
			Timing:    timing,
		})
	}
	if remaining := s.remainingGlobalLock(source.RuntimeID(), now); remaining > 0 {
		if canQueueAction(timing, remaining) {
			return withProfileMetadata(PlayerSkillCommandValidation{
				Accepted:  true,
				SkillID:   skillID,
				Remaining: remaining,
				Cooldown:  cooldown,
				Scope:     "global",
				Timing:    timing,
				Queued:    true,
				QueueIn:   remaining,
			})
		}
		return withProfileMetadata(PlayerSkillCommandValidation{
			Code:      "cooldown_active",
			Message:   "global combat cooldown is active",
			SkillID:   skillID,
			Remaining: remaining,
			Cooldown:  cooldown,
			Scope:     "global",
			Timing:    timing,
		})
	}
	cast := skill.CastContext{
		Caster:         source,
		SkillID:        intent.SkillID,
		Target:         intent.Target,
		HasTarget:      intent.HasTarget,
		TargetPosition: intent.TargetPosition,
		HasPosition:    intent.HasPosition,
		AimDirection:   intent.AimDirection,
		HasAim:         intent.HasAim,
		ClientTick:     intent.ClientTick,
		RequestedAt:    intent.ReceivedAt,
	}
	if err := s.CastPipeline.Validate(ctx, cast, castSkill, now); err != nil {
		validation := PlayerSkillCommandValidation{Code: "combat_command_rejected", Message: err.Error(), SkillID: skillID, Cooldown: cooldown, Timing: timing}
		switch {
		case errors.Is(err, skill.ErrOnCooldown):
			validation.Code = "cooldown_active"
			validation.Message = "combat action is on cooldown"
			if s.CastPipeline.Cooldowns != nil {
				validation.Remaining = s.CastPipeline.Cooldowns.Remaining(source.RuntimeID(), skillID, now)
			}
			validation.Scope = "skill"
		case errors.Is(err, skill.ErrInsufficientMana):
			validation.Code = "insufficient_mana"
		case errors.Is(err, skill.ErrInsufficientHealth):
			validation.Code = "insufficient_health"
		case errors.Is(err, skill.ErrTargetRequired):
			validation.Code = "target_required"
		}
		return withProfileMetadata(validation)
	}
	return withProfileMetadata(PlayerSkillCommandValidation{Accepted: true, SkillID: skillID, Cooldown: cooldown, Timing: timing})
}

func (s *RegionPlayerSkillCombatSystem) tickRegion(ctx context.Context, region *regionruntime.RegionRuntime, now time.Time, tick uint64) error {
	entities := region.Entities().All()
	s.releaseExpiredActionStates(entities, now)
	s.releaseExpiredMovementLocks(entities, now)
	s.applyRegionSafeZones(region, entities)
	index, err := s.spatialIndex(region, entities)
	if err != nil {
		return err
	}
	resolver := hitbox.EntityResolverFunc(func(id ids.RuntimeEntityID) (domainentity.Entity, bool) {
		return region.Entities().Get(id)
	})
	for _, entity := range entities {
		if entity == nil || entity.EntityType() != domainentity.EntityTypePlayer {
			continue
		}
		if outcomes, resolved, err := s.resolvePendingSkill(ctx, region, entity, index, resolver, now, tick); err != nil {
			return err
		} else if resolved {
			for _, outcome := range outcomes {
				s.LastResults = append(s.LastResults, outcome.Result)
				s.LastOutcomes = append(s.LastOutcomes, outcome)
			}
		}
		queuedOutcomes, err := s.castQueued(ctx, region, entity, index, resolver, now, tick)
		if err != nil {
			return err
		}
		for _, outcome := range queuedOutcomes {
			s.LastResults = append(s.LastResults, outcome.Result)
			s.LastOutcomes = append(s.LastOutcomes, outcome)
		}
		for _, intent := range s.Intents.Consume(entity.RuntimeID()) {
			if intent.Action != skill.IntentCast {
				continue
			}
			outcomes, err := s.cast(ctx, region, entity, intent, index, resolver, now, tick)
			if err != nil {
				return err
			}
			for _, outcome := range outcomes {
				s.LastResults = append(s.LastResults, outcome.Result)
				s.LastOutcomes = append(s.LastOutcomes, outcome)
			}
		}
	}
	return nil
}

func (s *RegionPlayerSkillCombatSystem) cast(ctx context.Context, region *regionruntime.RegionRuntime, source domainentity.Entity, intent skill.Intent, index spatial.SpatialIndex, resolver hitbox.EntityResolver, now time.Time, tick uint64) ([]AttackOutcome, error) {
	intent = s.resolveBasicAttackComboIntent(ctx, source, intent, now)
	profile := s.playerAttackProfileForSkill(ctx, source, intent.SkillID, now, "cast")
	if attackProfileMissingRuntimeData(profile) {
		s.recordMiss(skillMissContext{
			Source:   source,
			Intent:   intent,
			Profile:  profile,
			Aim:      s.aimDirection(source, intent, resolver),
			Reason:   "action_profile_missing",
			Tick:     tick,
			Resolver: resolver,
		})
		return nil, nil
	}
	castSkill := actionCombatCastSkill(profile, intent)
	skillID := ids.SkillID(profile.Skill.GetId())
	movementContract, hasMovementContract := s.movementContractForSkill(ctx, source, skillID)
	timing := ActionTimingFromProfile(profile)
	timing = playerActionTimingForRuntimeWithContract(profile, timing, movementContract, hasMovementContract)
	cooldown := playerSkillCooldown(profile, timing)
	s.clearNonCooldownPlayerSkill(source, profile)
	if skillMovementContractRequiredForProfile(profile) && !hasMovementContract {
		s.recordActionRuntimeEvent(actionRuntimeContractMissingEvent(actionruntime.ActorKindPlayer, source.RuntimeID(), skillID, now, "cast"))
		s.recordMiss(skillMissContext{
			Source:   source,
			Intent:   intent,
			Profile:  profile,
			Aim:      s.aimDirection(source, intent, resolver),
			Reason:   "movement_contract_missing",
			Tick:     tick,
			Resolver: resolver,
		})
		return nil, nil
	}
	cast := skill.CastContext{
		Caster:         source,
		SkillID:        intent.SkillID,
		Target:         intent.Target,
		HasTarget:      intent.HasTarget,
		TargetPosition: intent.TargetPosition,
		HasPosition:    intent.HasPosition,
		AimDirection:   intent.AimDirection,
		HasAim:         intent.HasAim,
		ClientTick:     intent.ClientTick,
		RequestedAt:    intent.ReceivedAt,
	}
	if remaining := s.remainingActionLock(source.RuntimeID(), now); remaining > 0 {
		if s.canQueueDuringCurrentAction(source.RuntimeID(), timing, now, remaining) {
			s.queueAction(source.RuntimeID(), intent, now.Add(remaining), now)
			s.recordActionRuntimeEvent(ActionRuntimeEvent{
				Kind:             ActionRuntimeEventQueueAccepted,
				At:               now,
				EntityID:         source.RuntimeID(),
				ActorKind:        actionruntime.ActorKindPlayer,
				ActionKind:       playerActionKindForSkill(intent, skillID),
				SkillID:          skillID,
				ActionInstanceID: playerActionInstanceID(source, intent, skillID, tick),
				Reason:           "action_lock_queue_window",
			})
		} else {
			s.recordActionRuntimeEvent(ActionRuntimeEvent{
				Kind:             ActionRuntimeEventQueueRejected,
				At:               now,
				EntityID:         source.RuntimeID(),
				ActorKind:        actionruntime.ActorKindPlayer,
				ActionKind:       playerActionKindForSkill(intent, skillID),
				SkillID:          skillID,
				ActionInstanceID: playerActionInstanceID(source, intent, skillID, tick),
				Reason:           "action_lock_outside_queue_window",
			})
		}
		return nil, nil
	}
	if remaining := s.remainingGlobalLock(source.RuntimeID(), now); remaining > 0 {
		if canQueueAction(timing, remaining) {
			s.queueAction(source.RuntimeID(), intent, now.Add(remaining), now)
			s.recordActionRuntimeEvent(ActionRuntimeEvent{
				Kind:             ActionRuntimeEventQueueAccepted,
				At:               now,
				EntityID:         source.RuntimeID(),
				ActorKind:        actionruntime.ActorKindPlayer,
				ActionKind:       playerActionKindForSkill(intent, skillID),
				SkillID:          skillID,
				ActionInstanceID: playerActionInstanceID(source, intent, skillID, tick),
				Reason:           "global_lock_queue_window",
			})
		} else {
			s.recordActionRuntimeEvent(ActionRuntimeEvent{
				Kind:             ActionRuntimeEventQueueRejected,
				At:               now,
				EntityID:         source.RuntimeID(),
				ActorKind:        actionruntime.ActorKindPlayer,
				ActionKind:       playerActionKindForSkill(intent, skillID),
				SkillID:          skillID,
				ActionInstanceID: playerActionInstanceID(source, intent, skillID, tick),
				Reason:           "global_lock_outside_queue_window",
			})
		}
		return nil, nil
	}
	s.LastAttempts++
	if err := s.CastPipeline.Validate(ctx, cast, castSkill, now); err != nil {
		s.recordMiss(skillMissContext{
			Source:   source,
			Intent:   intent,
			Profile:  profile,
			Aim:      s.aimDirection(source, intent, resolver),
			Reason:   "cast_validation_failed:" + err.Error(),
			Tick:     tick,
			Resolver: resolver,
		})
		return nil, nil
	}
	if err := s.CastPipeline.Commit(ctx, cast, castSkill, now.Add(cooldown), now); err != nil {
		return nil, err
	}
	s.startActionTiming(source, skillID, intent, timing, cooldown, movementContract, hasMovementContract, now, tick)
	aim := s.aimDirection(source, intent, resolver)
	if playerSkillUsesPendingRuntimeWithContract(profile, hasMovementContract) {
		pending := s.startPendingPlayerSkillAction(ctx, source, profile, intent, aim, timing, movementContract, hasMovementContract, now, tick)
		beforeMovement := pending
		pending = s.applyPendingPlayerSkillMovement(region, source, index, resolver, pending, now, tick)
		s.syncPendingPlayerSkillState(source, pending, now)
		if pending.ResolveAt.After(now) {
			s.setPendingPlayerAction(source.RuntimeID(), pending)
			return nil, nil
		}
		if playerSkillPendingMovementChanged(beforeMovement, pending) {
			var err error
			index, err = s.spatialIndex(region, region.Entities().All())
			if err != nil {
				return nil, err
			}
			resolver = hitbox.EntityResolverFunc(func(id ids.RuntimeEntityID) (domainentity.Entity, bool) {
				return region.Entities().Get(id)
			})
		}
		outcomes, _, err := s.resolvePlayerSkillAction(ctx, region, source, pending, index, resolver, now, tick, true)
		return outcomes, err
	}
	resolveAt := now.Add(timing.ActiveStart)
	evaluation := hitbox.EvaluationContext{
		Caster:       source,
		Skill:        profile.Skill,
		InstanceID:   playerActionInstanceID(source, intent, ids.SkillID(profile.Skill.GetId()), tick),
		StartedAt:    now,
		Now:          resolveAt,
		Origin:       source.Position(),
		AimDirection: aim,
		Target:       intent.Target,
		HasTarget:    intent.HasTarget,
		Hitboxes:     profile.Hitboxes,
		Projectile:   profile.Projectile,
		Area:         profile.Area,
		Spatial:      index,
		Resolver:     resolver,
		LOS:          lineOfSightForRegion(region),
	}
	hits, err := s.Hitboxes.Evaluate(evaluation)
	if err != nil {
		return nil, err
	}
	if debug, err := hitbox.ActiveHitboxDebugs(evaluation); err == nil {
		s.LastHitboxDebugEvents = appendHitboxDebugEvents(s.LastHitboxDebugEvents, source, debug, tick)
	}
	if len(hits) == 0 {
		s.recordMiss(skillMissContext{
			Source:   source,
			Intent:   intent,
			Profile:  profile,
			Aim:      aim,
			Reason:   "no_hitbox_hit",
			Tick:     tick,
			Resolver: resolver,
		})
		return nil, nil
	}
	outcomes := make([]AttackOutcome, 0, len(hits))
	rejectedReason := ""
	for _, hit := range hits {
		target, ok := resolver.Resolve(hit.TargetID)
		if !ok || target == nil {
			rejectedReason = "target_unresolved"
			continue
		}
		result, err := s.Damage.Apply(ctx, DamageContext{
			Source:         source,
			Target:         target,
			Hit:            hit,
			Skill:          profile.Skill,
			Impact:         profile.Impact,
			ControlEffects: profile.ControlEffects,
			SourceCore:     profile.SourceCore,
			TargetCore:     profile.TargetCore,
			Defense:        profile.Defense,
			Now:            resolveAt,
			Tick:           tick,
			CurrentTick:    tick,
		})
		if errors.Is(err, ErrPvPRejected) || errors.Is(err, ErrInvalidTarget) {
			rejectedReason = err.Error()
			continue
		}
		if err != nil {
			return nil, err
		}
		if result.Reason == "" {
			result.Reason = combatOutcomeReason(result)
		}
		s.interruptCreatureTargetFromPlayerSkill(target, resolveAt, profile, result, "player_skill")
		result.SourceReaction, result.SourceReactionUntilMS = s.applySourceDefenseReaction(source, result, now)
		outcomes = append(outcomes, AttackOutcome{
			Result:                      result,
			Source:                      source.Ref(),
			Target:                      target.Ref(),
			RegionID:                    target.RegionID(),
			HitboxID:                    hit.HitboxID,
			SkillID:                     ids.SkillID(profile.Skill.GetId()),
			MotionProfileID:             hit.MotionProfileID,
			DamageGroupID:               hit.DamageGroupID,
			MotionTStart:                hit.MotionTStart,
			MotionTEnd:                  hit.MotionTEnd,
			MotionSampleStartIndex:      hit.MotionSampleStartIndex,
			MotionSampleEndIndex:        hit.MotionSampleEndIndex,
			HitQuality:                  hit.HitQuality,
			HitQualitySpatialScore:      hit.HitQualitySpatialScore,
			HitboxDebugShape:            hit.HitboxDebugShape,
			HitboxDebugCenter:           hit.HitboxDebugCenter,
			HitboxDebugExtent:           hit.HitboxDebugExtent,
			HitboxDebugForward:          hit.HitboxDebugForward,
			HitboxDebugRight:            hit.HitboxDebugRight,
			HitboxDebugUp:               hit.HitboxDebugUp,
			HitboxDebugSegmentA:         hit.HitboxDebugSegmentA,
			HitboxDebugSegmentB:         hit.HitboxDebugSegmentB,
			HitboxDebugSize:             hit.HitboxDebugSize,
			HitboxDebugRadius:           hit.HitboxDebugRadius,
			HitboxDebugLength:           hit.HitboxDebugLength,
			HitboxDebugHeight:           hit.HitboxDebugHeight,
			HitboxDebugMinAngleDeg:      hit.HitboxDebugMinAngleDeg,
			HitboxDebugMaxAngleDeg:      hit.HitboxDebugMaxAngleDeg,
			DamageType:                  profile.Skill.GetDamageType(),
			ElementalType:               profile.Skill.GetElementalType(),
			ImpactType:                  profile.Impact.GetImpactType(),
			TargetImpactResponseProfile: ImpactResponseProfileForEntity(target),
			Tick:                        tick,
			Killed:                      result.Killed,
		})
	}
	if len(outcomes) == 0 {
		if rejectedReason == "" {
			rejectedReason = "damage_rejected"
		}
		s.recordMiss(skillMissContext{
			Source:   source,
			Intent:   intent,
			Profile:  profile,
			Aim:      aim,
			Reason:   rejectedReason,
			Tick:     tick,
			Resolver: resolver,
		})
	}
	return outcomes, nil
}

func playerSkillUsesPendingRuntime(profile AttackProfile) bool {
	_, ok := skillMovementConfigForAttackProfile(profile)
	return ok
}

func playerSkillUsesPendingRuntimeWithContract(profile AttackProfile, hasMovementContract bool) bool {
	return hasMovementContract || playerSkillUsesPendingRuntime(profile)
}

func playerActionTimingForRuntime(profile AttackProfile, timing ActionTimingConfig) ActionTimingConfig {
	return playerActionTimingForRuntimeWithContract(profile, timing, movement.MovementActionContract{}, false)
}

func playerActionTimingForRuntimeWithContract(profile AttackProfile, timing ActionTimingConfig, contract movement.MovementActionContract, hasContract bool) ActionTimingConfig {
	cfg, ok := skillMovementConfigForAttackProfileOrContract(profile, contract, hasContract)
	if !ok {
		return timing
	}
	hitboxStart, hitboxEnd := hitboxActiveWindow(profile.Hitboxes)
	movementEnd := playerSkillMovementEndDelay(profile, timing, hitboxStart, hitboxEnd, cfg, contract, hasContract)
	hitboxSpan := hitboxEnd - hitboxStart
	if hitboxSpan <= 0 {
		hitboxSpan = 120 * time.Millisecond
	}
	if timing.ActiveStart <= 0 {
		timing.ActiveStart = skillMovementStart(skillMovementTimelineContext{
			Timing:      timing,
			HitboxStart: hitboxStart,
		}, cfg)
	}
	if timing.ActiveEnd < movementEnd+hitboxSpan {
		timing.ActiveEnd = movementEnd + hitboxSpan
	}
	if timing.ActionLock < timing.ActiveEnd+timing.Recovery {
		timing.ActionLock = timing.ActiveEnd + timing.Recovery
	}
	if !playerSkillUsesCooldown(profile) {
		timing.GlobalCooldown = 0
	} else if timing.GlobalCooldown < timing.ActionLock {
		timing.GlobalCooldown = timing.ActionLock
	}
	return timing
}

func pendingPlayerSkillMovementConfig(pending pendingPlayerSkillAction) (skillMovementConfig, bool) {
	return skillMovementConfigForAttackProfileOrContract(pending.Profile, pending.MovementContract, pending.HasMovementContract)
}

func playerSkillMovementEndDelay(profile AttackProfile, timing ActionTimingConfig, hitboxStart time.Duration, hitboxEnd time.Duration, cfg skillMovementConfig, contract movement.MovementActionContract, hasContract bool) time.Duration {
	movementStart := skillMovementStart(skillMovementTimelineContext{
		Timing:              timing,
		HitboxStart:         hitboxStart,
		HasMovementContract: hasContract,
		MovementContract:    contract,
	}, cfg)
	if hasContract && playerSkillMovementContractUsesForwardDistance(contract) {
		movementStart = time.Duration(maxInt32(contract.StartupMS, 0)) * time.Millisecond
	}
	duration := time.Duration(cfg.DurationMS) * time.Millisecond
	if hasContract {
		if contract.ActiveMS > 0 {
			duration = time.Duration(contract.ActiveMS) * time.Millisecond
		} else if contract.DurationMS > 0 {
			duration = time.Duration(contract.DurationMS) * time.Millisecond
		}
	}
	if duration <= 0 && timing.ActiveEnd > movementStart {
		duration = timing.ActiveEnd - movementStart
	}
	if duration <= 0 {
		duration = 200 * time.Millisecond
	}
	return movementStart + duration
}

func (s *RegionPlayerSkillCombatSystem) movementContractForSkill(ctx context.Context, source domainentity.Entity, skillID ids.SkillID) (movement.MovementActionContract, bool) {
	if s == nil || s.MovementContracts == nil || source == nil || skillID == "" {
		return movement.MovementActionContract{}, false
	}
	contract, ok := s.MovementContracts.MovementActionContract(ctx, source, skillID)
	if !ok || !contract.Enabled {
		return movement.MovementActionContract{}, false
	}
	return contract, true
}

func (s *RegionPlayerSkillCombatSystem) startPendingPlayerSkillAction(ctx context.Context, source domainentity.Entity, profile AttackProfile, intent skill.Intent, aim domainmath.Vec3, timing ActionTimingConfig, movementContract movement.MovementActionContract, hasMovementContract bool, now time.Time, tick uint64) pendingPlayerSkillAction {
	hitboxStart, hitboxEnd := hitboxActiveWindow(profile.Hitboxes)
	if hitboxEnd <= hitboxStart {
		hitboxStart = timing.ActiveStart
		hitboxEnd = timing.ActiveEnd
	}
	_ = ctx
	resolveDelay := timing.ActiveStart
	if cfg, ok := skillMovementConfigForAttackProfileOrContract(profile, movementContract, hasMovementContract); ok {
		if movementEnd := playerSkillMovementEndDelay(profile, timing, hitboxStart, hitboxEnd, cfg, movementContract, hasMovementContract); movementEnd > resolveDelay {
			resolveDelay = movementEnd
		}
	}
	if resolveDelay < 0 {
		resolveDelay = 0
	}
	pending := pendingPlayerSkillAction{
		Profile:             profile,
		Intent:              intent,
		StartedAt:           now,
		ResolveAt:           now.Add(resolveDelay),
		HitboxStart:         hitboxStart,
		HitboxEnd:           hitboxEnd,
		Timing:              timing,
		Tick:                tick,
		InstanceID:          playerActionInstanceID(source, intent, ids.SkillID(profile.Skill.GetId()), tick),
		ActionStartPosition: source.Position(),
	}
	if !aim.IsZero() {
		pending.CommittedMoveDirection = aim.Normalize()
	}
	if hasMovementContract {
		pending.MovementContract = movementContract
		pending.HasMovementContract = true
	}
	return pending
}

func (s *RegionPlayerSkillCombatSystem) resolvePendingSkill(ctx context.Context, region *regionruntime.RegionRuntime, source domainentity.Entity, index spatial.SpatialIndex, resolver hitbox.EntityResolver, now time.Time, tick uint64) ([]AttackOutcome, bool, error) {
	if s == nil || source == nil {
		return nil, false, nil
	}
	entityID := source.RuntimeID()
	pending, ok := s.pendingPlayerAction(entityID)
	if !ok {
		return nil, false, nil
	}
	if pending.ImpactResolved {
		pending = s.applyPendingPlayerSkillMovement(region, source, index, resolver, pending, now, tick)
		if pending.ReleaseStartedAt.IsZero() || pending.ContactTargetReleased {
			if cfg, ok := pendingPlayerSkillMovementConfig(pending); ok && pendingPlayerSkillMovementTimelineActive(pending, cfg, now) {
				s.setPendingPlayerAction(entityID, pending)
				s.syncPendingPlayerSkillState(source, pending, now)
				return nil, false, nil
			}
			s.clearPendingPlayerAction(entityID)
			s.syncPendingPlayerSkillState(source, pending, now)
			s.prunePlayerActionState(entityID, now)
			return nil, true, nil
		}
		s.syncPendingPlayerSkillState(source, pending, now)
		s.setPendingPlayerAction(entityID, pending)
		return nil, false, nil
	}
	s.syncPendingPlayerSkillState(source, pending, now)
	activeAt, inactiveAt := pendingPlayerHitboxWindow(pending)
	if now.Before(activeAt) {
		pending = s.applyPendingPlayerSkillMovement(region, source, index, resolver, pending, now, tick)
		s.syncPendingPlayerSkillState(source, pending, now)
		s.setPendingPlayerAction(entityID, pending)
		return nil, false, nil
	}

	evaluationTime := now
	if now.After(inactiveAt) {
		evaluationTime = inactiveAt
	}
	beforeMovement := pending
	evaluationTick := pendingPlayerEvaluationTick(pending, evaluationTime, tick)
	pending = s.applyPendingPlayerSkillMovement(region, source, index, resolver, pending, evaluationTime, evaluationTick)
	if playerSkillPendingMovementChanged(beforeMovement, pending) {
		var err error
		index, err = s.spatialIndex(region, region.Entities().All())
		if err != nil {
			return nil, true, err
		}
		resolver = hitbox.EntityResolverFunc(func(id ids.RuntimeEntityID) (domainentity.Entity, bool) {
			return region.Entities().Get(id)
		})
	}
	outcomes, hit, err := s.resolvePlayerSkillAction(ctx, region, source, pending, index, resolver, evaluationTime, evaluationTick, !now.Before(inactiveAt))
	if err != nil {
		return outcomes, true, err
	}
	pending.LastHitboxEvaluationAt = evaluationTime
	pending.LastHitboxEvaluationOrigin = source.Position()
	if hit || !now.Before(inactiveAt) {
		pending.ImpactResolved = true
		if hit && !pending.ReleaseStartedAt.IsZero() && !pending.ContactTargetReleased {
			s.setPendingPlayerAction(entityID, pending)
			s.syncPendingPlayerSkillState(source, pending, evaluationTime)
			return outcomes, true, nil
		}
		if cfg, ok := pendingPlayerSkillMovementConfig(pending); ok && pendingPlayerSkillMovementTimelineActive(pending, cfg, evaluationTime) {
			s.setPendingPlayerAction(entityID, pending)
			s.syncPendingPlayerSkillState(source, pending, evaluationTime)
			return outcomes, true, nil
		}
		s.clearPendingPlayerAction(entityID)
		s.syncPendingPlayerSkillState(source, pending, evaluationTime)
		s.prunePlayerActionState(entityID, evaluationTime)
		return outcomes, true, nil
	}
	s.syncPendingPlayerSkillState(source, pending, evaluationTime)
	s.setPendingPlayerAction(entityID, pending)
	return nil, false, nil
}

func pendingPlayerHitboxWindow(pending pendingPlayerSkillAction) (time.Time, time.Time) {
	start := pending.StartedAt.Add(pending.HitboxStart)
	end := pending.StartedAt.Add(pending.HitboxEnd)
	if _, ok := pendingPlayerSkillMovementConfig(pending); ok {
		span := pending.HitboxEnd - pending.HitboxStart
		if span <= 0 {
			span = 120 * time.Millisecond
		}
		start = pending.ResolveAt
		end = pending.ResolveAt.Add(span)
	}
	if end.Before(start) {
		end = start
	}
	return start, end
}

func pendingPlayerEvaluationStartedAt(pending pendingPlayerSkillAction) time.Time {
	if _, ok := pendingPlayerSkillMovementConfig(pending); ok {
		return pending.ResolveAt.Add(-pending.HitboxStart)
	}
	return pending.StartedAt
}

func pendingPlayerEvaluationTick(pending pendingPlayerSkillAction, evaluationTime time.Time, fallback uint64) uint64 {
	if pending.Tick == 0 || pending.StartedAt.IsZero() || evaluationTime.Before(pending.StartedAt) {
		return fallback
	}
	offset := uint64(evaluationTime.Sub(pending.StartedAt).Seconds()*30.0 + 0.5)
	return pending.Tick + offset
}

func (s *RegionPlayerSkillCombatSystem) IncomingThreats(now time.Time) []PlayerIncomingThreatSnapshot {
	if s == nil || s.Regions == nil {
		return nil
	}
	if now.IsZero() {
		now = time.Now()
	}
	s.ensurePlayerActionStates()
	sources := make(map[ids.RuntimeEntityID]domainentity.Entity)
	for _, region := range s.Regions.Active() {
		if region == nil || region.Entities() == nil {
			continue
		}
		for _, entity := range region.Entities().All() {
			if entity != nil && entity.EntityType() == domainentity.EntityTypePlayer {
				sources[entity.RuntimeID()] = entity
			}
		}
	}
	out := make([]PlayerIncomingThreatSnapshot, 0, len(s.actionStates))
	for sourceID, state := range s.actionStates {
		source := sources[sourceID]
		if source == nil {
			continue
		}
		if state.HasExecution {
			snapshot, ok := playerPendingActionThreatSnapshot(source, state.Execution, now)
			if ok {
				out = append(out, snapshot)
				continue
			}
		}
		snapshot, ok := playerActiveActionThreatSnapshot(source, state, now)
		if ok {
			out = append(out, snapshot)
		}
	}
	return out
}

func playerActiveActionThreatSnapshot(source domainentity.Entity, state playerActionRuntimeState, now time.Time) (PlayerIncomingThreatSnapshot, bool) {
	instance := state.Instance
	if source == nil || instance.SkillID == "" || instance.StartedAt.IsZero() {
		return PlayerIncomingThreatSnapshot{}, false
	}
	timing := actionTimingConfigFromRuntime(instance.Timing)
	phase, ok := playerActionThreatPhase(timing, now.Sub(instance.StartedAt))
	if !ok {
		return PlayerIncomingThreatSnapshot{}, false
	}
	return PlayerIncomingThreatSnapshot{
		SourceID:    source.RuntimeID(),
		RegionID:    source.RegionID(),
		SkillID:     instance.SkillID,
		Phase:       phase,
		Position:    source.Position(),
		Forward:     sourceFacingDirection(source),
		RangeCM:     firstPositiveFloat(state.RangeCM, 180),
		ThreatScore: playerActionThreatScore(phase),
		StartedAt:   instance.StartedAt,
		ActiveAt:    instance.StartedAt.Add(timing.ActiveStart),
		ExpiresAt:   now.Add(120 * time.Millisecond),
		Tick:        instance.ServerActionSequence,
	}, true
}

func playerPendingActionThreatSnapshot(source domainentity.Entity, pending pendingPlayerSkillAction, now time.Time) (PlayerIncomingThreatSnapshot, bool) {
	if source == nil || pending.StartedAt.IsZero() {
		return PlayerIncomingThreatSnapshot{}, false
	}
	phase, ok := playerActionThreatPhase(pending.Timing, now.Sub(pending.StartedAt))
	if !ok {
		return PlayerIncomingThreatSnapshot{}, false
	}
	forward := pending.CommittedMoveDirection
	if forward.IsZero() {
		forward = sourceFacingDirection(source)
	}
	rangeCM := playerActionThreatRange(pending.Profile, pending.Timing)
	if cfg, cfgOK := pendingPlayerSkillMovementConfig(pending); cfgOK {
		rangeCM = firstPositiveFloat(rangeCM, pendingPlayerSkillMovementDistance(pending, cfg)+180)
	}
	return PlayerIncomingThreatSnapshot{
		SourceID:     source.RuntimeID(),
		RegionID:     source.RegionID(),
		SkillID:      ids.SkillID(pendingPlayerSkillID(pending)),
		Phase:        phase,
		Position:     source.Position(),
		Forward:      forward,
		RangeCM:      firstPositiveFloat(rangeCM, 180),
		ThreatScore:  playerActionThreatScore(phase),
		StartedAt:    pending.StartedAt,
		ActiveAt:     pending.StartedAt.Add(pending.Timing.ActiveStart),
		ExpiresAt:    now.Add(120 * time.Millisecond),
		Tick:         pending.Tick,
		TargetLocked: pending.Intent.HasTarget || !pending.CommittedTargetPosition.IsZero(),
	}, true
}

func playerActionThreatPhase(timing ActionTimingConfig, elapsed time.Duration) (string, bool) {
	if elapsed < 0 {
		return "", false
	}
	switch {
	case timing.ActiveStart > 0 && elapsed < timing.ActiveStart:
		return "windup", true
	case elapsed < timing.ActiveEnd:
		return "active", true
	case timing.ActionLock > 0 && elapsed < timing.ActionLock:
		return "recovery", true
	default:
		return "", false
	}
}

func playerActionThreatScore(phase string) float64 {
	switch phase {
	case "active":
		return 1
	case "windup":
		return 0.78
	case "recovery":
		return 0.22
	default:
		return 0
	}
}

func playerActionThreatRange(profile AttackProfile, timing ActionTimingConfig) float64 {
	rangeCM := 0.0
	if profile.Skill != nil {
		rangeCM = firstPositiveFloat(profile.Skill.GetMaxRange(), profile.Skill.GetMovementDistance(), 0)
	}
	for _, hitbox := range profile.Hitboxes {
		if hitbox == nil {
			continue
		}
		rangeCM = math.Max(rangeCM, hitbox.GetRadius())
		rangeCM = math.Max(rangeCM, hitbox.GetLength())
		rangeCM = math.Max(rangeCM, hitbox.GetSizeX())
		rangeCM = math.Max(rangeCM, hitbox.GetSizeY())
	}
	if rangeCM <= 0 && timing.ActiveEnd > timing.ActiveStart {
		rangeCM = 180
	}
	return firstPositiveFloat(rangeCM, 180)
}

func playerSkillPendingMovementChanged(before pendingPlayerSkillAction, after pendingPlayerSkillAction) bool {
	return math.Abs(after.AppliedMovementDistance-before.AppliedMovementDistance) > domainmath.Epsilon ||
		math.Abs(after.AppliedTargetPushDistance-before.AppliedTargetPushDistance) > domainmath.Epsilon ||
		math.Abs(after.ReleaseAppliedDistance-before.ReleaseAppliedDistance) > domainmath.Epsilon ||
		after.ContactTargetReleased != before.ContactTargetReleased
}

func pendingPlayerSkillMovementTimeline(pending pendingPlayerSkillAction, cfg skillMovementConfig) (time.Duration, time.Duration, time.Duration) {
	movementStart := pendingPlayerSkillMovementStart(pending, cfg)
	movementDuration := pendingPlayerSkillMovementDuration(pending, cfg, movementStart)
	actionDuration := pendingPlayerSkillActionDuration(pending, movementStart, movementDuration)
	return movementStart, movementDuration, actionDuration
}

func pendingPlayerSkillMovementTimelineActive(pending pendingPlayerSkillAction, cfg skillMovementConfig, now time.Time) bool {
	if pending.StartedAt.IsZero() || now.IsZero() {
		return false
	}
	_, _, actionDuration := pendingPlayerSkillMovementTimeline(pending, cfg)
	if actionDuration <= 0 {
		return false
	}
	elapsed := now.Sub(pending.StartedAt)
	return elapsed >= 0 && elapsed <= actionDuration
}

func (s *RegionPlayerSkillCombatSystem) ActiveMovementIntent(ctx context.Context, region *regionruntime.RegionRuntime, source domainentity.Entity, index spatial.SpatialIndex, now time.Time, delta time.Duration, tick uint64) (movement.Intent, bool) {
	if s == nil || source == nil || region == nil || region.Entities() == nil || now.IsZero() || delta <= 0 {
		return movement.Intent{}, false
	}
	entityID := source.RuntimeID()
	pending, ok := s.pendingPlayerAction(entityID)
	if !ok || pending.StartedAt.IsZero() {
		return movement.Intent{}, false
	}
	cfg, ok := pendingPlayerSkillMovementConfig(pending)
	if !ok {
		return movement.Intent{}, false
	}
	resolver := hitbox.EntityResolverFunc(func(id ids.RuntimeEntityID) (domainentity.Entity, bool) {
		return region.Entities().Get(id)
	})
	movementStart, duration, actionDuration := pendingPlayerSkillMovementTimeline(pending, cfg)
	actionElapsed := now.Sub(pending.StartedAt)
	if actionElapsed < 0 || (actionDuration > 0 && actionElapsed > actionDuration) {
		return movement.Intent{}, false
	}
	if !pending.MovementTargetCommitted {
		pending = commitPendingPlayerSkillMovementTarget(source, resolver, pending, cfg)
		s.setPendingPlayerAction(entityID, pending)
		s.syncPendingPlayerSkillState(source, pending, now)
	}
	landingTarget := pending.CommittedTargetPosition
	direction := pending.CommittedMoveDirection
	direction.Z = 0
	if direction.IsZero() && !landingTarget.IsZero() {
		direction = domainmath.Direction(source.Position(), landingTarget)
		direction.Z = 0
	}
	if direction.IsZero() {
		direction = sourceFacingDirection(source)
		direction.Z = 0
	}
	if direction.IsZero() {
		return movement.Intent{}, false
	}
	direction = direction.Normalize()

	totalDistance := pendingPlayerSkillMovementDistance(pending, cfg)
	zeroIntent := pendingPlayerSkillMovementIntent(source, pending, cfg, direction, 0, 0, movementStart, duration, totalDistance, now)
	if !pending.ReleaseStartedAt.IsZero() {
		return zeroIntent, true
	}
	elapsed := actionElapsed - movementStart
	if elapsed <= 0 {
		return zeroIntent, true
	}
	if totalDistance <= 0 || pending.AppliedMovementDistance >= totalDistance-domainmath.Epsilon {
		return zeroIntent, true
	}
	if elapsed >= duration {
		if pending.ContactTargetID.Valid() && pending.ReleaseStartedAt.IsZero() {
			pending = s.beginPendingPlayerSkillContactRelease(source, resolver, pending, cfg, direction, now, tick)
			s.setPendingPlayerAction(entityID, pending)
			s.syncPendingPlayerSkillState(source, pending, now)
		}
		return zeroIntent, true
	}
	targetApplied := pendingPlayerSkillMovementDistanceAtElapsed(pending, cfg, elapsed, duration, totalDistance)
	travel := targetApplied - pending.AppliedMovementDistance
	if travel <= domainmath.Epsilon {
		return zeroIntent, true
	}
	sourceTravel := travel
	distance := source.Position().Distance(landingTarget)
	stopDistance := playerSkillMovementStopDistance(source, resolver, pending, cfg)
	contactTarget, hasContactTarget := s.resolvePendingPlayerSkillContactTarget(source, index, resolver, pending, cfg, direction, sourceTravel)
	if hasContactTarget && contactTarget != nil && !cfg.CanPhaseThroughTargets {
		pending.ContactTargetID = contactTarget.RuntimeID()
		distance = source.Position().Distance(contactTarget.Position())
		stopDistance = playerSkillMovementContactStopDistance(pending.Profile, source, contactTarget, cfg)
		s.setPendingPlayerAction(entityID, pending)
	}
	if hasContactTarget && contactTarget != nil && !cfg.CanPhaseThroughTargets && cfg.AppliesKnockback {
		maxTravel := distance - stopDistance
		if maxTravel < 0 {
			maxTravel = 0
		}
		if blockedTravel := sourceTravel - maxTravel; blockedTravel > domainmath.Epsilon {
			pending = s.applyPendingPlayerSkillIntegratedContactEffects(region, source, resolver, pending, cfg, direction, blockedTravel, now, tick)
			pending.PreMovementContactPushTick = tick
			s.setPendingPlayerAction(entityID, pending)
			s.syncPendingPlayerSkillState(source, pending, now)
			if index != nil {
				_ = index.Update(spatial.SpatialObjectFromEntity(contactTarget))
			}
			if !pending.ReleaseStartedAt.IsZero() {
				return zeroIntent, true
			}
			distance = source.Position().Distance(contactTarget.Position())
			stopDistance = playerSkillMovementContactStopDistance(pending.Profile, source, contactTarget, cfg)
		}
	}
	if maxTravel := distance - stopDistance; maxTravel <= 0 {
		sourceTravel = 0
	} else if sourceTravel > maxTravel {
		sourceTravel = maxTravel
	}
	if sourceTravel <= domainmath.Epsilon {
		if hasContactTarget && contactTarget != nil {
			pending = s.applyPendingPlayerSkillIntegratedContactEffects(region, source, resolver, pending, cfg, direction, travel, now, tick)
			s.setPendingPlayerAction(entityID, pending)
			s.syncPendingPlayerSkillState(source, pending, now)
		}
		return zeroIntent, true
	}
	force, speedScale := pendingPlayerSkillMovementIntentForceAndScale(pending, cfg, sourceTravel, duration, totalDistance, delta)
	intent := pendingPlayerSkillMovementIntent(source, pending, cfg, direction, force, speedScale, movementStart, duration, totalDistance, now)
	return intent, true
}

func (s *RegionPlayerSkillCombatSystem) CommitMovementResult(ctx context.Context, region *regionruntime.RegionRuntime, source domainentity.Entity, intent movement.Intent, result movement.Result, before domainmath.Position, now time.Time, tick uint64) {
	if s == nil || source == nil || region == nil || region.Entities() == nil {
		return
	}
	entityID := source.RuntimeID()
	pending, ok := s.pendingPlayerAction(entityID)
	if !ok {
		return
	}
	cfg, ok := pendingPlayerSkillMovementConfig(pending)
	if !ok {
		return
	}
	direction := intent.Direction
	direction.Z = 0
	if direction.IsZero() {
		return
	}
	direction = direction.Normalize()
	applied := source.Position().Sub(before)
	applied.Z = 0
	projected := applied.Dot(direction)
	appliedTravel := 0.0
	if projected > domainmath.Epsilon {
		totalDistance := pendingPlayerSkillMovementDistance(pending, cfg)
		remaining := totalDistance - pending.AppliedMovementDistance
		if remaining > 0 && projected > remaining {
			projected = remaining
		}
		if projected > domainmath.Epsilon {
			pending.AppliedMovementDistance += projected
			appliedTravel = projected
		}
	}
	totalDistance := pendingPlayerSkillMovementDistance(pending, cfg)
	resolver := hitbox.EntityResolverFunc(func(id ids.RuntimeEntityID) (domainentity.Entity, bool) {
		return region.Entities().Get(id)
	})
	contactTravel := appliedTravel
	if pending.PreMovementContactPushTick == tick {
		contactTravel = 0
	}
	if contactTravel <= domainmath.Epsilon && pending.PreMovementContactPushTick != tick && pending.ContactTargetID.Valid() && intent.Force > 0 {
		movementStart := pendingPlayerSkillMovementStart(pending, cfg)
		movementDuration := pendingPlayerSkillMovementDuration(pending, cfg, movementStart)
		elapsed := now.Sub(pending.StartedAt) - movementStart
		targetApplied := pendingPlayerSkillMovementDistanceAtElapsed(pending, cfg, elapsed, movementDuration, totalDistance)
		if remainingTravel := targetApplied - pending.AppliedMovementDistance; remainingTravel > contactTravel {
			contactTravel = remainingTravel
		}
	}
	pending = s.applyPendingPlayerSkillIntegratedContactEffects(region, source, resolver, pending, cfg, direction, contactTravel, now, tick)
	if totalDistance > 0 && pending.AppliedMovementDistance >= totalDistance-domainmath.Epsilon {
		pending.AppliedMovementDistance = totalDistance
		pending = s.beginPendingPlayerSkillContactRelease(source, resolver, pending, cfg, direction, now, tick)
	}
	s.setPendingPlayerAction(entityID, pending)
	s.syncPendingPlayerSkillState(source, pending, now)
	_ = ctx
	_ = result
}

func (s *RegionPlayerSkillCombatSystem) resolvePlayerSkillAction(ctx context.Context, region *regionruntime.RegionRuntime, source domainentity.Entity, pending pendingPlayerSkillAction, index spatial.SpatialIndex, resolver hitbox.EntityResolver, now time.Time, tick uint64, recordMiss bool) ([]AttackOutcome, bool, error) {
	profile := pending.Profile
	aim := pending.CommittedMoveDirection
	if aim.IsZero() {
		aim = s.aimDirection(source, pending.Intent, resolver)
	}
	evaluation := hitbox.EvaluationContext{
		Caster:         source,
		Skill:          profile.Skill,
		InstanceID:     pending.InstanceID,
		StartedAt:      pendingPlayerEvaluationStartedAt(pending),
		PreviousNow:    pending.LastHitboxEvaluationAt,
		PreviousOrigin: pending.LastHitboxEvaluationOrigin,
		Now:            now,
		Origin:         source.Position(),
		AimDirection:   aim,
		Target:         pending.Intent.Target,
		HasTarget:      pending.Intent.HasTarget,
		Hitboxes:       profile.Hitboxes,
		Projectile:     profile.Projectile,
		Area:           profile.Area,
		Spatial:        index,
		Resolver:       resolver,
		LOS:            lineOfSightForRegion(region),
	}
	hits, err := s.Hitboxes.Evaluate(evaluation)
	if err != nil {
		return nil, false, err
	}
	if debug, err := hitbox.ActiveHitboxDebugs(evaluation); err == nil {
		s.LastHitboxDebugEvents = appendHitboxDebugEvents(s.LastHitboxDebugEvents, source, debug, tick)
	}
	outcomes := make([]AttackOutcome, 0, len(hits))
	rejectedReason := ""
	for _, hit := range hits {
		target, ok := resolver.Resolve(hit.TargetID)
		if !ok || target == nil {
			rejectedReason = "target_unresolved"
			continue
		}
		result, err := s.Damage.Apply(ctx, DamageContext{
			Source:         source,
			Target:         target,
			Hit:            hit,
			Skill:          profile.Skill,
			Impact:         profile.Impact,
			ControlEffects: profile.ControlEffects,
			SourceCore:     profile.SourceCore,
			TargetCore:     profile.TargetCore,
			Defense:        profile.Defense,
			Now:            now,
			Tick:           tick,
			CurrentTick:    tick,
		})
		if errors.Is(err, ErrPvPRejected) || errors.Is(err, ErrInvalidTarget) {
			rejectedReason = err.Error()
			continue
		}
		if err != nil {
			return nil, false, err
		}
		if result.Reason == "" {
			result.Reason = combatOutcomeReason(result)
		}
		s.interruptCreatureTargetFromPlayerSkill(target, now, profile, result, "player_skill")
		result.SourceReaction, result.SourceReactionUntilMS = s.applySourceDefenseReaction(source, result, now)
		outcomes = append(outcomes, AttackOutcome{
			Result:                      result,
			Source:                      source.Ref(),
			Target:                      target.Ref(),
			RegionID:                    target.RegionID(),
			HitboxID:                    hit.HitboxID,
			SkillID:                     ids.SkillID(profile.Skill.GetId()),
			MotionProfileID:             hit.MotionProfileID,
			DamageGroupID:               hit.DamageGroupID,
			MotionTStart:                hit.MotionTStart,
			MotionTEnd:                  hit.MotionTEnd,
			MotionSampleStartIndex:      hit.MotionSampleStartIndex,
			MotionSampleEndIndex:        hit.MotionSampleEndIndex,
			HitQuality:                  hit.HitQuality,
			HitQualitySpatialScore:      hit.HitQualitySpatialScore,
			HitboxDebugShape:            hit.HitboxDebugShape,
			HitboxDebugCenter:           hit.HitboxDebugCenter,
			HitboxDebugExtent:           hit.HitboxDebugExtent,
			HitboxDebugForward:          hit.HitboxDebugForward,
			HitboxDebugRight:            hit.HitboxDebugRight,
			HitboxDebugUp:               hit.HitboxDebugUp,
			HitboxDebugSegmentA:         hit.HitboxDebugSegmentA,
			HitboxDebugSegmentB:         hit.HitboxDebugSegmentB,
			HitboxDebugSize:             hit.HitboxDebugSize,
			HitboxDebugRadius:           hit.HitboxDebugRadius,
			HitboxDebugLength:           hit.HitboxDebugLength,
			HitboxDebugHeight:           hit.HitboxDebugHeight,
			HitboxDebugMinAngleDeg:      hit.HitboxDebugMinAngleDeg,
			HitboxDebugMaxAngleDeg:      hit.HitboxDebugMaxAngleDeg,
			DamageType:                  profile.Skill.GetDamageType(),
			ElementalType:               profile.Skill.GetElementalType(),
			ImpactType:                  profile.Impact.GetImpactType(),
			TargetImpactResponseProfile: ImpactResponseProfileForEntity(target),
			Tick:                        tick,
			Killed:                      result.Killed,
		})
	}
	if len(outcomes) > 0 {
		return outcomes, true, nil
	}
	if recordMiss {
		if rejectedReason == "" {
			rejectedReason = "no_hitbox_hit"
		}
		s.recordMiss(skillMissContext{
			Source:   source,
			Intent:   pending.Intent,
			Profile:  profile,
			Aim:      aim,
			Reason:   rejectedReason,
			Tick:     tick,
			Resolver: resolver,
		})
	}
	return nil, false, nil
}

func (s *RegionPlayerSkillCombatSystem) applyPendingPlayerSkillMovement(region *regionruntime.RegionRuntime, source domainentity.Entity, index spatial.SpatialIndex, resolver hitbox.EntityResolver, pending pendingPlayerSkillAction, now time.Time, tick uint64) pendingPlayerSkillAction {
	cfg, ok := pendingPlayerSkillMovementConfig(pending)
	if source == nil || !ok || now.IsZero() || pending.StartedAt.IsZero() {
		return pending
	}
	_ = index
	return s.applyPendingPlayerSkillMovementCombatSide(region, source, resolver, pending, cfg, now, tick)
}

func (s *RegionPlayerSkillCombatSystem) applyPendingPlayerSkillMovementCombatSide(region *regionruntime.RegionRuntime, source domainentity.Entity, resolver hitbox.EntityResolver, pending pendingPlayerSkillAction, cfg skillMovementConfig, now time.Time, tick uint64) pendingPlayerSkillAction {
	movementStart := pendingPlayerSkillMovementStart(pending, cfg)
	elapsed := now.Sub(pending.StartedAt) - movementStart
	if elapsed < 0 {
		return pending
	}
	if !pending.MovementTargetCommitted {
		pending = commitPendingPlayerSkillMovementTarget(source, resolver, pending, cfg)
	}
	if !pending.ReleaseStartedAt.IsZero() && !pending.ContactTargetReleased {
		return s.applyPendingPlayerSkillContactRelease(region, resolver, pending, now, tick)
	}
	totalDistance := pendingPlayerSkillMovementDistance(pending, cfg)
	if totalDistance <= 0 {
		return pending
	}
	if pending.AppliedMovementDistance < totalDistance-domainmath.Epsilon {
		return pending
	}
	direction := pending.CommittedMoveDirection
	direction.Z = 0
	if direction.IsZero() && !pending.CommittedTargetPosition.IsZero() {
		direction = domainmath.Direction(source.Position(), pending.CommittedTargetPosition)
		direction.Z = 0
	}
	if !direction.IsZero() {
		direction = direction.Normalize()
		pending = s.beginPendingPlayerSkillContactRelease(source, resolver, pending, cfg, direction, now, tick)
	}
	if !pending.ReleaseStartedAt.IsZero() && !pending.ContactTargetReleased {
		pending = s.applyPendingPlayerSkillContactRelease(region, resolver, pending, now, tick)
	}
	return pending
}

func pendingPlayerSkillMovementIntent(source domainentity.Entity, pending pendingPlayerSkillAction, cfg skillMovementConfig, direction domainmath.Vec3, force float64, speedScale float64, movementStart time.Duration, movementDuration time.Duration, totalDistance float64, now time.Time) movement.Intent {
	skillID := pendingPlayerSkillID(pending)
	if skillID == "" && pending.Intent.SkillID != "" {
		skillID = pending.Intent.SkillID.String()
	}
	actionDuration := pendingPlayerSkillActionDuration(pending, movementStart, movementDuration)
	recoveryDuration := actionDuration - movementStart - movementDuration
	if recoveryDuration < 0 {
		recoveryDuration = 0
	}
	if speedScale < 0 {
		speedScale = 0
	}
	effectID := ""
	if pending.Profile.Movement != nil {
		effectID = pending.Profile.Movement.GetId()
	}
	startedAt := pending.StartedAt
	if startedAt.IsZero() {
		startedAt = now
	}
	receivedAt := pending.Intent.ReceivedAt
	if receivedAt.IsZero() {
		receivedAt = startedAt
	}
	return movement.NewGroundedSkillMovementIntent(movement.GroundedSkillMovementIntentSpec{
		EntityID:                  source.RuntimeID(),
		CommandID:                 pending.Intent.CommandID,
		Sequence:                  pending.Intent.Sequence,
		ClientActionSequence:      pending.Intent.Sequence,
		ClientTick:                pending.Intent.ClientTick,
		ServerReceivedTick:        pending.Tick,
		ServerActionStartedTick:   pending.Tick,
		ClientPredictedActionTick: pending.Intent.ClientTick,
		SkillID:                   skillID,
		AbilityKey:                skillID,
		ActionFamily:              "combat",
		MovementType:              cfg.MovementType,
		ActionPriority:            45,
		Direction:                 direction,
		Force:                     force,
		SpeedScale:                speedScale,
		ActionStartPosition:       pending.ActionStartPosition,
		HasActionStartPosition:    true,
		ActionDuration:            actionDuration,
		StartupDuration:           movementStart,
		ActiveDuration:            movementDuration,
		RecoveryDuration:          recoveryDuration,
		Distance:                  totalDistance,
		SpeedCurveID:              cfg.ArcCurve,
		SpeedCurveSamples:         skillMovementIntentCurveSamples(cfg),
		PredictionErrorPolicy:     "correction_debt",
		TimelineClassification:    "server_skill_movement",
		Effect: movement.SkillMovementEffect{
			ID:                     effectID,
			SkillID:                skillID,
			MovementType:           cfg.MovementType,
			Distance:               totalDistance,
			Speed:                  force,
			DurationMS:             int32(math.Round(movementDuration.Seconds() * 1000)),
			DesiredLandingDistance: cfg.DesiredLandingDistance,
			MinLandingDistance:     cfg.MinLandingDistance,
			StopAtContactRatio:     cfg.StopAtContactRatio,
			ArcHeight:              cfg.ArcHeight,
			ArcCurve:               cfg.ArcCurve,
			TakeoffMS:              cfg.TakeoffMS,
			LandingLockMS:          cfg.LandingLockMS,
			MovementStartPhase:     cfg.MovementStartPhase,
			MovementStartOffsetMS:  cfg.MovementStartOffsetMS,
			SteeringPolicy:         cfg.SteeringPolicy,
			MaxTurnDegPerSec:       cfg.MaxTurnDegPerSec,
			MaxTotalRedirectAngle:  cfg.MaxTotalRedirectAngle,
			RedirectLockoutMS:      cfg.RedirectLockoutMS,
			CanPhaseThroughTargets: cfg.CanPhaseThroughTargets,
			AppliesKnockback:       cfg.AppliesKnockback,
			KnockbackDistance:      cfg.KnockbackDistance,
			KnockbackSpeed:         cfg.KnockbackSpeed,
			RespectsNavMesh:        true,
		},
		Contract:    pending.MovementContract,
		HasContract: pending.HasMovementContract,
		ReceivedAt:  receivedAt,
		StoredAt:    startedAt,
	})
}

func (s *RegionPlayerSkillCombatSystem) applyPendingPlayerSkillIntegratedContactEffects(region *regionruntime.RegionRuntime, source domainentity.Entity, resolver hitbox.EntityResolver, pending pendingPlayerSkillAction, cfg skillMovementConfig, direction domainmath.Vec3, travel float64, now time.Time, tick uint64) pendingPlayerSkillAction {
	if s == nil || source == nil || resolver == nil {
		return pending
	}
	multiTarget := playerSkillContactAllowsMultiTarget(pending.Profile)
	var contactIndex spatial.SpatialIndex
	needsContactScan := multiTarget || (!pending.ContactTargetID.Valid() && !pending.Intent.HasTarget)
	if needsContactScan && region != nil && region.Entities() != nil {
		if index, err := s.spatialIndex(region, region.Entities().All()); err == nil {
			contactIndex = index
		}
	}
	targets := s.resolvePendingPlayerSkillContactTargets(source, contactIndex, resolver, pending, cfg, direction, travel)
	if len(targets) == 0 {
		return pending
	}
	direction.Z = 0
	if direction.IsZero() {
		direction = pending.CommittedMoveDirection
		direction.Z = 0
	}
	if direction.IsZero() && len(targets) > 0 && targets[0] != nil {
		direction = domainmath.Direction(source.Position(), targets[0].Position())
		direction.Z = 0
	}
	if direction.IsZero() {
		return pending
	}
	direction = direction.Normalize()
	for _, target := range targets {
		if target == nil || target.RuntimeID() == source.RuntimeID() {
			continue
		}
		if !multiTarget && pending.ContactTargetID.Valid() && pending.ContactTargetID != target.RuntimeID() {
			continue
		}
		if !pending.ContactTargetID.Valid() {
			pending.ContactTargetID = target.RuntimeID()
		}
		pending = s.applyPendingPlayerSkillIntegratedContactEffectsToTarget(region, source, target, pending, cfg, direction, travel, now, tick, multiTarget)
	}
	return pending
}

func (s *RegionPlayerSkillCombatSystem) applyPendingPlayerSkillIntegratedContactEffectsToTarget(region *regionruntime.RegionRuntime, source domainentity.Entity, target domainentity.Entity, pending pendingPlayerSkillAction, cfg skillMovementConfig, direction domainmath.Vec3, travel float64, now time.Time, tick uint64, multiTarget bool) pendingPlayerSkillAction {
	if s == nil || source == nil || target == nil {
		return pending
	}
	stopDistance := playerSkillMovementContactStopDistance(pending.Profile, source, target, cfg)
	contactSlack := math.Max(12, firstPositiveFloat(cfg.MinLandingDistance, cfg.DesiredLandingDistance, stopDistance)*0.25)
	sourceRadius := firstPositiveFloat(source.Components().Transform.Radius, 45)
	targetRadius := firstPositiveFloat(target.Components().Transform.Radius, 45)
	bodyContact := sourceRadius + targetRadius
	if bodyContact > stopDistance {
		contactSlack = math.Max(contactSlack, bodyContact-stopDistance+8)
	}
	contactReach := stopDistance + contactSlack
	if travel > 0 {
		contactReach += travel
	}
	if source.Position().Distance(target.Position()) > contactReach {
		return pending
	}

	controlArmor := s.playerSkillTargetControlArmorActive(target, now)
	pending = s.applyPendingPlayerSkillContactControl(source, target, pending, cfg, direction, now, contactReach)
	if controlArmor || !cfg.AppliesKnockback || travel <= domainmath.Epsilon {
		return pending
	}
	maxPushDistance := cfg.KnockbackDistance
	if maxPushDistance <= 0 {
		maxPushDistance = math.Max(80, cfg.Distance*0.25)
	}
	appliedPush := pendingPlayerSkillTargetPushDistance(pending, target.RuntimeID(), multiTarget)
	push := math.Min(travel, maxPushDistance-appliedPush)
	if push <= domainmath.Epsilon {
		return pending
	}
	targetNext := target.Position().Add(direction.Mul(push))
	targetNext.Z = target.Position().Z
	if playerSkillReleaseHitsHardBlocker(region, target.Position(), targetNext) {
		pending = addPendingPlayerSkillTargetPushDistance(pending, target.RuntimeID(), push, multiTarget)
		pending.ReleaseStartedAt = now
		pending.ReleaseTargetID = target.RuntimeID()
		pending.ReleaseDirection = direction
		pending.ReleaseDistance = push
		pending.ReleaseAppliedDistance = push
		pending.ReleaseSpeed = firstPositiveFloat(cfg.KnockbackSpeed, cfg.Speed)
		pending.ReleaseStunMS = playerSkillReleasePolicyInt(playerSkillContactReleasePolicy(pending.Profile), "stun", 0)
		pending.ContactTargetReleased = true
		if pending.ReleaseStunMS > 0 {
			s.applyPlayerSkillReleaseStun(target, pending, pending.ReleaseStunMS, now)
		}
		applyPlayerSkillTargetPlacement(target, target.Position(), domainmath.Vec3{}, true, "player_skill_contact_blocked", tick)
		return pending
	}
	applyPlayerSkillTargetPlacement(target, targetNext, domainmath.Vec3{}, false, "player_skill_contact_push", tick)
	pending = addPendingPlayerSkillTargetPushDistance(pending, target.RuntimeID(), push, multiTarget)
	pending = separatePendingPlayerSkillContactBodies(source, target, pending, direction, tick, multiTarget)
	pushSpeed := firstPositiveFloat(cfg.KnockbackSpeed, cfg.Speed)
	if pushSpeed > 0 {
		applyPlayerSkillTargetPlacement(target, target.Position(), direction.Mul(pushSpeed), true, "player_skill_contact_push_velocity", tick)
	}
	return pending
}

func (s *RegionPlayerSkillCombatSystem) applyPendingPlayerSkillContactControl(source domainentity.Entity, target domainentity.Entity, pending pendingPlayerSkillAction, cfg skillMovementConfig, direction domainmath.Vec3, now time.Time, projectedContactReach float64) pendingPlayerSkillAction {
	if source == nil || target == nil {
		return pending
	}
	if target.RuntimeID() == source.RuntimeID() {
		return pending
	}
	direction.Z = 0
	if direction.IsZero() {
		direction = domainmath.Direction(source.Position(), target.Position())
		direction.Z = 0
	}
	if direction.IsZero() {
		return pending
	}
	direction = direction.Normalize()
	stopDistance := playerSkillMovementContactStopDistance(pending.Profile, source, target, cfg)
	contactSlack := math.Max(12, firstPositiveFloat(cfg.MinLandingDistance, cfg.DesiredLandingDistance, stopDistance)*0.25)
	sourceRadius := firstPositiveFloat(source.Components().Transform.Radius, 45)
	targetRadius := firstPositiveFloat(target.Components().Transform.Radius, 45)
	bodyContact := sourceRadius + targetRadius
	if bodyContact > stopDistance {
		contactSlack = math.Max(contactSlack, bodyContact-stopDistance+8)
	}
	contactReach := stopDistance + contactSlack
	if projectedContactReach > contactReach {
		contactReach = projectedContactReach
	}
	if source.Position().Distance(target.Position()) > contactReach {
		return pending
	}
	controlArmor := s.playerSkillTargetControlArmorActive(target, now)
	skillID := pendingPlayerSkillID(pending)
	if controlArmor && playerSkillContactDebugLogs(skillID) {
		log := logging.WithComponent("combat")
		log.Info().
			Str("event", "player_skill_contact_control_blocked").
			Str("skill_id", skillID).
			Int64("source_id", int64(source.RuntimeID())).
			Int64("target_id", int64(target.RuntimeID())).
			Float64("stop_distance", stopDistance).
			Float64("contact_slack", contactSlack).
			Msg("player skill contact control blocked by target armor")
	}
	if !controlArmor {
		pending = s.applyPendingPlayerSkillContactControlEffects(source, target, pending, now)
		if pendingPlayerSkillContactControlApplied(pending, target.RuntimeID()) {
			pending = setPendingPlayerSkillContactInterruptApplied(pending, target.RuntimeID())
		}
		pending = s.applyPendingPlayerSkillContactInterrupt(target, pending, now)
	}
	pending = separatePendingPlayerSkillContactBodies(source, target, pending, direction, 0, playerSkillContactAllowsMultiTarget(pending.Profile))
	return pending
}

func applyPlayerSkillTargetPlacement(target domainentity.Entity, position domainmath.Position, velocity domainmath.Vec3, hasVelocity bool, reason string, tick uint64) {
	if target == nil {
		return
	}
	movement.ApplyServerPlacement(target, position, movement.ServerPlacementOptions{
		Velocity:    velocity,
		HasVelocity: hasVelocity,
		Reason:      reason,
	}, tick)
}

func separatePendingPlayerSkillContactBodies(source domainentity.Entity, target domainentity.Entity, pending pendingPlayerSkillAction, direction domainmath.Vec3, tick uint64, multiTarget bool) pendingPlayerSkillAction {
	if source == nil || target == nil || source.RuntimeID() == target.RuntimeID() {
		return pending
	}
	direction.Z = 0
	if direction.IsZero() {
		direction = domainmath.Direction(source.Position(), target.Position())
		direction.Z = 0
	}
	if direction.IsZero() {
		return pending
	}
	direction = direction.Normalize()
	sourceRadius := firstPositiveFloat(source.Components().Transform.Radius, 45)
	targetRadius := firstPositiveFloat(target.Components().Transform.Radius, 45)
	minSeparation := sourceRadius + targetRadius
	if minSeparation <= 0 {
		return pending
	}
	delta := target.Position().Sub(source.Position())
	delta.Z = 0
	separationDirection := delta
	if separationDirection.IsZero() {
		separationDirection = direction
	}
	if separationDirection.IsZero() {
		return pending
	}
	separationDirection = separationDirection.Normalize()
	currentSeparation := math.Max(0, delta.Dot(separationDirection))
	if currentSeparation >= minSeparation-domainmath.Epsilon {
		return pending
	}
	adjust := minSeparation - currentSeparation
	targetNext := target.Position().Add(separationDirection.Mul(adjust))
	targetNext.Z = target.Position().Z
	applyPlayerSkillTargetPlacement(target, targetNext, domainmath.Vec3{}, false, "player_skill_contact_separation", tick)
	pending = addPendingPlayerSkillTargetPushDistance(pending, target.RuntimeID(), adjust, multiTarget)
	return pending
}

func (s *RegionPlayerSkillCombatSystem) applyPendingPlayerSkillContactInterrupt(target domainentity.Entity, pending pendingPlayerSkillAction, now time.Time) pendingPlayerSkillAction {
	if s == nil || target == nil || pendingPlayerSkillContactInterruptApplied(pending, target.RuntimeID()) || !playerSkillProfileCanInterruptCreature(pending.Profile) {
		return pending
	}
	skillID := "player_skill"
	if pending.Profile.Skill != nil && pending.Profile.Skill.GetId() != "" {
		skillID = pending.Profile.Skill.GetId()
	}
	if s.interruptControlledCreatureTarget(target, now, fmt.Sprintf("player_skill_contact_interrupt:%s", skillID)) {
		pending = setPendingPlayerSkillContactInterruptApplied(pending, target.RuntimeID())
	}
	return pending
}

func (s *RegionPlayerSkillCombatSystem) applyPendingPlayerSkillContactControlEffects(source domainentity.Entity, target domainentity.Entity, pending pendingPlayerSkillAction, now time.Time) pendingPlayerSkillAction {
	if s == nil || s.Damage == nil || s.Damage.Status == nil || source == nil || target == nil || pendingPlayerSkillContactControlApplied(pending, target.RuntimeID()) {
		return pending
	}
	applied := false
	for _, control := range pending.Profile.ControlEffects {
		if control == nil || !control.GetEnabled() {
			continue
		}
		effect := statusEffectFromSkillControlEffect(control)
		if effect == nil {
			continue
		}
		if s.Damage.Status.ApplyControl(source, target, effect, control, now) {
			applied = true
		}
	}
	if applied {
		pending = setPendingPlayerSkillContactControlApplied(pending, target.RuntimeID())
		skillID := pendingPlayerSkillID(pending)
		interrupted := s.interruptControlledCreatureTarget(target, now, fmt.Sprintf("player_skill_contact_control:%s", skillID))
		if playerSkillContactDebugLogs(skillID) {
			status := target.Components().Status
			log := logging.WithComponent("combat")
			log.Info().
				Str("event", "player_skill_contact_control_applied").
				Str("skill_id", skillID).
				Int64("source_id", int64(source.RuntimeID())).
				Int64("target_id", int64(target.RuntimeID())).
				Bool("interrupted", interrupted).
				Bool("stunned", status.Stunned).
				Bool("staggered", status.Staggered).
				Bool("silenced", status.Silenced).
				Bool("rooted", status.Rooted).
				Msg("player skill contact control applied")
		}
	}
	return pending
}

func (s *RegionPlayerSkillCombatSystem) interruptControlledCreatureTarget(target domainentity.Entity, now time.Time, reason string) bool {
	if s == nil || s.CreatureInterrupts == nil || target == nil || target.EntityType() != domainentity.EntityTypeCreature {
		return false
	}
	return s.CreatureInterrupts.InterruptCreatureAction(target, now, reason)
}

func (s *RegionPlayerSkillCombatSystem) interruptCreatureTargetFromPlayerSkill(target domainentity.Entity, now time.Time, profile AttackProfile, result DamageResult, reasonPrefix string) bool {
	if target == nil {
		return false
	}
	if len(result.StatusApplied) == 0 && !playerSkillTargetHasActiveControl(target, now) && !playerSkillImpactCanInterruptCreature(profile, result) {
		return false
	}
	skillID := "player_skill"
	if profile.Skill != nil && profile.Skill.GetId() != "" {
		skillID = profile.Skill.GetId()
	}
	if reasonPrefix == "" {
		reasonPrefix = "player_skill"
	}
	return s.interruptControlledCreatureTarget(target, now, fmt.Sprintf("%s_interrupt:%s", reasonPrefix, skillID))
}

func pendingPlayerSkillID(pending pendingPlayerSkillAction) string {
	if pending.Profile.Skill != nil && pending.Profile.Skill.GetId() != "" {
		return pending.Profile.Skill.GetId()
	}
	if pending.Intent.SkillID != "" {
		return pending.Intent.SkillID.String()
	}
	return "player_skill"
}

func playerSkillContactDebugLogs(skillID string) bool {
	switch strings.ToLower(strings.TrimSpace(skillID)) {
	case "player_shield_bash", "player_shield_rush":
		return true
	default:
		return false
	}
}

func playerSkillImpactCanInterruptCreature(profile AttackProfile, result DamageResult) bool {
	if result.Evaded || result.Parried || result.Killed {
		return false
	}
	if !playerSkillProfileCanInterruptCreature(profile) {
		return false
	}
	return result.FinalDamage > 0 ||
		result.PostureDamage > 0 ||
		result.PoiseDamage > 0 ||
		result.Blocked ||
		result.Staggered
}

func playerSkillProfileCanInterruptCreature(profile AttackProfile) bool {
	if profile.Skill != nil {
		skillType := strings.ToLower(strings.TrimSpace(profile.Skill.GetSkillType()))
		if strings.Contains(skillType, "interrupt") {
			return true
		}
	}
	if profile.Impact != nil {
		if profile.Impact.GetInterruptPower() >= 0.5 || profile.Impact.GetStaggerPower() >= 0.75 {
			return true
		}
		hitReaction := strings.ToLower(strings.TrimSpace(profile.Impact.GetHitReaction()))
		if hitReaction == "interrupt" || hitReaction == "stagger" || hitReaction == "stun" || hitReaction == "knockdown" {
			return true
		}
		impactType := strings.ToLower(strings.TrimSpace(profile.Impact.GetImpactType()))
		if impactType == "interrupt" || impactType == "stagger" || impactType == "stun" {
			return true
		}
	}
	for _, control := range profile.ControlEffects {
		if control == nil || !control.GetEnabled() {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(control.GetControlType())) {
		case "stun", "stagger", "knockdown", "suppress_actions", "suppress_movement", "root", "grab", "attach":
			return true
		}
	}
	return false
}

func playerSkillTargetHasActiveControl(target domainentity.Entity, now time.Time) bool {
	if target == nil {
		return false
	}
	status := target.Components().Status
	if status.Stunned || status.Rooted || status.Silenced || status.Staggered || status.KnockedDown || status.Attached {
		return true
	}
	return status.CCEndMS > 0 && (now.IsZero() || status.CCEndMS > now.UnixMilli())
}

func (s *RegionPlayerSkillCombatSystem) beginPendingPlayerSkillContactRelease(source domainentity.Entity, resolver hitbox.EntityResolver, pending pendingPlayerSkillAction, cfg skillMovementConfig, direction domainmath.Vec3, now time.Time, tick uint64) pendingPlayerSkillAction {
	if s == nil || source == nil || resolver == nil || pending.ContactTargetReleased || !pending.ContactTargetID.Valid() {
		return pending
	}
	if !pending.ReleaseStartedAt.IsZero() {
		return pending
	}
	policyID := playerSkillContactReleasePolicy(pending.Profile)
	if !controlReleasePolicyHas(policyID, "throw") || !controlReleasePolicyHas(policyID, "forward") {
		return pending
	}
	target, ok := resolver.Resolve(pending.ContactTargetID)
	if !ok || target == nil || target.RuntimeID() == source.RuntimeID() {
		return pending
	}
	direction.Z = 0
	if direction.IsZero() {
		direction = pending.CommittedMoveDirection
		direction.Z = 0
	}
	if direction.IsZero() {
		direction = domainmath.Direction(source.Position(), target.Position())
		direction.Z = 0
	}
	if direction.IsZero() {
		return pending
	}
	direction = direction.Normalize()
	distance := playerSkillReleaseDistance(policyID, cfg)
	speed := playerSkillReleasePolicyFloat(policyID, "speed", firstPositiveFloat(cfg.KnockbackSpeed, cfg.Speed, 850))
	if distance <= 0 && speed <= 0 {
		return pending
	}
	if distance > 0 && speed <= 0 {
		speed = firstPositiveFloat(cfg.KnockbackSpeed, cfg.Speed, distance/0.18)
	}
	pending.ReleaseStartedAt = now
	pending.ReleaseTargetID = target.RuntimeID()
	pending.ReleaseDirection = direction
	pending.ReleaseDistance = distance
	pending.ReleaseSpeed = speed
	pending.ReleaseStunMS = playerSkillReleasePolicyInt(policyID, "stun", 0)
	if speed > 0 {
		applyPlayerSkillTargetPlacement(target, target.Position(), direction.Mul(speed), true, "player_skill_contact_release_start", tick)
	}
	return pending
}

func (s *RegionPlayerSkillCombatSystem) applyPendingPlayerSkillContactRelease(region *regionruntime.RegionRuntime, resolver hitbox.EntityResolver, pending pendingPlayerSkillAction, now time.Time, tick uint64) pendingPlayerSkillAction {
	if s == nil || resolver == nil || pending.ReleaseStartedAt.IsZero() || pending.ContactTargetReleased {
		return pending
	}
	target, ok := resolver.Resolve(pending.ReleaseTargetID)
	if !ok || target == nil {
		pending.ContactTargetReleased = true
		return pending
	}
	direction := pending.ReleaseDirection
	direction.Z = 0
	if direction.IsZero() {
		pending.ContactTargetReleased = true
		applyPlayerSkillTargetPlacement(target, target.Position(), domainmath.Vec3{}, true, "player_skill_contact_release_no_direction", tick)
		return pending
	}
	direction = direction.Normalize()
	distance := pending.ReleaseDistance
	speed := pending.ReleaseSpeed
	if distance <= 0 {
		pending.ContactTargetReleased = true
		applyPlayerSkillTargetPlacement(target, target.Position(), domainmath.Vec3{}, true, "player_skill_contact_release_no_distance", tick)
		return pending
	}
	elapsed := now.Sub(pending.ReleaseStartedAt)
	if elapsed < 0 {
		return pending
	}
	targetApplied := distance
	if speed > 0 {
		targetApplied = math.Min(distance, speed*elapsed.Seconds())
	}
	travel := targetApplied - pending.ReleaseAppliedDistance
	if travel <= domainmath.Epsilon {
		if targetApplied >= distance-domainmath.Epsilon {
			pending.ContactTargetReleased = true
			applyPlayerSkillTargetPlacement(target, target.Position(), domainmath.Vec3{}, true, "player_skill_contact_release_complete", tick)
		}
		return pending
	}
	from := target.Position()
	to := from.Add(direction.Mul(travel))
	to.Z = from.Z
	if playerSkillReleaseHitsHardBlocker(region, from, to) {
		if pending.ReleaseStunMS > 0 {
			s.applyPlayerSkillReleaseStun(target, pending, pending.ReleaseStunMS, now)
		}
		applyPlayerSkillTargetPlacement(target, target.Position(), domainmath.Vec3{}, true, "player_skill_contact_release_blocked", tick)
		pending.ReleaseAppliedDistance = distance
		pending.ContactTargetReleased = true
	} else {
		applyPlayerSkillTargetPlacement(target, to, domainmath.Vec3{}, false, "player_skill_contact_release_move", tick)
		pending.ReleaseAppliedDistance += travel
		if speed > 0 {
			applyPlayerSkillTargetPlacement(target, target.Position(), direction.Mul(speed), true, "player_skill_contact_release_velocity", tick)
		}
	}
	if pending.ReleaseAppliedDistance >= distance-domainmath.Epsilon {
		pending.ContactTargetReleased = true
		applyPlayerSkillTargetPlacement(target, target.Position(), domainmath.Vec3{}, true, "player_skill_contact_release_complete", tick)
	}
	return pending
}

func playerSkillReleaseDistance(policyID string, cfg skillMovementConfig) float64 {
	if distance := playerSkillReleasePolicyFloat(policyID, "distance", 0); distance > 0 {
		return distance
	}
	if ratio := playerSkillReleasePolicyFloat(policyID, "ratio", 0); ratio > 0 {
		return cfg.Distance * ratio
	}
	return firstPositiveFloat(cfg.Distance*0.2, cfg.KnockbackDistance*0.2, 140)
}

func playerSkillContactReleasePolicy(profile AttackProfile) string {
	for _, control := range profile.ControlEffects {
		if control == nil || !control.GetEnabled() {
			continue
		}
		if policyID := strings.TrimSpace(control.GetReleasePolicyId()); policyID != "" {
			return policyID
		}
	}
	return ""
}

func playerSkillReleasePolicyFloat(policyID string, key string, fallback float64) float64 {
	value, ok := playerSkillReleasePolicyToken(policyID, key)
	if !ok {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func playerSkillReleasePolicyInt(policyID string, key string, fallback int32) int32 {
	value, ok := playerSkillReleasePolicyToken(policyID, key)
	if !ok {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return fallback
	}
	return int32(parsed)
}

func playerSkillReleasePolicyToken(policyID string, key string) (string, bool) {
	key = strings.ToLower(strings.TrimSpace(key))
	if key == "" {
		return "", false
	}
	tokens := strings.FieldsFunc(strings.ToLower(strings.TrimSpace(policyID)), func(r rune) bool {
		return r == '_' || r == '-' || r == ':' || r == '=' || r == ' '
	})
	for i := 0; i+1 < len(tokens); i++ {
		if tokens[i] == key {
			return tokens[i+1], true
		}
	}
	return "", false
}

func playerSkillReleaseHitsHardBlocker(region *regionruntime.RegionRuntime, from domainmath.Position, to domainmath.Position) bool {
	if region == nil {
		return false
	}
	segment := domainmath.NewSegment(from, to)
	for _, blocker := range region.Package().Blockers {
		if !blocker.BlocksNav {
			continue
		}
		if domainmath.SegmentIntersectsAABB(segment, blocker.Bounds()) {
			return true
		}
	}
	return false
}

func (s *RegionPlayerSkillCombatSystem) applyPlayerSkillReleaseStun(target domainentity.Entity, pending pendingPlayerSkillAction, durationMS int32, now time.Time) {
	if s == nil || s.Damage == nil || s.Damage.Status == nil || target == nil || durationMS <= 0 {
		return
	}
	skillID := "player_skill"
	if pending.Profile.Skill != nil && pending.Profile.Skill.GetId() != "" {
		skillID = pending.Profile.Skill.GetId()
	}
	effect := &apeironv1.StatusEffect{
		Id:             skillID + "_release_wall_stun",
		Name:           skillID + "_release_wall_stun",
		EffectType:     "crowd_control",
		EffectCategory: "crowd_control",
		ControlType:    "stun",
		StackingMode:   "refresh",
		MaxStacks:      1,
		DurationMs:     durationMS,
		IsPvpEnabled:   true,
		BlocksMovement: true,
		BlocksActions:  true,
		BlocksSkills:   true,
	}
	s.Damage.Status.Apply(target, effect, now)
}

func (s *RegionPlayerSkillCombatSystem) resolvePendingPlayerSkillContactTarget(source domainentity.Entity, index spatial.SpatialIndex, resolver hitbox.EntityResolver, pending pendingPlayerSkillAction, cfg skillMovementConfig, direction domainmath.Vec3, travel float64) (domainentity.Entity, bool) {
	targets := s.resolvePendingPlayerSkillContactTargets(source, index, resolver, pending, cfg, direction, travel)
	if len(targets) == 0 {
		return nil, false
	}
	return targets[0], true
}

func (s *RegionPlayerSkillCombatSystem) resolvePendingPlayerSkillContactTargets(source domainentity.Entity, index spatial.SpatialIndex, resolver hitbox.EntityResolver, pending pendingPlayerSkillAction, cfg skillMovementConfig, direction domainmath.Vec3, travel float64) []domainentity.Entity {
	if source == nil || resolver == nil || travel <= domainmath.Epsilon {
		return nil
	}
	multiTarget := playerSkillContactAllowsMultiTarget(pending.Profile)
	maxTargets := playerSkillContactMaxTargets(pending.Profile)
	candidates := make([]playerSkillContactCandidate, 0, playerSkillContactCandidateCapacity(maxTargets))
	seen := make(map[ids.RuntimeEntityID]struct{})
	addCandidate := func(target domainentity.Entity, priority int) {
		if target == nil || target.RuntimeID() == source.RuntimeID() {
			return
		}
		if _, ok := seen[target.RuntimeID()]; ok {
			return
		}
		projected, _, ok := playerSkillContactLaneProjection(source, target, pending, cfg, direction, travel, multiTarget)
		if !ok {
			return
		}
		seen[target.RuntimeID()] = struct{}{}
		candidates = append(candidates, playerSkillContactCandidate{
			target:    target,
			projected: projected,
			distance:  source.Position().Distance(target.Position()),
			priority:  priority,
		})
	}
	if pending.ContactTargetID.Valid() {
		if target, ok := resolver.Resolve(pending.ContactTargetID); ok && target != nil && target.RuntimeID() != source.RuntimeID() {
			addCandidate(target, 2)
			if !multiTarget && len(candidates) > 0 {
				return []domainentity.Entity{candidates[0].target}
			}
		}
	}
	if pending.Intent.HasTarget {
		if target, ok := resolver.Resolve(pending.Intent.Target.RuntimeID); ok && target != nil && target.RuntimeID() != source.RuntimeID() {
			addCandidate(target, 1)
			if !multiTarget && len(candidates) > 0 {
				return []domainentity.Entity{candidates[0].target}
			}
		}
	}
	if index == nil {
		return contactCandidateTargets(candidates, maxTargets)
	}
	direction.Z = 0
	if direction.IsZero() {
		direction = pending.CommittedMoveDirection
		direction.Z = 0
	}
	if direction.IsZero() {
		return contactCandidateTargets(candidates, maxTargets)
	}
	direction = direction.Normalize()
	sourceRadius := firstPositiveFloat(source.Components().Transform.Radius, 45)
	laneHalfWidth := playerSkillContactLaneHalfWidth(pending.Profile, sourceRadius, multiTarget)
	queryRadius := math.Max(60, laneHalfWidth+sourceRadius+math.Max(12, firstPositiveFloat(cfg.MinLandingDistance, cfg.DesiredLandingDistance, 30)*0.35))
	probeDistance := travel + math.Max(queryRadius, sourceRadius*2)
	from := source.Position()
	to := from.Add(direction.Mul(probeDistance))
	queryBounds := domainmath.NewSegment(from, to).Bounds().Expand(queryRadius + sourceRadius)
	queryFilter := spatial.QueryFilter{
		RegionID: source.RegionID(),
		Types:    []domainentity.EntityType{domainentity.EntityTypeCreature, domainentity.EntityTypePlayer},
		Exclude: map[ids.RuntimeEntityID]struct{}{
			source.RuntimeID(): {},
		},
	}
	results := index.QueryAABB(spatial.AABBQuery{
		Bounds: queryBounds,
		Filter: queryFilter,
	})
	if len(results) == 0 {
		queryFilter.RegionID = ""
		queryFilter.Types = nil
		results = index.QueryAABB(spatial.AABBQuery{
			Bounds: queryBounds,
			Filter: queryFilter,
		})
	}
	for _, result := range results {
		target, ok := resolver.Resolve(result.Object.ID)
		if !ok || target == nil || target.RuntimeID() == source.RuntimeID() {
			continue
		}
		addCandidate(target, 0)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].priority != candidates[j].priority {
			return candidates[i].priority > candidates[j].priority
		}
		if candidates[i].projected != candidates[j].projected {
			return candidates[i].projected < candidates[j].projected
		}
		return candidates[i].distance < candidates[j].distance
	})
	return contactCandidateTargets(candidates, maxTargets)
}

func playerSkillContactCandidateCapacity(maxTargets int32) int {
	if maxTargets > 0 && maxTargets < 16 {
		return int(maxTargets)
	}
	return 4
}

func contactCandidateTargets(candidates []playerSkillContactCandidate, maxTargets int32) []domainentity.Entity {
	if len(candidates) == 0 {
		return nil
	}
	if maxTargets > 0 && len(candidates) > int(maxTargets) {
		candidates = candidates[:int(maxTargets)]
	}
	targets := make([]domainentity.Entity, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.target != nil {
			targets = append(targets, candidate.target)
		}
	}
	return targets
}

func playerSkillContactMaxTargets(profile AttackProfile) int32 {
	maxTargets := int32(0)
	if profile.Skill != nil && profile.Skill.GetMaxTargets() > maxTargets {
		maxTargets = profile.Skill.GetMaxTargets()
	}
	for _, hitboxProfile := range profile.Hitboxes {
		if hitboxProfile != nil && hitboxProfile.GetMaxTargets() > maxTargets {
			maxTargets = hitboxProfile.GetMaxTargets()
		}
	}
	if maxTargets <= 0 {
		return 1
	}
	return maxTargets
}

func playerSkillContactAllowsMultiTarget(profile AttackProfile) bool {
	return playerSkillContactMaxTargets(profile) > 1
}

func playerSkillContactLaneHalfWidth(profile AttackProfile, sourceRadius float64, multiTarget bool) float64 {
	halfWidth := firstPositiveFloat(sourceRadius, 45)
	if !multiTarget {
		return halfWidth
	}
	for _, hitboxProfile := range profile.Hitboxes {
		if hitboxProfile == nil {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(hitboxProfile.GetHitboxShape())) {
		case "box", "oriented_box":
			halfWidth = math.Max(halfWidth, hitboxProfile.GetSizeY()*0.5)
		case "capsule", "line", "ray", "raycast":
			halfWidth = math.Max(halfWidth, hitboxProfile.GetRadius())
		default:
			halfWidth = math.Max(halfWidth, hitboxProfile.GetRadius())
			halfWidth = math.Max(halfWidth, hitboxProfile.GetSizeY()*0.5)
		}
	}
	return math.Max(halfWidth, firstPositiveFloat(sourceRadius, 45)*2)
}

func playerSkillContactLaneProjection(source domainentity.Entity, target domainentity.Entity, pending pendingPlayerSkillAction, cfg skillMovementConfig, direction domainmath.Vec3, travel float64, multiTarget bool) (float64, float64, bool) {
	if source == nil || target == nil || target.RuntimeID() == source.RuntimeID() {
		return 0, 0, false
	}
	direction.Z = 0
	if direction.IsZero() {
		direction = pending.CommittedMoveDirection
		direction.Z = 0
	}
	if direction.IsZero() {
		return 0, 0, false
	}
	direction = direction.Normalize()
	from := source.Position()
	toTarget := target.Position().Sub(from)
	toTarget.Z = 0
	projected := toTarget.Dot(direction)
	if projected < -domainmath.Epsilon {
		return 0, 0, false
	}
	stopDistance := playerSkillMovementContactStopDistance(pending.Profile, source, target, cfg)
	if projected > travel+stopDistance+domainmath.Epsilon {
		return 0, 0, false
	}
	sourceRadius := firstPositiveFloat(source.Components().Transform.Radius, 45)
	targetRadius := firstPositiveFloat(target.Components().Transform.Radius, sourceRadius)
	lateral := toTarget.Sub(direction.Mul(projected)).Length()
	lateralLimit := sourceRadius + targetRadius
	if multiTarget {
		lateralLimit = playerSkillContactLaneHalfWidth(pending.Profile, sourceRadius, true) + targetRadius
	}
	if lateral > lateralLimit+domainmath.Epsilon {
		return 0, 0, false
	}
	return projected, lateral, true
}

func pendingPlayerSkillTargetPushDistance(pending pendingPlayerSkillAction, targetID ids.RuntimeEntityID, multiTarget bool) float64 {
	if multiTarget && targetID.Valid() && pending.AppliedTargetPushDistanceByTarget != nil {
		return pending.AppliedTargetPushDistanceByTarget[targetID]
	}
	return pending.AppliedTargetPushDistance
}

func addPendingPlayerSkillTargetPushDistance(pending pendingPlayerSkillAction, targetID ids.RuntimeEntityID, distance float64, multiTarget bool) pendingPlayerSkillAction {
	if distance <= domainmath.Epsilon {
		return pending
	}
	if multiTarget && targetID.Valid() {
		if pending.AppliedTargetPushDistanceByTarget == nil {
			pending.AppliedTargetPushDistanceByTarget = make(map[ids.RuntimeEntityID]float64)
		}
		pending.AppliedTargetPushDistanceByTarget[targetID] += distance
		pending.AppliedTargetPushDistance += distance
		return pending
	}
	pending.AppliedTargetPushDistance += distance
	return pending
}

func pendingPlayerSkillContactControlApplied(pending pendingPlayerSkillAction, targetID ids.RuntimeEntityID) bool {
	if targetID.Valid() && pending.ContactControlAppliedByTarget != nil {
		return pending.ContactControlAppliedByTarget[targetID]
	}
	return pending.ContactControlApplied
}

func setPendingPlayerSkillContactControlApplied(pending pendingPlayerSkillAction, targetID ids.RuntimeEntityID) pendingPlayerSkillAction {
	pending.ContactControlApplied = true
	if targetID.Valid() {
		if pending.ContactControlAppliedByTarget == nil {
			pending.ContactControlAppliedByTarget = make(map[ids.RuntimeEntityID]bool)
		}
		pending.ContactControlAppliedByTarget[targetID] = true
	}
	return pending
}

func pendingPlayerSkillContactInterruptApplied(pending pendingPlayerSkillAction, targetID ids.RuntimeEntityID) bool {
	if targetID.Valid() && pending.ContactInterruptAppliedByTarget != nil {
		return pending.ContactInterruptAppliedByTarget[targetID]
	}
	return pending.ContactInterruptApplied
}

func setPendingPlayerSkillContactInterruptApplied(pending pendingPlayerSkillAction, targetID ids.RuntimeEntityID) pendingPlayerSkillAction {
	pending.ContactInterruptApplied = true
	if targetID.Valid() {
		if pending.ContactInterruptAppliedByTarget == nil {
			pending.ContactInterruptAppliedByTarget = make(map[ids.RuntimeEntityID]bool)
		}
		pending.ContactInterruptAppliedByTarget[targetID] = true
	}
	return pending
}

func (s *RegionPlayerSkillCombatSystem) playerSkillTargetControlArmorActive(target domainentity.Entity, now time.Time) bool {
	if s == nil || s.Damage == nil || s.Damage.Policies == nil || target == nil || now.IsZero() {
		return false
	}
	skills := target.Components().Skills
	if skills.CurrentSkillID == "" || skills.StartedAtMS <= 0 {
		return false
	}
	elapsedMS := now.UnixMilli() - skills.StartedAtMS
	if elapsedMS < 0 {
		return false
	}
	for _, window := range s.Damage.Policies.ResolveSkillActionWindows(skills.CurrentSkillID.String()) {
		if !window.Enabled || int32(elapsedMS) < window.StartMS || int32(elapsedMS) > window.EndMS {
			continue
		}
		if strings.EqualFold(window.WindowType, "hyperarmor") {
			return true
		}
		if movementPoisePolicyIgnoresStagger(window.PoisePolicy) && skillInterruptPolicyIgnoresStagger(window.InterruptPolicy) {
			return true
		}
	}
	return false
}

func commitPendingPlayerSkillMovementTarget(source domainentity.Entity, resolver hitbox.EntityResolver, pending pendingPlayerSkillAction, cfg skillMovementConfig) pendingPlayerSkillAction {
	if source == nil {
		return pending
	}
	landingTarget := domainmath.Position{}

	// Dash/charge-style grounded skills own their root by a forward distance
	// contract. The target entity and mouse position are hit/contact inputs, not
	// movement anchors; using them here makes close-range Shield Rush/Bash cut
	// diagonally or shrink when the cursor is on the ground near the target.
	if pendingPlayerSkillMovementUsesForwardDistance(pending, cfg) {
		direction := playerSkillForwardMovementDirection(source, pending)
		distance := pendingPlayerSkillMovementDistance(pending, cfg)
		if !direction.IsZero() && distance > 0 {
			landingTarget = source.Position().Add(direction.Mul(distance))
			landingTarget.Z = source.Position().Z
		}
	}
	if landingTarget.IsZero() && pending.Intent.HasTarget && resolver != nil {
		if target, ok := resolver.Resolve(pending.Intent.Target.RuntimeID); ok && target != nil {
			landingTarget = target.Position()
		}
	}
	if landingTarget.IsZero() && pending.Intent.HasPosition {
		landingTarget = pending.Intent.TargetPosition
	}
	if landingTarget.IsZero() {
		direction := pending.CommittedMoveDirection
		if direction.IsZero() && pending.Intent.HasAim {
			direction = pending.Intent.AimDirection
		}
		direction.Z = 0
		if !direction.IsZero() {
			distance := pendingPlayerSkillMovementDistance(pending, cfg)
			landingTarget = source.Position().Add(direction.Normalize().Mul(distance))
		}
	}
	if landingTarget.IsZero() {
		return pending
	}
	direction := domainmath.Direction(source.Position(), landingTarget)
	direction.Z = 0
	pending.CommittedTargetPosition = landingTarget
	if !direction.IsZero() {
		pending.CommittedMoveDirection = direction.Normalize()
	}
	pending.MovementTargetCommitted = true
	return pending
}

func playerSkillMovementStopDistance(source domainentity.Entity, resolver hitbox.EntityResolver, pending pendingPlayerSkillAction, cfg skillMovementConfig) float64 {
	if pending.Intent.HasTarget && resolver != nil {
		if target, ok := resolver.Resolve(pending.Intent.Target.RuntimeID); ok && target != nil {
			return playerSkillMovementContactStopDistance(pending.Profile, source, target, cfg)
		}
	}
	if cfg.CanPhaseThroughTargets {
		return 0
	}
	if pendingPlayerSkillMovementUsesForwardDistance(pending, cfg) {
		return 0
	}
	if desired := firstPositiveFloat(cfg.DesiredLandingDistance, cfg.MinLandingDistance); desired > 0 {
		return desired
	}
	return 0
}

func playerSkillMovementContactStopDistance(profile AttackProfile, source domainentity.Entity, target domainentity.Entity, cfg skillMovementConfig) float64 {
	if source == nil || target == nil {
		return 0
	}
	base := skillMovementContactStopDistance(profile, source, target, cfg)
	if cfg.CanPhaseThroughTargets {
		return base
	}
	sourceRadius := firstPositiveFloat(source.Components().Transform.Radius, 45)
	targetRadius := firstPositiveFloat(target.Components().Transform.Radius, 45)
	bodyContact := firstPositiveFloat(sourceRadius+targetRadius, base)
	return math.Max(base, bodyContact)
}

func playerSkillMovementUsesForwardDistance(cfg skillMovementConfig) bool {
	switch strings.ToLower(strings.TrimSpace(cfg.MovementType)) {
	case "dash", "charge":
		return true
	default:
		return false
	}
}

func playerSkillMovementTypeUsesForwardDistance(movementType string) bool {
	switch strings.ToLower(strings.TrimSpace(movementType)) {
	case "dash", "charge", "skill_step", "ground_slide", "grounded_skill", "grounded_skill_action":
		return true
	default:
		return false
	}
}

func playerSkillMovementContractUsesForwardDistance(contract movement.MovementActionContract) bool {
	return contract.HorizontalDistanceCM > 0 && playerSkillMovementTypeUsesForwardDistance(contract.MovementType)
}

func pendingPlayerSkillMovementUsesForwardDistance(pending pendingPlayerSkillAction, cfg skillMovementConfig) bool {
	if pending.HasMovementContract {
		return playerSkillMovementContractUsesForwardDistance(pending.MovementContract)
	}
	return playerSkillMovementUsesForwardDistance(cfg)
}

func playerSkillForwardMovementDirection(source domainentity.Entity, pending pendingPlayerSkillAction) domainmath.Vec3 {
	direction := pending.CommittedMoveDirection
	if direction.IsZero() && pending.Intent.HasAim {
		direction = pending.Intent.AimDirection
	}
	if direction.IsZero() {
		direction = sourceFacingDirection(source)
	}
	direction.Z = 0
	if direction.IsZero() {
		return domainmath.Vec3{}
	}
	return direction.Normalize()
}

func pendingPlayerSkillMovementDistance(pending pendingPlayerSkillAction, cfg skillMovementConfig) float64 {
	if pending.HasMovementContract {
		contract := pending.MovementContract
		if contract.HorizontalDistanceCM > 0 {
			return contract.HorizontalDistanceCM
		}
		if contract.BaseSpeedCMPerSec > 0 {
			durationMS := contract.ActiveMS
			if durationMS <= 0 {
				durationMS = contract.DurationMS
			}
			if durationMS > 0 {
				return contract.BaseSpeedCMPerSec * (float64(durationMS) / 1000.0)
			}
		}
	}
	distance := cfg.Distance
	if distance <= 0 && cfg.Speed > 0 && cfg.DurationMS > 0 {
		distance = cfg.Speed * (float64(cfg.DurationMS) / 1000.0)
	}
	if distance <= 0 && pending.Profile.Skill != nil {
		distance = firstPositiveFloat(pending.Profile.Skill.GetMovementDistance(), pending.Profile.Skill.GetMaxRange())
	}
	return firstPositiveFloat(distance, 300)
}

func sourceFacingDirection(source domainentity.Entity) domainmath.Vec3 {
	if source == nil {
		return domainmath.Vec3{}
	}
	components := source.Components()
	yaw := components.Movement.Locomotion.AuthoritativeYaw
	if yaw == 0 {
		yaw = components.Transform.RotationY
	}
	return yawToDirection(yaw)
}

func pendingPlayerSkillMovementStart(pending pendingPlayerSkillAction, cfg skillMovementConfig) time.Duration {
	if pending.HasMovementContract && playerSkillMovementContractUsesForwardDistance(pending.MovementContract) {
		return time.Duration(maxInt32(pending.MovementContract.StartupMS, 0)) * time.Millisecond
	}
	return skillMovementStart(skillMovementTimelineContext{
		Timing:              pending.Timing,
		HitboxStart:         pending.HitboxStart,
		HasMovementContract: pending.HasMovementContract,
		MovementContract:    pending.MovementContract,
	}, cfg)
}

func pendingPlayerSkillMovementDuration(pending pendingPlayerSkillAction, cfg skillMovementConfig, movementStart time.Duration) time.Duration {
	duration := time.Duration(cfg.DurationMS) * time.Millisecond
	if pending.HasMovementContract {
		if pending.MovementContract.ActiveMS > 0 {
			duration = time.Duration(pending.MovementContract.ActiveMS) * time.Millisecond
		} else if pending.MovementContract.DurationMS > 0 {
			duration = time.Duration(pending.MovementContract.DurationMS) * time.Millisecond
		}
	}
	if duration <= 0 && pending.Timing.ActiveEnd > movementStart {
		duration = pending.Timing.ActiveEnd - movementStart
	}
	if duration <= 0 {
		duration = 200 * time.Millisecond
	}
	return duration
}

func pendingPlayerSkillActionDuration(pending pendingPlayerSkillAction, movementStart time.Duration, movementDuration time.Duration) time.Duration {
	duration := pending.Timing.ActionLock
	if pending.HasMovementContract && pending.MovementContract.DurationMS > 0 {
		duration = time.Duration(pending.MovementContract.DurationMS) * time.Millisecond
	}
	if duration <= 0 {
		duration = pending.Timing.ActiveEnd + pending.Timing.Recovery
	}
	minimum := movementStart + movementDuration
	if duration < minimum {
		duration = minimum
	}
	if duration <= 0 {
		duration = minimum
	}
	return duration
}

func pendingPlayerSkillContractSpeedCurve(pending pendingPlayerSkillAction) (movement.MovementActionCurve, bool) {
	if !pending.HasMovementContract {
		return movement.MovementActionCurve{}, false
	}
	curve, ok := pending.MovementContract.Curve(movement.MovementActionCurveHorizontalSpeedScale)
	return curve, ok && len(curve.Samples) > 0
}

func pendingPlayerSkillMovementDistanceAtElapsed(pending pendingPlayerSkillAction, cfg skillMovementConfig, elapsed time.Duration, duration time.Duration, totalDistance float64) float64 {
	if totalDistance <= 0 || elapsed <= 0 {
		return 0
	}
	if duration <= 0 || elapsed >= duration {
		return totalDistance
	}
	progress := clamp(float64(elapsed)/float64(duration), 0, 1)
	if curve, ok := pendingPlayerSkillContractSpeedCurve(pending); ok {
		return totalDistance * integrateSkillMovementCurveProgress(curve.Samples, progress)
	}
	return skillMovementDistanceAtElapsed(cfg, elapsed, duration, totalDistance)
}

func pendingPlayerSkillMovementSpeedScale(pending pendingPlayerSkillAction, cfg skillMovementConfig, elapsed time.Duration, duration time.Duration) float64 {
	if duration <= 0 || elapsed < 0 || elapsed > duration {
		return 0
	}
	progress := float64(elapsed) / float64(duration)
	if curve, ok := pendingPlayerSkillContractSpeedCurve(pending); ok {
		return sampleSkillMovementCurve(curve.Samples, progress)
	}
	return skillMovementSpeedScaleForProgress(cfg, progress)
}

func pendingPlayerSkillMovementBaseForce(pending pendingPlayerSkillAction, cfg skillMovementConfig, duration time.Duration, totalDistance float64) float64 {
	if pending.HasMovementContract {
		if speed := skillMovementContractHorizontalSpeed(pending.MovementContract); speed > 0 {
			return speed
		}
	}
	if cfg.Speed > 0 {
		return cfg.Speed
	}
	if totalDistance > 0 && duration > 0 {
		return totalDistance / duration.Seconds()
	}
	return 0
}

func pendingPlayerSkillMovementIntentForceAndScale(pending pendingPlayerSkillAction, cfg skillMovementConfig, sourceTravel float64, duration time.Duration, totalDistance float64, delta time.Duration) (float64, float64) {
	deltaSeconds := movement.DeltaSeconds(delta)
	if sourceTravel <= 0 || deltaSeconds <= 0 {
		return 0, 0
	}
	effectiveSpeed := sourceTravel / deltaSeconds
	baseForce := pendingPlayerSkillMovementBaseForce(pending, cfg, duration, totalDistance)
	if baseForce <= 0 {
		return effectiveSpeed, 1
	}
	return baseForce, effectiveSpeed / baseForce
}

func playerSkillMovementContractPredictionErrorPolicy(contract movement.MovementActionContract) string {
	for _, policy := range contract.AxisPolicies {
		if !policy.Enabled || policy.Axis != "horizontal" || policy.ReconciliationPolicy == "" {
			continue
		}
		return policy.ReconciliationPolicy
	}
	return ""
}

func pendingPlayerSkillLocomotionPhase(elapsed time.Duration, movementStart time.Duration, movementDuration time.Duration, actionDuration time.Duration) (string, int32, int32) {
	if elapsed < movementStart {
		return "startup",
			int32(math.Round(elapsed.Seconds() * 1000)),
			int32(math.Round((movementStart - elapsed).Seconds() * 1000))
	}
	movementElapsed := elapsed - movementStart
	if movementElapsed < movementDuration {
		return "active",
			int32(math.Round(movementElapsed.Seconds() * 1000)),
			int32(math.Round((movementDuration - movementElapsed).Seconds() * 1000))
	}
	recoveryDuration := actionDuration - movementStart - movementDuration
	if recoveryDuration < 0 {
		recoveryDuration = 0
	}
	recoveryElapsed := elapsed - movementStart - movementDuration
	if recoveryElapsed < 0 {
		recoveryElapsed = 0
	}
	remaining := recoveryDuration - recoveryElapsed
	if remaining < 0 {
		remaining = 0
	}
	return "recovery",
		int32(math.Round(recoveryElapsed.Seconds() * 1000)),
		int32(math.Round(remaining.Seconds() * 1000))
}

func (s *RegionPlayerSkillCombatSystem) syncPendingPlayerSkillState(source domainentity.Entity, pending pendingPlayerSkillAction, now time.Time) {
	if source == nil || pending.Profile.Skill == nil {
		return
	}
	s.updatePlayerActionPhase(source.RuntimeID(), now, "sync_pending_player_skill")
	elapsed := now.Sub(pending.StartedAt)
	state := "windup"
	switch {
	case elapsed >= pending.Timing.ActiveEnd:
		state = "recovery"
	case elapsed >= pending.Timing.ActiveStart:
		state = "active"
	}
	components := source.Components()
	components.Skills.CurrentSkillID = ids.SkillID(pending.Profile.Skill.GetId())
	components.Skills.State = state
	components.Skills.StartedAtMS = pending.StartedAt.UnixMilli()
	components.Skills.CooldownEndMS = pending.StartedAt.Add(playerSkillCooldown(pending.Profile, pending.Timing)).UnixMilli()
	components.Skills.LastResolvedAtMS = pending.ResolveAt.UnixMilli()
	if _, ok := pendingPlayerSkillMovementConfig(pending); !ok {
		return
	}
	if pending.Intent.Sequence > 0 {
		components.Movement.LastProcessedSequence = pending.Intent.Sequence
	}
	if pending.Intent.ClientTick > 0 {
		components.Movement.LastProcessedClientTick = pending.Intent.ClientTick
	}
}

type skillMissContext struct {
	Source   domainentity.Entity
	Intent   skill.Intent
	Profile  AttackProfile
	Aim      domainmath.Vec3
	Reason   string
	Tick     uint64
	Resolver hitbox.EntityResolver
}

func (s *RegionPlayerSkillCombatSystem) recordMiss(ctx skillMissContext) {
	if s == nil || ctx.Source == nil {
		return
	}
	miss := SkillMiss{
		SkillID:        ids.SkillID(ctx.Profile.Skill.GetId()),
		SourceID:       ctx.Source.RuntimeID(),
		Reason:         ctx.Reason,
		Origin:         ctx.Source.Position(),
		AimDirection:   ctx.Aim,
		TargetPosition: ctx.Intent.TargetPosition,
		HasTarget:      ctx.Intent.HasTarget,
		HasPosition:    ctx.Intent.HasPosition,
		HitboxCount:    len(ctx.Profile.Hitboxes),
		Tick:           ctx.Tick,
	}
	if miss.SkillID == "" {
		miss.SkillID = ctx.Intent.SkillID
	}
	if ctx.Intent.HasTarget {
		miss.TargetID = ctx.Intent.Target.RuntimeID
		if ctx.Resolver != nil {
			if target, ok := ctx.Resolver.Resolve(ctx.Intent.Target.RuntimeID); ok && target != nil {
				miss.TargetPosition = target.Position()
			}
		}
	}
	s.LastMisses = append(s.LastMisses, miss)
}

func lineOfSightForRegion(region *regionruntime.RegionRuntime) hitbox.LineOfSight {
	if region == nil {
		return nil
	}
	blockers := region.Package().Blockers
	for _, blocker := range blockers {
		if blocker.BlocksLOS {
			return worldBlockerLineOfSight{Blockers: blockers}
		}
	}
	return nil
}

type worldBlockerLineOfSight struct {
	Blockers []world.BlockerDefinition
}

func (l worldBlockerLineOfSight) Clear(from domainmath.Position, to domainmath.Position) bool {
	segment := domainmath.NewSegment(from, to)
	for _, blocker := range l.Blockers {
		if !blocker.BlocksLOS {
			continue
		}
		if domainmath.SegmentIntersectsAABB(segment, blocker.Bounds()) {
			return false
		}
	}
	return true
}

func (s *RegionPlayerSkillCombatSystem) aimDirection(source domainentity.Entity, intent skill.Intent, resolver hitbox.EntityResolver) domainmath.Vec3 {
	if intent.HasAim && !intent.AimDirection.IsZero() {
		return intent.AimDirection.Normalize()
	}
	if intent.HasTarget {
		if target, ok := resolver.Resolve(intent.Target.RuntimeID); ok && target != nil {
			return domainmath.Direction(source.Position(), target.Position())
		}
	}
	if intent.HasPosition {
		return domainmath.Direction(source.Position(), intent.TargetPosition)
	}
	return domainmath.V3(1, 0, 0)
}

func (s *RegionPlayerSkillCombatSystem) spatialIndex(region *regionruntime.RegionRuntime, entities []domainentity.Entity) (spatial.SpatialIndex, error) {
	cfg := s.SpatialConfig
	if cfg.Bounds.Min.IsZero() && cfg.Bounds.Max.IsZero() {
		cfg.Bounds = region.Boundary().Bounds()
	}
	index := spatial.NewLooseQuadtree(cfg)
	for _, entity := range entities {
		if entity == nil {
			continue
		}
		if err := index.Insert(spatial.SpatialObjectFromEntity(entity)); err != nil {
			return nil, fmt.Errorf("player combat spatial insert failed for %s: %w", entity.RuntimeID(), err)
		}
	}
	return index, nil
}

func (s *RegionPlayerSkillCombatSystem) applyRegionSafeZones(region *regionruntime.RegionRuntime, entities []domainentity.Entity) {
	validator := pvp.SafeZoneValidator{Zones: region.Package().SafeZones}
	for _, entity := range entities {
		if entity != nil {
			validator.Apply(entity)
		}
	}
}

func (s *RegionPlayerSkillCombatSystem) profileForSkill(ctx context.Context, skillID ids.SkillID) AttackProfile {
	if s.Profiles == nil {
		return AttackProfile{}
	}
	return s.Profiles.ProfileForSkill(ctx, skillID)
}

func (s *RegionPlayerSkillCombatSystem) playerAttackProfileForSkill(ctx context.Context, source domainentity.Entity, skillID ids.SkillID, now time.Time, reason string) AttackProfile {
	profile := s.profileForSkill(ctx, skillID)
	if attackProfileMissingRuntimeData(profile) {
		if profile.Skill == nil {
			profile.Skill = &apeironv1.Skill{Id: skillID.String()}
		}
		entityID := ids.RuntimeEntityID(0)
		if source != nil {
			entityID = source.RuntimeID()
		}
		s.recordActionRuntimeEvent(actionRuntimeContractMissingEvent(actionruntime.ActorKindPlayer, entityID, ids.SkillID(profile.Skill.GetId()), now, reason+"_missing_profile"))
		return profile
	}
	return profile
}

func (s *RegionPlayerSkillCombatSystem) resolveBasicAttackComboIntent(ctx context.Context, source domainentity.Entity, intent skill.Intent, now time.Time) skill.Intent {
	if !skill.IsDefaultWeaponBasicAction(intent.SkillID) {
		return intent
	}
	intent.SkillID = s.nextBasicAttackComboSkillID(ctx, source, now)
	return intent
}

func (s *RegionPlayerSkillCombatSystem) nextBasicAttackComboSkillID(ctx context.Context, source domainentity.Entity, now time.Time) ids.SkillID {
	if len(playerBasicAttackComboSkillIDs) == 0 {
		return skill.DefaultWeaponBasicActionID
	}
	first := playerBasicAttackComboSkillIDs[0]
	if source == nil || now.IsZero() {
		return first
	}
	components := source.Components()
	currentSkillID := components.Skills.CurrentSkillID
	currentIndex, ok := basicAttackComboIndex(currentSkillID)
	if !ok {
		return first
	}
	startedAtMS := components.Skills.StartedAtMS
	if startedAtMS <= 0 {
		return first
	}
	previousProfile := s.playerAttackProfileForSkill(ctx, source, currentSkillID, now, "basic_combo_previous_profile")
	if previousProfile.Skill == nil || previousProfile.Skill.GetComboGroup() != playerBasicAttackComboGroup {
		return first
	}
	comboWindowMS := int64(previousProfile.Skill.GetComboWindowMs())
	if comboWindowMS <= 0 {
		comboWindowMS = 700
	}
	comboWindowStartMS := startedAtMS + basicAttackComboWindowStartOffsetMS(previousProfile)
	if now.UnixMilli()-comboWindowStartMS > comboWindowMS {
		return first
	}
	nextIndex := currentIndex + 1
	if nextIndex >= len(playerBasicAttackComboSkillIDs) {
		nextIndex = 0
	}
	return playerBasicAttackComboSkillIDs[nextIndex]
}

func basicAttackComboWindowStartOffsetMS(profile AttackProfile) int64 {
	timing := playerActionTimingForRuntime(profile, ActionTimingFromProfile(profile))
	if timing.ActionLock <= 0 {
		return 0
	}
	return timing.ActionLock.Milliseconds()
}

func basicAttackComboIndex(skillID ids.SkillID) (int, bool) {
	for index, candidate := range playerBasicAttackComboSkillIDs {
		if skillID == candidate {
			return index, true
		}
	}
	return 0, false
}

func playerActionKindForSkill(intent skill.Intent, skillID ids.SkillID) actionruntime.ActionKind {
	if skill.IsDefaultWeaponBasicAction(intent.SkillID) {
		return actionruntime.ActionKindWeaponBasic
	}
	if _, ok := basicAttackComboIndex(skillID); ok {
		return actionruntime.ActionKindWeaponBasic
	}
	return actionruntime.ActionKindActiveSkill
}

func playerActionInstanceID(source domainentity.Entity, intent skill.Intent, skillID ids.SkillID, serverActionSequence uint64) string {
	entityID := intent.EntityID
	if source != nil && source.RuntimeID().Valid() {
		entityID = source.RuntimeID()
	}
	return actionruntime.NewInstanceID(entityID, skillID.String(), intent.CommandID, intent.Sequence, serverActionSequence)
}

func actionCombatCastSkill(profile AttackProfile, intent skill.Intent) *apeironv1.Skill {
	if profile.Skill == nil {
		return nil
	}
	if intent.HasTarget || intent.HasPosition || !isActionResolvableProfile(profile) {
		return profile.Skill
	}
	clone := *profile.Skill
	clone.RequiresTarget = false
	return &clone
}

func isActionResolvableProfile(profile AttackProfile) bool {
	if len(profile.Hitboxes) > 0 || profile.Projectile != nil || profile.Area != nil {
		return true
	}
	skillType := profile.Skill.GetSkillType()
	return skillType == "melee_attack" || skillType == "ranged_attack" || skillType == "action_skill"
}

func (s *RegionPlayerSkillCombatSystem) castQueued(ctx context.Context, region *regionruntime.RegionRuntime, source domainentity.Entity, index spatial.SpatialIndex, resolver hitbox.EntityResolver, now time.Time, tick uint64) ([]AttackOutcome, error) {
	if s == nil || source == nil {
		return nil, nil
	}
	entityID := source.RuntimeID()
	state, ok := s.actionStates[entityID]
	if !ok || !state.HasQueued || state.Queued.ExecuteAt.IsZero() || now.Before(state.Queued.ExecuteAt) {
		return nil, nil
	}
	if s.remainingActionLock(source.RuntimeID(), now) > 0 || s.remainingGlobalLock(source.RuntimeID(), now) > 0 {
		return nil, nil
	}
	queued := state.Queued
	state.Queued = queuedPlayerSkillAction{}
	state.HasQueued = false
	s.actionStates[entityID] = state
	return s.cast(ctx, region, source, queued.Intent, index, resolver, now, tick)
}

func (s *RegionPlayerSkillCombatSystem) RefreshEntityLocks(entity domainentity.Entity, now time.Time) {
	if s == nil || entity == nil {
		return
	}
	if now.IsZero() {
		now = time.Now()
	}
	s.releaseExpiredActionState(entity, now)
	s.releaseExpiredMovementLock(entity, now)
}

func (s *RegionPlayerSkillCombatSystem) remainingActionLock(entityID ids.RuntimeEntityID, now time.Time) time.Duration {
	if s == nil || !entityID.Valid() {
		return 0
	}
	state, ok := s.actionStates[entityID]
	if !ok {
		return 0
	}
	return state.Instance.LockRemaining(now)
}

func (s *RegionPlayerSkillCombatSystem) remainingGlobalLock(entityID ids.RuntimeEntityID, now time.Time) time.Duration {
	if s == nil || !entityID.Valid() {
		return 0
	}
	state, ok := s.actionStates[entityID]
	if !ok {
		return 0
	}
	return state.Instance.GlobalCooldownRemaining(now)
}

func (s *RegionPlayerSkillCombatSystem) startActionTiming(source domainentity.Entity, skillID ids.SkillID, intent skill.Intent, timing ActionTimingConfig, cooldown time.Duration, movementContract movement.MovementActionContract, hasMovementContract bool, now time.Time, tick uint64) {
	if source == nil {
		return
	}
	s.ensurePlayerActionStates()
	entityID := source.RuntimeID()
	state := s.actionStates[entityID]
	state.Instance = actionruntime.NewInstance(actionruntime.NewInstanceSpec{
		EntityID:             entityID,
		ActorKind:            actionruntime.ActorKindPlayer,
		ActionKind:           playerActionKindForSkill(intent, skillID),
		SkillID:              skillID,
		CommandID:            intent.CommandID,
		CommandSequence:      intent.Sequence,
		ServerActionSequence: tick,
		ClientTick:           intent.ClientTick,
		StartedAt:            now,
		Timing:               actionRuntimeTimingFromConfig(timing),
		Cooldown:             cooldown,
		MovementContract:     movementContract,
		HasMovementContract:  hasMovementContract,
		ActionStartPosition:  source.Position(),
	})
	state.RangeCM = playerActionThreatRange(s.profileForSkill(context.Background(), skillID), timing)
	state.HasQueued = false
	state.Queued = queuedPlayerSkillAction{}
	if lockFor := movementLockDuration(timing); lockFor > 0 {
		state.MovementLockedUntil = now.Add(lockFor)
		components := source.Components()
		components.Movement.MovementLocked = true
	} else {
		state.MovementLockedUntil = time.Time{}
	}
	s.actionStates[entityID] = state
	s.recordActionRuntimeEvent(actionRuntimeEventFromInstance(ActionRuntimeEventInstanceCreated, state.Instance, now, "player_action_started"))
	if hasMovementContract {
		s.recordActionRuntimeEvent(actionRuntimeEventFromInstance(ActionRuntimeEventMovementContractResolved, state.Instance, now, "player_action_started"))
	}
	components := source.Components()
	components.Skills.CurrentSkillID = skillID
	components.Skills.State = "active"
	components.Skills.StartedAtMS = now.UnixMilli()
	components.Skills.CooldownEndMS = now.Add(cooldown).UnixMilli()
	components.Skills.LastResolvedAtMS = now.Add(timing.ActiveStart).UnixMilli()
}

func (s *RegionPlayerSkillCombatSystem) ensurePlayerActionStates() {
	if s.actionStates == nil {
		s.actionStates = make(map[ids.RuntimeEntityID]playerActionRuntimeState)
	}
}

func (s *RegionPlayerSkillCombatSystem) pendingPlayerAction(entityID ids.RuntimeEntityID) (pendingPlayerSkillAction, bool) {
	if s == nil || !entityID.Valid() {
		return pendingPlayerSkillAction{}, false
	}
	state, ok := s.actionStates[entityID]
	if !ok || !state.HasExecution {
		return pendingPlayerSkillAction{}, false
	}
	return state.Execution, true
}

func (s *RegionPlayerSkillCombatSystem) setPendingPlayerAction(entityID ids.RuntimeEntityID, pending pendingPlayerSkillAction) {
	if s == nil || !entityID.Valid() {
		return
	}
	s.ensurePlayerActionStates()
	state := s.actionStates[entityID]
	state.Execution = pending
	state.HasExecution = true
	s.actionStates[entityID] = state
}

func (s *RegionPlayerSkillCombatSystem) clearPendingPlayerAction(entityID ids.RuntimeEntityID) {
	if s == nil || !entityID.Valid() {
		return
	}
	state, ok := s.actionStates[entityID]
	if !ok {
		return
	}
	state.Execution = playerActionExecutionState{}
	state.HasExecution = false
	s.actionStates[entityID] = state
}

func (s *RegionPlayerSkillCombatSystem) prunePlayerActionState(entityID ids.RuntimeEntityID, now time.Time) {
	if s == nil || !entityID.Valid() {
		return
	}
	state, ok := s.actionStates[entityID]
	if !ok {
		return
	}
	if state.HasExecution || state.HasQueued || !state.MovementLockedUntil.IsZero() {
		return
	}
	if state.Instance.LockRemaining(now) > 0 || state.Instance.GlobalCooldownRemaining(now) > 0 {
		return
	}
	delete(s.actionStates, entityID)
}

func (s *RegionPlayerSkillCombatSystem) queueAction(entityID ids.RuntimeEntityID, intent skill.Intent, executeAt time.Time, queuedAt time.Time) {
	if s == nil || !entityID.Valid() || executeAt.IsZero() {
		return
	}
	s.ensurePlayerActionStates()
	state := s.actionStates[entityID]
	state.Queued = queuedPlayerSkillAction{Intent: intent, ExecuteAt: executeAt, QueuedAt: queuedAt}
	state.HasQueued = true
	s.actionStates[entityID] = state
}

func canQueueAction(timing ActionTimingConfig, remaining time.Duration) bool {
	return actionruntime.CanQueueTiming(actionRuntimeTimingFromConfig(timing), remaining)
}

func (s *RegionPlayerSkillCombatSystem) canQueueDuringCurrentAction(entityID ids.RuntimeEntityID, incomingTiming ActionTimingConfig, now time.Time, remaining time.Duration) bool {
	if s == nil || remaining <= 0 {
		return false
	}
	state, ok := s.actionStates[entityID]
	if !ok || state.Instance.StartedAt.IsZero() {
		return canQueueAction(incomingTiming, remaining)
	}
	return canQueueFromCurrentAction(state.Instance, now, remaining)
}

func canQueueFromCurrentAction(current actionruntime.Instance, now time.Time, remaining time.Duration) bool {
	if remaining <= 0 {
		return false
	}
	timing := current.Timing
	if actionruntime.CanQueueTiming(timing, remaining) {
		return true
	}
	return actionruntime.CanCancelIntoTiming(timing, current.StartedAt, now)
}

func movementLockDuration(timing ActionTimingConfig) time.Duration {
	switch timing.MovementLockPolicy {
	case movementLockDuringWindup:
		return timing.Windup
	case movementLockDuringActive:
		return timing.ActiveEnd
	case movementLockUntilRecoveryEnd, movementLockFullAction:
		return timing.ActionLock
	default:
		return 0
	}
}

func (s *RegionPlayerSkillCombatSystem) releaseExpiredMovementLocks(entities []domainentity.Entity, now time.Time) {
	if len(s.actionStates) == 0 {
		return
	}
	for _, entity := range entities {
		if entity == nil {
			continue
		}
		s.releaseExpiredMovementLock(entity, now)
	}
}

func (s *RegionPlayerSkillCombatSystem) releaseExpiredMovementLock(entity domainentity.Entity, now time.Time) {
	if s == nil || entity == nil || len(s.actionStates) == 0 {
		return
	}
	entityID := entity.RuntimeID()
	state, ok := s.actionStates[entityID]
	if !ok || state.MovementLockedUntil.IsZero() || now.Before(state.MovementLockedUntil) {
		return
	}
	components := entity.Components()
	components.Movement.MovementLocked = false
	state.MovementLockedUntil = time.Time{}
	s.actionStates[entityID] = state
	s.prunePlayerActionState(entityID, now)
}

func (s *RegionPlayerSkillCombatSystem) releaseExpiredActionStates(entities []domainentity.Entity, now time.Time) {
	if len(s.actionStates) == 0 {
		return
	}
	for _, entity := range entities {
		if entity == nil {
			continue
		}
		s.releaseExpiredActionState(entity, now)
	}
}

func (s *RegionPlayerSkillCombatSystem) releaseExpiredActionState(entity domainentity.Entity, now time.Time) {
	if s == nil || entity == nil || len(s.actionStates) == 0 {
		return
	}
	entityID := entity.RuntimeID()
	state, ok := s.actionStates[entityID]
	if !ok || state.Instance.LockRemaining(now) > 0 {
		return
	}
	s.updatePlayerActionPhase(entityID, now, "release_expired_action_state")
	components := entity.Components()
	switch components.Skills.State {
	case "windup", "active", "recovery", "interrupted":
		components.Skills.State = "idle"
	}
	s.prunePlayerActionState(entityID, now)
}

func (s *RegionPlayerSkillCombatSystem) applySourceDefenseReaction(source domainentity.Entity, result DamageResult, now time.Time) (string, int64) {
	if s == nil || source == nil || !result.Parried || s.Damage == nil || s.Damage.Defense == nil {
		return "", 0
	}
	state := s.Damage.Defense.State(source.RuntimeID(), now)
	if !state.RiposteVulnerableUntil.After(now) {
		return "", 0
	}
	sourceID := source.RuntimeID()
	s.ensurePlayerActionStates()
	runtimeState := s.actionStates[sourceID]
	runtimeState.Instance = actionruntime.NewInstance(actionruntime.NewInstanceSpec{
		EntityID:   sourceID,
		ActorKind:  actionruntime.ActorKindPlayer,
		ActionKind: actionruntime.ActionKindStatusControl,
		StartedAt:  now,
		Timing: actionruntime.Timing{
			ActionLock:     state.RiposteVulnerableUntil.Sub(now),
			GlobalCooldown: state.RiposteVulnerableUntil.Sub(now),
		},
	})
	runtimeState.MovementLockedUntil = state.RiposteVulnerableUntil
	s.actionStates[sourceID] = runtimeState
	s.recordActionRuntimeEvent(actionRuntimeEventFromInstance(ActionRuntimeEventInstanceCreated, runtimeState.Instance, now, "parry_riposte_vulnerable"))
	components := source.Components()
	components.Movement.MovementLocked = true
	components.Skills.State = "interrupted"
	components.Skills.CurrentSkillID = ""
	components.Skills.LastResolvedAtMS = state.RiposteVulnerableUntil.UnixMilli()
	return "parry_interrupted_riposte_vulnerable", state.RiposteVulnerableUntil.UnixMilli()
}

func playerSkillCooldown(profile AttackProfile, timing ActionTimingConfig) time.Duration {
	if !playerSkillUsesCooldown(profile) {
		return 0
	}
	if profile.Cooldown > 0 {
		return profile.Cooldown
	}
	if profile.Skill != nil && profile.Skill.GetCooldownMs() > 0 {
		return time.Duration(profile.Skill.GetCooldownMs()) * time.Millisecond
	}
	return timing.GlobalCooldown
}

func playerSkillUsesCooldown(profile AttackProfile) bool {
	if profile.Skill == nil {
		return true
	}
	if profile.Skill.GetComboGroup() == playerBasicAttackComboGroup {
		return false
	}
	skillID := ids.SkillID(profile.Skill.GetId())
	if skill.IsDefaultWeaponBasicAction(skillID) {
		return false
	}
	_, isBasicComboStep := basicAttackComboIndex(skillID)
	return !isBasicComboStep
}

func (s *RegionPlayerSkillCombatSystem) clearNonCooldownPlayerSkill(source domainentity.Entity, profile AttackProfile) {
	if s == nil || source == nil || playerSkillUsesCooldown(profile) || profile.Skill == nil || s.CastPipeline.Cooldowns == nil {
		return
	}
	s.CastPipeline.Cooldowns.Clear(source.RuntimeID(), ids.SkillID(profile.Skill.GetId()))
}

func maxDuration(a, b time.Duration) time.Duration {
	if b > a {
		return b
	}
	return a
}
