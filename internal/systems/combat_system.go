package systems

import "server-apeiron/internal/clock"

func NewCombatSystem(fn func(Context) error) System {
	return NewSystemFunc("combat", clock.PhaseCombat, fn)
}
