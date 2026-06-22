package combat

import (
	"context"
	"math"
	"strings"
	"time"

	apeironv1 "db-apeiron/gen/apeiron/v1"
	"server-apeiron/internal/combat/actionruntime"
	domainentity "server-apeiron/internal/domain/entity"
	"server-apeiron/internal/domain/ids"
	domainmath "server-apeiron/internal/domain/math"
	"server-apeiron/internal/hitbox"
	"server-apeiron/internal/pvp"
	regionruntime "server-apeiron/internal/runtime/region"
)

type RegionSource interface {
	All() []*regionruntime.RegionRuntime
	Active() []*regionruntime.RegionRuntime
}

type SkillAttackProfileProvider interface {
	AttackProfile(context.Context, ids.SkillID) (AttackProfile, bool)
	ProfileForSkill(context.Context, ids.SkillID) AttackProfile
}

type AttackProfile struct {
	Skill          *apeironv1.Skill
	Hitboxes       []*apeironv1.SkillHitboxProfile
	Impact         *apeironv1.SkillImpactProfile
	Projectile     any
	Area           any
	SourceCore     *apeironv1.CombatCoreProfile
	TargetCore     *apeironv1.CombatCoreProfile
	Defense        *apeironv1.CombatDefenseContract
	Timing         *apeironv1.SkillTimingProfile
	Movement       *apeironv1.SkillMovementProfile
	Cooldown       time.Duration
	ControlEffects []ControlEffectConfig
}

type ActionTimingConfig struct {
	Windup             time.Duration
	ActiveStart        time.Duration
	ActiveEnd          time.Duration
	Recovery           time.Duration
	ActionLock         time.Duration
	GlobalCooldown     time.Duration
	MovementLockPolicy string
}

type skillMovementConfig struct {
	MovementType           string
	Distance               float64
	Speed                  float64
	DurationMS             int32
	MovementStartPhase     string
	MovementStartOffsetMS  int32
	TakeoffMS              int32
	LandingLockMS          int32
	ArcHeight              float64
	ArcCurve               string
	Bounds                 string
	SteeringPolicy         string
	MaxTurnDegPerSec       float64
	MaxTotalRedirectAngle  float64
	RedirectLockoutMS      int32
	CanPhaseThroughTargets bool
	MinLandingDistance     float64
	DesiredLandingDistance float64
	StopAtContactRatio     float64
	AppliesKnockback       bool
	KnockbackDistance      float64
	KnockbackSpeed         float64
}

type ControlEffectConfig = *apeironv1.SkillControlEffect

type DamageResult struct {
	Object                domainentity.Entity
	FinalDamage           float64
	PoiseDamage           float64
	PostureDamage         float64
	Blocked               bool
	Parried               bool
	Evaded                bool
	Killed                bool
	Staggered             bool
	StatusApplied         []string
	Reason                string
	HitArc                string
	SourceReaction        string
	SourceReactionUntilMS int64
}

type AttackOutcome struct {
	SourceID                    ids.RuntimeEntityID
	TargetID                    ids.RuntimeEntityID
	Source                      domainentity.Ref
	Target                      domainentity.Ref
	RegionID                    ids.RegionID
	HitboxID                    string
	SkillID                     ids.SkillID
	Result                      DamageResult
	MotionProfileID             string
	DamageGroupID               string
	MotionTStart                float64
	MotionTEnd                  float64
	MotionSampleStartIndex      int32
	MotionSampleEndIndex        int32
	HitQuality                  string
	HitQualitySpatialScore      float64
	HitboxDebugShape            string
	HitboxDebugCenter           domainmath.Position
	HitboxDebugExtent           domainmath.Vec3
	HitboxDebugForward          domainmath.Vec3
	HitboxDebugRight            domainmath.Vec3
	HitboxDebugUp               domainmath.Vec3
	HitboxDebugSegmentA         domainmath.Position
	HitboxDebugSegmentB         domainmath.Position
	HitboxDebugSize             domainmath.Vec3
	HitboxDebugRadius           float64
	HitboxDebugLength           float64
	HitboxDebugHeight           float64
	HitboxDebugMinAngleDeg      float64
	HitboxDebugMaxAngleDeg      float64
	DamageType                  string
	ElementalType               string
	ImpactType                  string
	TargetImpactResponseProfile string
	Tick                        uint64
	Killed                      bool
}

type HitboxDebugEvent struct {
	Source                 domainentity.Ref
	SourceID               ids.RuntimeEntityID
	TargetID               ids.RuntimeEntityID
	RegionID               ids.RegionID
	SkillID                ids.SkillID
	ActionInstanceID       string
	HitboxID               string
	HitboxIndex            int
	Shape                  string
	MotionProfileID        string
	DamageGroupID          string
	MotionTStart           float64
	MotionTEnd             float64
	MotionSampleStartIndex int32
	MotionSampleEndIndex   int32
	HitboxDebugShape       string
	HitboxDebugCenter      domainmath.Position
	HitboxDebugExtent      domainmath.Vec3
	HitboxDebugForward     domainmath.Vec3
	HitboxDebugRight       domainmath.Vec3
	HitboxDebugUp          domainmath.Vec3
	HitboxDebugSegmentA    domainmath.Position
	HitboxDebugSegmentB    domainmath.Position
	HitboxDebugSize        domainmath.Vec3
	HitboxDebugRadius      float64
	HitboxDebugLength      float64
	HitboxDebugHeight      float64
	HitboxDebugMinAngleDeg float64
	HitboxDebugMaxAngleDeg float64
	Tick                   uint64
	Metadata               map[string]string
}

type ActionRuntimeEvent struct {
	Kind             string
	ActorKind        actionruntime.ActorKind
	ActionKind       actionruntime.ActionKind
	EntityID         ids.RuntimeEntityID
	SkillID          ids.SkillID
	ActionInstanceID string
	At               time.Time
	Reason           string
}

type ActionRuntimeCounters struct {
	Fallbacks        int
	MissingContracts int
}

type DefenseRuntime struct {
	states map[ids.RuntimeEntityID]DefenseState
}

type DefenseState struct {
	RiposteVulnerableUntil time.Time
}

func NewDefenseRuntime() *DefenseRuntime {
	return &DefenseRuntime{states: make(map[ids.RuntimeEntityID]DefenseState)}
}

func (d *DefenseRuntime) State(entityID ids.RuntimeEntityID, now time.Time) DefenseState {
	if d == nil {
		return DefenseState{}
	}
	return d.states[entityID]
}

type ImpactResolutionPipeline struct {
	Defense        *DefenseRuntime
	Status         *StatusRuntime
	Policies       *CombatPolicyRuntime
	Validator      *pvp.Validator
	Rewind         *pvp.RewindHistory
	MaxRewindTicks uint64
}

type DamageContext struct {
	Source         domainentity.Entity
	Target         domainentity.Entity
	Hit            hitbox.HitResult
	Skill          *apeironv1.Skill
	Impact         *apeironv1.SkillImpactProfile
	ControlEffects []ControlEffectConfig
	SourceCore     *apeironv1.CombatCoreProfile
	TargetCore     *apeironv1.CombatCoreProfile
	Defense        *apeironv1.CombatDefenseContract
	Now            time.Time
	Tick           uint64
	CurrentTick    uint64
}

func NewImpactResolutionPipeline(defense *DefenseRuntime, status *StatusRuntime, validator *pvp.Validator, policies *CombatPolicyRuntime) *ImpactResolutionPipeline {
	if defense == nil {
		defense = NewDefenseRuntime()
	}
	if status == nil {
		status = NewStatusRuntime()
	}
	if validator == nil {
		validator = pvp.NewValidator()
	}
	if policies == nil {
		policies = NewCombatPolicyRuntime()
	}
	return &ImpactResolutionPipeline{Defense: defense, Status: status, Validator: validator, Policies: policies, MaxRewindTicks: 16}
}

func (p *ImpactResolutionPipeline) Apply(ctx context.Context, damage DamageContext) (DamageResult, error) {
	if damage.Source == nil {
		return DamageResult{}, ErrSourceRequired
	}
	if damage.Target == nil {
		return DamageResult{}, ErrTargetRequired
	}
	if damage.Skill == nil {
		return DamageResult{}, ErrSkillRequired
	}
	if damage.Source.RuntimeID() == damage.Target.RuntimeID() {
		return DamageResult{}, ErrInvalidTarget
	}
	if p != nil && p.Validator != nil && !p.Validator.ValidateHit(damage.Source, damage.Target, damage.Hit, damage.CurrentTick) {
		return DamageResult{}, ErrPvPRejected
	}
	baseDamage := damage.Skill.GetBaseDamage()
	if damage.SourceCore != nil {
		baseDamage *= firstPositiveCombat(damage.SourceCore.GetDamageDealtMultiplier(), 1)
	}
	if damage.TargetCore != nil {
		baseDamage *= firstPositiveCombat(damage.TargetCore.GetDamageTakenMultiplier(), 1)
	}
	result := DamageResult{
		Object:      damage.Target,
		FinalDamage: baseDamage,
		HitArc:      "authoritative",
		Reason:      "hit",
	}
	if damage.Impact != nil {
		result.PoiseDamage = damage.Impact.GetPoiseDamage()
		if damage.TargetCore != nil {
			result.PostureDamage = result.PoiseDamage * firstPositiveCombat(damage.TargetCore.GetPostureDamageMultiplier(), 1)
		} else {
			result.PostureDamage = result.PoiseDamage
		}
	}
	if defensiveState := currentDefensiveState(damage.Target, damage.Now); defensiveState != "" {
		if defensiveState == "iframe" || defensiveState == "evade" || defensiveState == "dodge" {
			result.Evaded = true
			result.FinalDamage = 0
			result.PoiseDamage = 0
			result.PostureDamage = 0
			result.Reason = "evaded"
			_ = ctx
			return result, nil
		}
		if damage.Skill.GetIsParryable() && (defensiveState == "parry" || defensiveState == "parry_active") {
			result.Parried = true
			result.FinalDamage = 0
			result.PoiseDamage = 0
			result.PostureDamage = 0
			result.Reason = "parried"
			if p != nil && p.Defense != nil {
				p.Defense.markRiposteVulnerable(damage.Source.RuntimeID(), damage.Now, damage.Defense)
			}
			_ = ctx
			return result, nil
		}
		if damage.Skill.GetIsBlockable() && defensiveState == "blocking" && damage.TargetCore.GetCanBlock() && hitInsideDefenseArc(damage.Source, damage.Target, damage.Defense) {
			result.Blocked = true
			result.Reason = "blocked"
			reduction := clamp(damage.TargetCore.GetBlockDamageReduction(), 0, 1)
			result.FinalDamage *= 1 - reduction
			if damage.Impact != nil {
				result.PostureDamage = result.PoiseDamage * firstPositiveCombat(damage.Impact.GetGuardDamageMultiplier(), 1)
				if damage.TargetCore != nil {
					result.PostureDamage *= firstPositiveCombat(damage.TargetCore.GetPostureDamageMultiplier(), 1)
				}
			}
			if damage.Defense != nil && !damage.Defense.GetPostureDamageOnBlock() {
				result.PostureDamage = 0
			}
			if damage.Defense != nil && damage.Defense.GetStaminaDamageOnlyOnBlock() {
				result.HitArc = damage.Defense.GetId()
			}
		}
	}
	if p != nil && p.Status != nil {
		for _, control := range damage.ControlEffects {
			effect := statusEffectFromSkillControlEffect(control)
			if effect != nil && p.Status.ApplyControl(damage.Source, damage.Target, effect, control, damage.Now) {
				result.StatusApplied = append(result.StatusApplied, effect.GetId())
			}
		}
	}
	_ = ctx
	return result, nil
}

func (p *ImpactResolutionPipeline) Resolve(context.Context, domainentity.Entity, domainentity.Entity, AttackProfile, domainmath.Position, time.Time) DamageResult {
	return DamageResult{}
}

type StatusRuntime struct{}

func NewStatusRuntime() *StatusRuntime {
	return &StatusRuntime{}
}

func (s *StatusRuntime) Apply(domainentity.Entity, *apeironv1.StatusEffect, time.Time) bool {
	return true
}

func (s *StatusRuntime) ApplyControl(domainentity.Entity, domainentity.Entity, *apeironv1.StatusEffect, *apeironv1.SkillControlEffect, time.Time) bool {
	return true
}

type CombatPolicyRuntime struct{}

type SkillActionWindow struct {
	Enabled         bool
	StartMS         int32
	EndMS           int32
	WindowType      string
	PoisePolicy     string
	InterruptPolicy string
}

func NewCombatPolicyRuntime() *CombatPolicyRuntime {
	return &CombatPolicyRuntime{}
}

func (p *CombatPolicyRuntime) ResolveSkillActionWindows(skillID string) []SkillActionWindow {
	return nil
}

func firstPositiveCombat(values ...float64) float64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func (d *DefenseRuntime) markRiposteVulnerable(entityID ids.RuntimeEntityID, now time.Time, contract *apeironv1.CombatDefenseContract) {
	if d == nil || !entityID.Valid() || contract == nil {
		return
	}
	if d.states == nil {
		d.states = make(map[ids.RuntimeEntityID]DefenseState)
	}
	window := time.Duration(contract.GetParryWindowMs()) * time.Millisecond
	if window <= 0 {
		window = time.Duration(contract.GetPerfectBlockWindowMs()) * time.Millisecond
	}
	if window <= 0 {
		return
	}
	state := d.states[entityID]
	state.RiposteVulnerableUntil = now.Add(window)
	d.states[entityID] = state
}

func currentDefensiveState(entity domainentity.Entity, now time.Time) string {
	if entity == nil || entity.Components() == nil {
		return ""
	}
	components := entity.Components()
	if components.Combat.ControlImmuneUntil.After(now) {
		return "iframe"
	}
	if components.Status.Effects != nil {
		for effectID, until := range components.Status.Effects {
			if !until.After(now) {
				continue
			}
			normalized := strings.ToLower(strings.TrimSpace(effectID))
			if normalized == "iframe" || normalized == "evade" || normalized == "dodge_iframe" || strings.Contains(normalized, "iframe") {
				return "iframe"
			}
		}
	}
	state := strings.ToLower(strings.TrimSpace(components.Skills.State))
	switch state {
	case "blocking", "block", "guard":
		return "blocking"
	case "parry", "parry_active", "perfect_block":
		return "parry"
	case "iframe", "evade", "dodge":
		return "iframe"
	default:
		return ""
	}
}

func hitInsideDefenseArc(source domainentity.Entity, target domainentity.Entity, contract *apeironv1.CombatDefenseContract) bool {
	if source == nil || target == nil || contract == nil {
		return false
	}
	arc := contract.GetFrontalArcDeg()
	if arc <= 0 {
		return false
	}
	if arc >= 360 {
		return true
	}
	targetFacing := target.Facing().Normalize()
	if targetFacing.IsZero() {
		targetFacing = yawToDirection(target.Components().Movement.Locomotion.AuthoritativeYaw)
	}
	toSource := source.Position().Sub(target.Position()).Normalize()
	if targetFacing.IsZero() || toSource.IsZero() {
		return false
	}
	minDot := math.Cos((arc * 0.5) * math.Pi / 180)
	return targetFacing.Dot(toSource) >= minDot
}
