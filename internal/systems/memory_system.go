package systems

import "server-apeiron/internal/clock"

func NewMemorySystem(fn func(Context) error) System {
	return NewSystemFunc("memory", clock.PhaseAI, fn)
}
