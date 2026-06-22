package systems

import "server-apeiron/internal/clock"

func NewSnapshotSystem(fn func(Context) error) System {
	return NewSystemFunc("snapshot", clock.PhaseSnapshot, fn)
}
