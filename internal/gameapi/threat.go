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
