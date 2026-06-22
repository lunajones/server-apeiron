package systems

import "server-apeiron/internal/clock"

func NewPerceptionSystem(fn func(Context) error) System {
	return NewSystemFunc("perception", clock.PhaseAI, fn)
}
