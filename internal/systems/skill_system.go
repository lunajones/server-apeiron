package systems

import "server-apeiron/internal/clock"

func NewSkillSystem(fn func(Context) error) System {
	return NewSystemFunc("skill", clock.PhaseCombat, fn)
}
