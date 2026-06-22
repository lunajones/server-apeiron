package systems

import "server-apeiron/internal/clock"

func NewNeedsSystem(fn func(Context) error) System {
	return NewSystemFunc("needs", clock.PhaseAI, fn)
}
