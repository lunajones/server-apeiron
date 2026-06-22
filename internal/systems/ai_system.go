package systems

import "server-apeiron/internal/clock"

func NewAISystem(fn func(Context) error) System {
	return NewSystemFunc("ai", clock.PhaseAI, fn)
}
