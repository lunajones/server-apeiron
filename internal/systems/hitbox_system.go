package systems

import "server-apeiron/internal/clock"

func NewHitboxSystem(fn func(Context) error) System {
	return NewSystemFunc("hitbox", clock.PhaseCombat, fn)
}
