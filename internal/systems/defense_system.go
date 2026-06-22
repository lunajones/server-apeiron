package systems

import "server-apeiron/internal/clock"

func NewDefenseSystem(fn func(Context) error) System {
	return NewSystemFunc("defense", clock.PhaseCombat, fn)
}
