package pvp

import (
	"sync"

	"server-apeiron/internal/domain/ids"
	domainmath "server-apeiron/internal/domain/math"
)

type RewindSample struct {
	EntityID ids.RuntimeEntityID
	Tick     uint64
	Position domainmath.Position
	Facing   domainmath.Vec3
}

type RewindQuery struct {
	EntityID       ids.RuntimeEntityID
	RequestedTick  uint64
	CurrentTick    uint64
	MaxRewindTicks uint64
}

type RewindResult struct {
	Sample        RewindSample
	Found         bool
	Clamped       bool
	EffectiveTick uint64
}

type RewindHistory struct {
	mu       sync.RWMutex
	capacity int
	byEntity map[ids.RuntimeEntityID][]RewindSample
}

func NewRewindHistory(capacity int) *RewindHistory {
	if capacity <= 0 {
		capacity = 128
	}
	return &RewindHistory{capacity: capacity, byEntity: make(map[ids.RuntimeEntityID][]RewindSample)}
}

func (h *RewindHistory) Record(sample RewindSample) {
	if h == nil || !sample.EntityID.Valid() || sample.Tick == 0 {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	samples := append(h.byEntity[sample.EntityID], sample)
	if len(samples) > h.capacity {
		samples = samples[len(samples)-h.capacity:]
	}
	h.byEntity[sample.EntityID] = samples
}

func (h *RewindHistory) Resolve(query RewindQuery) RewindResult {
	if h == nil || !query.EntityID.Valid() || query.RequestedTick == 0 {
		return RewindResult{}
	}
	effectiveTick := query.RequestedTick
	clamped := false
	if query.CurrentTick > 0 && query.MaxRewindTicks > 0 {
		oldestAllowed := uint64(1)
		if query.CurrentTick > query.MaxRewindTicks {
			oldestAllowed = query.CurrentTick - query.MaxRewindTicks
		}
		if effectiveTick < oldestAllowed {
			effectiveTick = oldestAllowed
			clamped = true
		}
		if effectiveTick > query.CurrentTick {
			effectiveTick = query.CurrentTick
			clamped = true
		}
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	samples := h.byEntity[query.EntityID]
	for i := len(samples) - 1; i >= 0; i-- {
		if samples[i].Tick <= effectiveTick {
			return RewindResult{Sample: samples[i], Found: true, Clamped: clamped, EffectiveTick: effectiveTick}
		}
	}
	return RewindResult{Found: false, Clamped: clamped, EffectiveTick: effectiveTick}
}
