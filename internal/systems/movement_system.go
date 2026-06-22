package systems

import "server-apeiron/internal/clock"

func NewMovementSystem(fn func(Context) error) System {
	return NewSystemFunc("movement", clock.PhaseMovement, fn)
}
