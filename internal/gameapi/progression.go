package gameapi

import (
	"context"
	"time"

	dbv1 "db-apeiron/gen/apeiron/v1"

	"server-apeiron/internal/logging"
)

// progressionFlushInterval is how often dirty player progression is persisted to the data service.
// There is no disconnect hook, so a periodic flush (plus a final flush on shutdown) is the write path.
const progressionFlushInterval = 10 * time.Second

// Character progression — XP on kill (Slice 2) + persistence (Slice 1b).
// See docs/roadmap/aaa-character-progression-roadmap.md.
// Level XP is credited only on a creature's death, split across damage contributors by damage share.
// Weapon XP (per-mode, capped) lands with the mode-progress plumbing in a later slice.

// creditDamageLocked accumulates the raw damage an attacker has dealt to a creature, building the
// contribution ledger used to split kill XP. Creatures only; self-damage and zero are ignored.
func (r *Runtime) creditDamageLocked(creature, attacker *entityState, damage float64) {
	if creature == nil || attacker == nil || damage <= 0 {
		return
	}
	if creature.entityType != "creature" || creature.id == attacker.id {
		return
	}
	if creature.damageCredits == nil {
		creature.damageCredits = map[uint64]float64{}
	}
	creature.damageCredits[attacker.id] += damage
}

// creatureProgressionProfile resolves the XP payout for a creature. Today only the wolf policy carries
// one; this is the seam where per-template progression profiles bind later (mirrors threat/pack).
func (r *Runtime) creatureProgressionProfile(_ *entityState) CreatureProgressionProfile {
	return r.contracts.WolfPolicy.Progression
}

// awardKillXPLocked credits level XP to the player contributors when a creature dies, split by their
// damage share (a group never multiplies the pool), then despawns the creature. Healing/buff grant no
// level XP, so the damage ledger is the whole story for this pool.
func (r *Runtime) awardKillXPLocked(creature *entityState) {
	if creature == nil || creature.entityType != "creature" {
		return
	}
	xpValue := r.creatureProgressionProfile(creature).ExperienceValue
	if xpValue > 0 && len(creature.damageCredits) > 0 {
		var total float64
		for _, dmg := range creature.damageCredits {
			total += dmg
		}
		if total > 0 {
			for attackerID, dmg := range creature.damageCredits {
				player := r.playerByIDLocked(attackerID)
				if player == nil || player.progression == nil {
					continue
				}
				if award := int64(xpValue * (dmg / total)); award > 0 {
					player.progression.experience += award
					player.progression.dirty = true
				}
			}
		}
	}
	r.despawnCreatureLocked(creature)
}

// despawnCreatureLocked removes a creature from the world after death. Respawn cadence is a later
// concern; for now a killed creature is gone until the pack is re-ensured on the next attach.
func (r *Runtime) despawnCreatureLocked(creature *entityState) {
	if creature == nil {
		return
	}
	delete(r.entities, creature.id)
}

// playerProgressionToProto builds the data-service payload for persisting a player's progression.
func playerProgressionToProto(playerID string, p *playerProgression) *dbv1.Player {
	return &dbv1.Player{
		Id:              playerID,
		Level:           p.level,
		Experience:      p.experience,
		AttributePoints: p.attributePoints,
		Strength:        p.strength,
		Dexterity:       p.dexterity,
		Intelligence:    p.intelligence,
		Endurance:       p.endurance,
		Coin:            p.coin,
	}
}

// collectDirtyProgressionLocked snapshots players whose progression changed and clears their dirty
// flag, returning the payloads to write. Must be called under r.mu.
func (r *Runtime) collectDirtyProgressionLocked() []*dbv1.Player {
	var out []*dbv1.Player
	for playerID, entity := range r.players {
		if entity == nil || entity.progression == nil || !entity.progression.dirty {
			continue
		}
		out = append(out, playerProgressionToProto(playerID, entity.progression))
		entity.progression.dirty = false
	}
	return out
}

// flushDirtyProgression persists changed player progression. It locks only to collect the payloads,
// then writes OUTSIDE the lock so a slow data service never stalls the runtime. A failed write
// re-marks the player dirty so the next flush retries.
func (r *Runtime) flushDirtyProgression(ctx context.Context) {
	r.mu.Lock()
	source := r.playerSource
	payloads := r.collectDirtyProgressionLocked()
	r.mu.Unlock()
	if source == nil || len(payloads) == 0 {
		return
	}
	for _, payload := range payloads {
		if _, err := source.UpdatePlayer(ctx, payload); err != nil {
			logging.WithComponent("gameapi").Warn().Err(err).Str("player_id", payload.GetId()).
				Msg("player progression persist failed; will retry")
			r.mu.Lock()
			if entity := r.players[payload.GetId()]; entity != nil && entity.progression != nil {
				entity.progression.dirty = true
			}
			r.mu.Unlock()
		}
	}
}

// runProgressionFlushLoop persists dirty player progression on a fixed interval, with a final flush
// on shutdown. Started only when a player source is wired (prod), never in tests.
func (r *Runtime) runProgressionFlushLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			r.flushDirtyProgression(context.Background())
			return
		case <-ticker.C:
			r.flushDirtyProgression(ctx)
		}
	}
}
