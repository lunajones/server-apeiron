package systems

import "server-apeiron/internal/clock"

func NewInputCommandsSystem(fn func(Context) error) System {
	return NewSystemFunc("input_commands", clock.PhaseInput, fn)
}
