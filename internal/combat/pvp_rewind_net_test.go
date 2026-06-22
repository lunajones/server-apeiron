package combat

import (
	"context"
	"math/rand"
	"sort"
	"testing"
	"time"

	apeironv1 "db-apeiron/gen/apeiron/v1"
	domainmath "server-apeiron/internal/domain/math"
	"server-apeiron/internal/hitbox"
	"server-apeiron/internal/pvp"
)

type combatNetPacket struct {
	tick         uint64
	arrivalTick  uint64
	lost         bool
	sourcePos    domainmath.Position
	targetPos    domainmath.Position
	targetFacing domainmath.Vec3
}

func TestImpactResolutionPipelinePvPRewindUnderJitterLossAndReorder(t *testing.T) {
	rng := rand.New(rand.NewSource(20260613))
	rewind := pvp.NewRewindHistory(96)
	pipeline := NewImpactResolutionPipeline(nil, nil, nil, nil)
	pipeline.Rewind = rewind
	pipeline.MaxRewindTicks = 16
	source := combatEntity(1, "red", 0, 0)
	target := combatEntity(2, "blue", 0, 0)
	target.Components().Movement.Locomotion.AuthoritativeYaw = 0
	now := time.UnixMilli(9000)

	packets := make([]combatNetPacket, 0, 48)
	for tick := uint64(1); tick <= 48; tick++ {
		lost := rng.Float64() < 0.12
		delay := uint64(2 + rng.Intn(7))
		if rng.Float64() < 0.20 {
			delay += uint64(rng.Intn(8))
		}
		packets = append(packets, combatNetPacket{
			tick:         tick,
			arrivalTick:  tick + delay,
			lost:         lost,
			sourcePos:    domainmath.V3(-100+float64(tick), 0, 0),
			targetPos:    domainmath.V3(0, 0, 0),
			targetFacing: domainmath.V3(1, 0, 0),
		})
	}
	sort.SliceStable(packets, func(i, j int) bool {
		if packets[i].arrivalTick == packets[j].arrivalTick {
			return packets[i].tick > packets[j].tick
		}
		return packets[i].arrivalTick < packets[j].arrivalTick
	})

	resolved := 0
	for _, packet := range packets {
		if packet.lost {
			continue
		}
		source.SetPosition(packet.sourcePos)
		target.SetPosition(packet.targetPos)
		rewind.Record(pvp.RewindSample{EntityID: source.RuntimeID(), Tick: packet.tick, Position: packet.sourcePos, Facing: domainmath.V3(1, 0, 0)})
		rewind.Record(pvp.RewindSample{EntityID: target.RuntimeID(), Tick: packet.tick, Position: packet.targetPos, Facing: packet.targetFacing})
		result, err := pipeline.Apply(context.Background(), DamageContext{
			Source:      source,
			Target:      target,
			Hit:         hitbox.HitResult{SkillID: "pvp_slash"},
			Skill:       &apeironv1.Skill{Id: "pvp_slash", BaseDamage: 1},
			Now:         now.Add(time.Duration(packet.arrivalTick) * time.Millisecond),
			Tick:        packet.tick,
			CurrentTick: packet.arrivalTick,
		})
		if err != nil {
			t.Fatalf("Apply error at tick %d arrival %d: %v", packet.tick, packet.arrivalTick, err)
		}
		if result.HitArc == "" {
			t.Fatalf("expected hit arc under net emulation, got %#v", result)
		}
		resolved++
	}
	if resolved < 32 {
		t.Fatalf("expected enough delivered pvp samples, got %d", resolved)
	}
}
