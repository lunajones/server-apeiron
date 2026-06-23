package damagegroup

import (
	"testing"
	"time"
)

func TestRuntimeResolvesWhenTickCrossesWindow(t *testing.T) {
	runtime := NewRuntime[string]()
	startedAt := time.UnixMilli(1000)
	if !runtime.Enqueue(Schedule[string]{
		Key:         "actor:skill:instance",
		StartedAt:   startedAt,
		RequireTime: true,
		Windows:     []Window{{StartMS: 120, EndMS: 160}},
		Payload:     "slash",
	}) {
		t.Fatal("failed to enqueue schedule")
	}

	var resolvedAt float64
	resolved := runtime.Run(startedAt.Add(200*time.Millisecond), nil, func(schedule Schedule[string]) bool {
		resolvedAt = schedule.ElapsedMS
		return true
	})
	if len(resolved) != 1 {
		t.Fatalf("resolved schedules = %d, want 1", len(resolved))
	}
	if resolvedAt != 160 {
		t.Fatalf("evaluation ms = %.1f, want clamped window end 160", resolvedAt)
	}
	if runtime.PendingCount() != 0 {
		t.Fatalf("pending count after resolve = %d", runtime.PendingCount())
	}
	if !runtime.IsResolved("actor:skill:instance") {
		t.Fatal("resolved key was not remembered for dedupe")
	}
}

func TestRuntimeKeepsUnhitScheduleUntilWindowExpires(t *testing.T) {
	runtime := NewRuntime[string]()
	startedAt := time.UnixMilli(1000)
	runtime.Enqueue(Schedule[string]{
		Key:         "actor:skill:instance",
		StartedAt:   startedAt,
		RequireTime: true,
		Windows:     []Window{{StartMS: 120, EndMS: 160}},
		Payload:     "slash",
	})

	resolved := runtime.Run(startedAt.Add(130*time.Millisecond), nil, func(Schedule[string]) bool {
		return false
	})
	if len(resolved) != 0 {
		t.Fatalf("resolved schedules = %d, want 0", len(resolved))
	}
	if runtime.PendingCount() != 1 {
		t.Fatalf("pending count after missed hit = %d, want 1", runtime.PendingCount())
	}

	runtime.Run(startedAt.Add(200*time.Millisecond), nil, func(Schedule[string]) bool {
		return false
	})
	if runtime.PendingCount() != 0 {
		t.Fatalf("expired missed schedule still pending: %d", runtime.PendingCount())
	}
}

func TestRuntimeRejectsDuplicateAndResolvedKeys(t *testing.T) {
	runtime := NewRuntime[string]()
	startedAt := time.UnixMilli(1000)
	schedule := Schedule[string]{
		Key:         "actor:skill:instance",
		StartedAt:   startedAt,
		RequireTime: true,
		Windows:     []Window{{StartMS: 0, EndMS: 30}},
		Payload:     "slash",
	}
	if !runtime.Enqueue(schedule) {
		t.Fatal("first enqueue rejected")
	}
	if runtime.Enqueue(schedule) {
		t.Fatal("duplicate pending enqueue accepted")
	}
	runtime.Run(startedAt.Add(10*time.Millisecond), nil, func(Schedule[string]) bool {
		return true
	})
	if runtime.Enqueue(schedule) {
		t.Fatal("enqueue accepted after key resolved")
	}
}

func TestRuntimeCancelRemovesPendingAndBlocksReenqueue(t *testing.T) {
	runtime := NewRuntime[string]()
	startedAt := time.UnixMilli(1000)
	schedule := Schedule[string]{
		Key:         "actor:skill:interrupted-instance",
		StartedAt:   startedAt,
		RequireTime: true,
		Windows:     []Window{{StartMS: 100, EndMS: 200}},
		Payload:     "lunge",
	}
	if !runtime.Enqueue(schedule) {
		t.Fatal("enqueue rejected")
	}
	if !runtime.Cancel(schedule.Key) {
		t.Fatal("cancel did not report pending schedule")
	}
	if runtime.PendingCount() != 0 {
		t.Fatalf("pending count after cancel = %d", runtime.PendingCount())
	}
	if !runtime.IsResolved(schedule.Key) {
		t.Fatal("canceled key was not remembered as resolved")
	}
	if runtime.Enqueue(schedule) {
		t.Fatal("canceled key was accepted again")
	}
	resolved := runtime.Run(startedAt.Add(150*time.Millisecond), nil, func(Schedule[string]) bool {
		t.Fatal("canceled schedule should not resolve")
		return true
	})
	if len(resolved) != 0 {
		t.Fatalf("resolved canceled schedules = %d", len(resolved))
	}
}

func TestRuntimeRefreshCanUpdatePayloadBeforeResolve(t *testing.T) {
	runtime := NewRuntime[string]()
	startedAt := time.UnixMilli(1000)
	runtime.Enqueue(Schedule[string]{
		Key:         "actor:skill:instance",
		StartedAt:   startedAt,
		RequireTime: true,
		Windows:     []Window{{StartMS: 10, EndMS: 30}},
		Payload:     "stale",
	})

	var gotPayload string
	runtime.Run(startedAt.Add(12*time.Millisecond), func(schedule Schedule[string]) Schedule[string] {
		schedule.Payload = "fresh"
		return schedule
	}, func(schedule Schedule[string]) bool {
		gotPayload = schedule.Payload
		return true
	})
	if gotPayload != "fresh" {
		t.Fatalf("resolved payload = %q, want fresh", gotPayload)
	}
}
