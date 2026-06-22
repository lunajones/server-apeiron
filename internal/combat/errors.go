package combat

import "errors"

var (
	ErrSourceRequired = errors.New("damage source is required")
	ErrTargetRequired = errors.New("damage target is required")
	ErrSkillRequired  = errors.New("skill is required")
	ErrInvalidTarget  = errors.New("target cannot be damaged")
	ErrPvPRejected    = errors.New("pvp validator rejected damage")
)
