package systems

import "server-apeiron/internal/clock"

func NewPersistenceSystem(fn func(Context) error) System {
	return NewSystemFunc("persistence", clock.PhasePersistence, fn)
}
