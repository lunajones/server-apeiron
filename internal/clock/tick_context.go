package clock

import (
	"context"
	"time"
)

type TickContext struct {
	Context context.Context
	Tick    uint64
	Now     time.Time
	Delta   time.Duration
	Phase   TickPhase
}
