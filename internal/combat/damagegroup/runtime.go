package damagegroup

import "time"

const temporalEpsilon = 0.0001

type Window struct {
	StartMS float64
	EndMS   float64
}

type Schedule[T any] struct {
	Key         string
	StartedAt   time.Time
	ElapsedMS   float64
	PreviousMS  float64
	RequireTime bool
	Windows     []Window
	Payload     T
}

type Runtime[T any] struct {
	pending  map[string]Schedule[T]
	resolved map[string]struct{}
}

type Resolver[T any] func(schedule Schedule[T]) bool
type Refresher[T any] func(schedule Schedule[T]) Schedule[T]

func NewRuntime[T any]() *Runtime[T] {
	return &Runtime[T]{
		pending:  map[string]Schedule[T]{},
		resolved: map[string]struct{}{},
	}
}

func (r *Runtime[T]) Enqueue(schedule Schedule[T]) bool {
	if r == nil || schedule.Key == "" {
		return false
	}
	r.ensure()
	if _, ok := r.resolved[schedule.Key]; ok {
		return false
	}
	if _, ok := r.pending[schedule.Key]; ok {
		return false
	}
	r.pending[schedule.Key] = schedule
	return true
}

func (r *Runtime[T]) Run(now time.Time, refresh Refresher[T], resolve Resolver[T]) []Schedule[T] {
	if r == nil || len(r.pending) == 0 || resolve == nil {
		return nil
	}
	r.ensure()
	resolved := make([]Schedule[T], 0, len(r.pending))
	for key, schedule := range r.pending {
		schedule.PreviousMS = schedule.ElapsedMS
		schedule.ElapsedMS = scheduleElapsedMS(schedule, now)
		if refresh != nil {
			schedule = refresh(schedule)
		}
		evaluationMS, crossed := scheduleEvaluationElapsedMS(schedule)
		if !crossed {
			if scheduleExpired(schedule) {
				delete(r.pending, key)
				continue
			}
			r.pending[key] = schedule
			continue
		}
		evaluated := schedule
		evaluated.ElapsedMS = evaluationMS
		if resolve(evaluated) {
			r.resolved[key] = struct{}{}
			delete(r.pending, key)
			resolved = append(resolved, evaluated)
			continue
		}
		if scheduleExpired(schedule) {
			delete(r.pending, key)
			continue
		}
		r.pending[key] = schedule
	}
	return resolved
}

func (r *Runtime[T]) IsResolved(key string) bool {
	if r == nil || key == "" {
		return false
	}
	_, ok := r.resolved[key]
	return ok
}

func (r *Runtime[T]) Cancel(key string) bool {
	if r == nil || key == "" {
		return false
	}
	r.ensure()
	_, pending := r.pending[key]
	delete(r.pending, key)
	r.resolved[key] = struct{}{}
	return pending
}

func (r *Runtime[T]) PendingCount() int {
	if r == nil {
		return 0
	}
	return len(r.pending)
}

func (r *Runtime[T]) ensure() {
	if r.pending == nil {
		r.pending = map[string]Schedule[T]{}
	}
	if r.resolved == nil {
		r.resolved = map[string]struct{}{}
	}
}

func scheduleElapsedMS[T any](schedule Schedule[T], now time.Time) float64 {
	if !schedule.StartedAt.IsZero() {
		elapsed := now.Sub(schedule.StartedAt).Seconds() * 1000
		if elapsed > 0 {
			return elapsed
		}
	}
	return schedule.ElapsedMS
}

func scheduleExpired[T any](schedule Schedule[T]) bool {
	if !schedule.RequireTime {
		return true
	}
	end, ok := latestWindowEndMS(schedule.Windows)
	if !ok {
		return true
	}
	return schedule.ElapsedMS-temporalEpsilon > end
}

func scheduleEvaluationElapsedMS[T any](schedule Schedule[T]) (float64, bool) {
	if !schedule.RequireTime {
		return schedule.ElapsedMS, true
	}
	bestStart := 0.0
	bestEnd := 0.0
	found := false
	for _, window := range schedule.Windows {
		startMS := window.StartMS
		endMS := window.EndMS
		if endMS <= startMS {
			endMS = startMS
		}
		if schedule.ElapsedMS+temporalEpsilon < startMS || schedule.PreviousMS-temporalEpsilon > endMS {
			continue
		}
		if !found || startMS < bestStart {
			bestStart = startMS
			bestEnd = endMS
			found = true
		}
	}
	if !found {
		return schedule.ElapsedMS, false
	}
	if schedule.ElapsedMS < bestStart {
		return bestStart, true
	}
	if schedule.ElapsedMS > bestEnd {
		return bestEnd, true
	}
	return schedule.ElapsedMS, true
}

func latestWindowEndMS(windows []Window) (float64, bool) {
	best := -1.0
	for _, window := range windows {
		endMS := window.EndMS
		if endMS <= window.StartMS {
			endMS = window.StartMS
		}
		if endMS > best {
			best = endMS
		}
	}
	return best, best >= 0
}
