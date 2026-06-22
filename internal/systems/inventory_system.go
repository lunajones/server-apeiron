package systems

import "server-apeiron/internal/clock"

func NewInventorySystem(fn func(Context) error) System {
	return NewSystemFunc("inventory", clock.PhasePersistence, fn)
}
