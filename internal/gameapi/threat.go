package gameapi

import "time"

// threatTable is a creature's decaying per-target threat memory: who is hurting/pressuring it,
// how much, read for target selection. It is behavior memory, never movement authority.
// See docs/roadmap/aaa-threat-aggro-runtime-roadmap.md. Slice 1 populates Entries (emission +
// decay/prune); CurrentTarget/LastSwitchAt (selection) and AnchorPos (leash) land in later slices.
type threatTable struct {
	Entries       map[uint64]float64
	CurrentTarget uint64
	LastSwitchAt  time.Time
	AnchorPos     vector
	AnchorKnown   bool
}

func (e *entityState) ensureThreatTable() *threatTable {
	if e.threat == nil {
		e.threat = &threatTable{Entries: map[uint64]float64{}}
	} else if e.threat.Entries == nil {
		e.threat.Entries = map[uint64]float64{}
	}
	return e.threat
}

// creditThreatLocked adds threat to a creature's table for an attacker that just applied
// damage/posture to it. Only creatures keep tables (threat is "who is hurting me"); the first
// attacker on a fresh table earns the puller bonus.
func (r *Runtime) creditThreatLocked(creature *entityState, attacker *entityState, damage, posture float64) {
	if r == nil || creature == nil || attacker == nil {
		return
	}
	if creature.entityType != "creature" || creature.id == attacker.id {
		return
	}
	profile := r.creatureThreatProfile(creature)
	gain := damage*profile.DamageThreatPerPoint + posture*profile.PostureThreatPerPoint
	if gain <= 0 {
		return
	}
	table := creature.ensureThreatTable()
	if len(table.Entries) == 0 && profile.FirstAggroBonus > 0 {
		gain += profile.FirstAggroBonus
	}
	table.Entries[attacker.id] += gain
}

// decayCreatureThreatLocked decays and prunes a creature's threat table by one tick. Entries that
// fall to zero are removed so the table cannot grow unbounded across a long fight.
func (r *Runtime) decayCreatureThreatLocked(creature *entityState, dt float64) {
	if creature == nil || creature.threat == nil || len(creature.threat.Entries) == 0 || dt <= 0 {
		return
	}
	decay := r.creatureThreatProfile(creature).DecayPerSec * dt
	if decay <= 0 {
		return
	}
	for id, value := range creature.threat.Entries {
		if value-decay <= 0 {
			delete(creature.threat.Entries, id)
			if creature.threat.CurrentTarget == id {
				creature.threat.CurrentTarget = 0
			}
			continue
		}
		creature.threat.Entries[id] = value - decay
	}
}

// creatureThreatProfile resolves the threat tuning for a creature. Today only the wolf policy
// carries one; this is the seam where per-template threat profiles bind later.
func (r *Runtime) creatureThreatProfile(_ *entityState) ThreatRuntimeProfile {
	return r.contracts.WolfPolicy.Threat
}

// resolveCreatureTargetLocked picks which target a creature fights (Threat Slice 2). With a single
// candidate it returns that one unchanged (the single-player no-regression guarantee). With more
// than one it selects the highest-threat target, switching off the current target only when a
// challenger exceeds it by switch_threshold_ratio AND switch_cooldown_ms has elapsed (hysteresis,
// no per-tick flip-flop).
func (r *Runtime) resolveCreatureTargetLocked(creature *entityState, now time.Time) *entityState {
	if r == nil || creature == nil {
		return nil
	}
	candidates := make([]*entityState, 0, len(r.players))
	for _, p := range r.players {
		if p != nil && p.health > 0 {
			candidates = append(candidates, p)
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	if len(candidates) == 1 {
		return candidates[0]
	}

	table := creature.ensureThreatTable()
	profile := r.creatureThreatProfile(creature)

	var current, best *entityState
	bestThreat := -1.0
	for _, c := range candidates {
		if c.id == table.CurrentTarget {
			current = c
		}
		if th := table.Entries[c.id]; th > bestThreat {
			bestThreat, best = th, c
		}
	}
	if best == nil {
		best = candidates[0]
	}

	// Stick to the current target unless a challenger clearly out-threatens it and the cooldown
	// has elapsed.
	if current != nil && best.id != current.id {
		ratio := profile.SwitchThresholdRatio
		if ratio < 1 {
			ratio = 1
		}
		switchReady := now.Sub(table.LastSwitchAt) >= time.Duration(profile.SwitchCooldownMS)*time.Millisecond
		if !switchReady || bestThreat < table.Entries[current.id]*ratio {
			return current
		}
	}
	if best.id != table.CurrentTarget {
		table.CurrentTarget = best.id
		table.LastSwitchAt = now
	}
	return best
}
