package skill

import (
	"context"
	"errors"
	"time"

	apeironv1 "db-apeiron/gen/apeiron/v1"
	"server-apeiron/internal/domain/ids"
	domainmath "server-apeiron/internal/domain/math"
)

var (
	ErrOnCooldown         = errors.New("skill on cooldown")
	ErrInsufficientMana   = errors.New("insufficient mana")
	ErrInsufficientHealth = errors.New("insufficient health")
	ErrTargetRequired     = errors.New("target required")
)

const (
	IntentCast                 = "cast"
	DefaultWeaponBasicActionID = ids.SkillID("weapon_basic")
)

type Intent struct {
	Action         string
	EntityID       ids.RuntimeEntityID
	SkillID        ids.SkillID
	CommandID      string
	Sequence       uint64
	Target         TargetRef
	HasTarget      bool
	TargetID       ids.RuntimeEntityID
	TargetPosition domainmath.Position
	HasPosition    bool
	AimDirection   domainmath.Vec3
	HasAim         bool
	ClientTick     uint64
	ReceivedAt     time.Time
	SubmittedAt    time.Time
}

type TargetRef struct {
	RuntimeID ids.RuntimeEntityID
}

func IsDefaultWeaponBasicAction(skillID ids.SkillID) bool {
	return skillID == "" || skillID == DefaultWeaponBasicActionID || skillID == "basic_attack"
}

type CastContext struct {
	Context        context.Context
	Skill          *apeironv1.Skill
	Intent         Intent
	Now            time.Time
	Caster         any
	SkillID        ids.SkillID
	Target         TargetRef
	HasTarget      bool
	TargetPosition domainmath.Position
	HasPosition    bool
	AimDirection   domainmath.Vec3
	HasAim         bool
	ClientTick     uint64
	RequestedAt    time.Time
}

type PipelineConfig struct{}

type CooldownTracker struct {
	until map[ids.RuntimeEntityID]map[ids.SkillID]time.Time
}

func NewCastPipeline(any, any, PipelineConfig) CastPipeline {
	return CastPipeline{Cooldowns: NewCooldownTracker()}
}

type CastPipeline struct {
	Cooldowns *CooldownTracker
}

func NewCooldownTracker() *CooldownTracker {
	return &CooldownTracker{until: make(map[ids.RuntimeEntityID]map[ids.SkillID]time.Time)}
}

func (c *CooldownTracker) Remaining(entityID ids.RuntimeEntityID, skillID ids.SkillID, now time.Time) time.Duration {
	if c == nil {
		return 0
	}
	until := c.until[entityID][skillID]
	if until.After(now) {
		return until.Sub(now)
	}
	return 0
}

func (c *CooldownTracker) Put(entityID ids.RuntimeEntityID, skillID ids.SkillID, until time.Time) {
	if c == nil || !entityID.Valid() || skillID == "" || until.IsZero() {
		return
	}
	if c.until == nil {
		c.until = make(map[ids.RuntimeEntityID]map[ids.SkillID]time.Time)
	}
	if c.until[entityID] == nil {
		c.until[entityID] = make(map[ids.SkillID]time.Time)
	}
	c.until[entityID][skillID] = until
}

func (c *CooldownTracker) Clear(entityID ids.RuntimeEntityID, skillID ids.SkillID) {
	if c == nil || c.until == nil {
		return
	}
	if skills := c.until[entityID]; skills != nil {
		delete(skills, skillID)
	}
}

func (p CastPipeline) Validate(context.Context, CastContext, *apeironv1.Skill, time.Time) error {
	return nil
}

func (p CastPipeline) Commit(ctx context.Context, cast CastContext, castSkill *apeironv1.Skill, cooldownUntil time.Time, now time.Time) error {
	if p.Cooldowns != nil && cast.SkillID != "" {
		if caster, ok := cast.Caster.(interface{ RuntimeID() ids.RuntimeEntityID }); ok {
			p.Cooldowns.Put(caster.RuntimeID(), cast.SkillID, cooldownUntil)
		}
	}
	_ = ctx
	_ = castSkill
	_ = now
	return nil
}
