package clock

import "time"

type TickBudget struct {
	Phase TickPhase
	Limit time.Duration
}

func NewTickBudget(phase TickPhase, limit time.Duration) TickBudget {
	return TickBudget{Phase: phase, Limit: limit}
}

func (b TickBudget) Exceeded(elapsed time.Duration) bool {
	return b.Limit > 0 && elapsed > b.Limit
}
