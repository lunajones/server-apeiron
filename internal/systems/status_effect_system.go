package systems

import "server-apeiron/internal/clock"

func NewStatusEffectSystem(fn func(Context) error) System {
	return NewSystemFunc("status_effect", clock.PhaseCombat, fn)
}
