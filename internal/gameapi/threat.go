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

// accrueProximityThreatLocked adds slow threat for targets standing within proximity range, so a
// creature engages something in its face even if it never attacks (Threat Slice 3). No puller
// bonus here — that is reserved for the first attacker.
func (r *Runtime) accrueProximityThreatLocked(creature *entityState, dt float64) {
	if r == nil || creature == nil || creature.entityType != "creature" || dt <= 0 {
		return
	}
	profile := r.creatureThreatProfile(creature)
	if profile.ProximityThreatPerSec <= 0 || profile.ProximityRangeCM <= 0 {
		return
	}
	gain := profile.ProximityThreatPerSec * dt
	for _, p := range r.players {
		if p == nil || p.health <= 0 || p.id == creature.id {
			continue
		}
		if distance(creature.position, p.position) <= profile.ProximityRangeCM {
			creature.ensureThreatTable().Entries[p.id] += gain
		}
	}
}

// decayCreatureThreatLocked decays and prunes a creature's threat table by one tick (Threat
// Slices 1 + 3). A target still inside proximity range is "engaged" and does not decay (proximity
// keeps it relevant); a disengaged or gone target decays faster (out_of_range_decay_multiplier).
// Entries that reach zero are pruned so the table cannot grow unbounded.
func (r *Runtime) decayCreatureThreatLocked(creature *entityState, dt float64) {
	if creature == nil || creature.threat == nil || len(creature.threat.Entries) == 0 || dt <= 0 {
		return
	}
	profile := r.creatureThreatProfile(creature)
	if profile.DecayPerSec <= 0 {
		return
	}
	outMult := profile.OutOfRangeDecayMultiplier
	if outMult <= 0 {
		outMult = 1
	}
	for id, value := range creature.threat.Entries {
		target := r.playerByIDLocked(id)
		if target != nil && target.health > 0 && profile.ProximityRangeCM > 0 &&
			distance(creature.position, target.position) <= profile.ProximityRangeCM {
			continue // engaged: in proximity range, no decay
		}
		decay := profile.DecayPerSec * outMult * dt
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

func (r *Runtime) playerByIDLocked(id uint64) *entityState {
	for _, p := range r.players {
		if p != nil && p.id == id {
			return p
		}
	}
	return nil
}

const creatureLeashReturnRadiusCM = 120.0

// updateCreatureLeashLocked enforces the souls-like leash/reset (Threat Slice 4). The creature
// anchors at its spawn position; if pulled beyond leash_distance_cm it disengages (wipes threat),
// then walks home regenerating, and resumes normal behavior once home. Returns true while the
// creature is leashing so the caller skips combat for this tick. Leash overrides target selection
// on purpose — this is the new reset behavior, not the steady-state combat path.
func (r *Runtime) updateCreatureLeashLocked(creature *entityState, now time.Time) bool {
	if r == nil || creature == nil || creature.entityType != "creature" {
		return false
	}
	profile := r.creatureThreatProfile(creature)
	if profile.LeashDistanceCM <= 0 {
		return false
	}
	table := creature.ensureThreatTable()
	if !table.AnchorKnown {
		table.AnchorPos = creature.position
		table.AnchorKnown = true
	}
	distHome := distance(creature.position, table.AnchorPos)

	if !creature.creatureLeashed {
		if distHome <= profile.LeashDistanceCM {
			return false
		}
		// Breach: disengage and wipe threat.
		creature.creatureLeashed = true
		table.Entries = map[uint64]float64{}
		table.CurrentTarget = 0
	}

	if distHome <= creatureLeashReturnRadiusCM {
		creature.creatureLeashed = false
		if creature.maxHealth > 0 {
			creature.health = creature.maxHealth
		}
		if creature.maxStamina > 0 {
			creature.stamina = creature.maxStamina
		}
		creature.velocity = vector{}
		creature.movementState = "grounded"
		creature.skillState = "idle"
		creature.combatState = "ready"
		return false
	}

	// Walk home, regenerating on the way.
	dir := normalize(vector{x: table.AnchorPos.x - creature.position.x, y: table.AnchorPos.y - creature.position.y})
	speed := positiveOr(r.contracts.WolfPolicy.ChaseSpeedCMS, 300)
	step := speed / tickRate
	if step > distHome {
		step = distHome
	}
	creature.position = add(creature.position, scale(dir, step))
	creature.position.z = creature.position.z
	creature.velocity = scale(dir, speed)
	creature.velocity.z = 0
	creature.movementState = "returning_home"
	creature.skillState = "idle"
	creature.combatState = "ready"
	r.regenerateCreatureStaminaLocked(creature, r.contracts.WolfPolicy)
	return true
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
