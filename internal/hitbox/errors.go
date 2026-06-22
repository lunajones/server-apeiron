package hitbox

import "errors"

var (
	ErrCasterRequired   = errors.New("caster is required")
	ErrSkillRequired    = errors.New("skill is required")
	ErrSpatialRequired  = errors.New("spatial index is required")
	ErrResolverRequired = errors.New("entity resolver is required")
)
