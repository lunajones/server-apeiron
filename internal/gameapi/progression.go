package gameapi

import (
	"context"
	"time"

	dbv1 "db-apeiron/gen/apeiron/v1"
	gamev1 "server-apeiron/gen/apeiron/game/v1"

	"server-apeiron/internal/logging"
)

// progressionFlushInterval is how often dirty player progression is persisted to the data service.
// There is no disconnect hook, so a periodic flush (plus a final flush on shutdown) is the write path.
const progressionFlushInterval = 10 * time.Second

// Character leveling (Slice 4). v1 cap is 10; +3 attribute points per level. cumulativeCharacterXP[L]
// is the total experience required to reach level L (index = level), per the v1 curve in
// aaa-character-progression-roadmap.md §6 (wolf = 100 level XP → ~337 wolves to cap 10). Tunable data.
const (
	characterLevelCapV1     = 10
	attributePointsPerLevel = 3
)

var cumulativeCharacterXP = []int64{0, 0, 1200, 2800, 4900, 7600, 11000, 15200, 20300, 26400, 33700}

// Attribute -> derived combat stats (Slice 5). Additive over the base profile, never a rewrite.
// Strength scales physical damage, max health and physical resistance (the only one visible in v1,
// since all skills are physical); Dexterity (armor pen) and Intelligence (chem/bio) bind when those
// damage families exist. Attributes start at 1.0, so only points ABOVE the base scale anything.
// Constants are tunable starting values.
const (
	baseAttributeValue             = 1.0
	basePlayerMaxHealth            = 100.0
	strengthMaxHealthPerPoint      = 10.0 // +HP per Strength point above base
	strengthPhysicalDamagePerPoint = 0.05 // +5% physical damage per Strength point above base
	strengthPhysicalResistPerPoint = 2.0  // +physical resistance rating per Strength point above base
)

func strengthAboveBase(prog *playerProgression) float64 {
	if prog == nil || prog.strength <= baseAttributeValue {
		return 0
	}
	return prog.strength - baseAttributeValue
}

// attributePhysicalDamageMultiplier scales outgoing physical damage by Strength (1.0 at base).
func attributePhysicalDamageMultiplier(prog *playerProgression) float64 {
	return 1 + strengthAboveBase(prog)*strengthPhysicalDamagePerPoint
}

// attributePhysicalResistanceBonus is the additive physical resistance rating from Strength.
func attributePhysicalResistanceBonus(prog *playerProgression) float64 {
	return strengthAboveBase(prog) * strengthPhysicalResistPerPoint
}

// attributeMaxHealth is the player's max health: base + Strength bonus.
func attributeMaxHealth(prog *playerProgression) float64 {
	return basePlayerMaxHealth + strengthAboveBase(prog)*strengthMaxHealthPerPoint
}

// applyAttributeDerivedStatsLocked recomputes a player's derived stats (max health) from attributes.
// Called on attach (and after progression changes) so Strength visibly raises the health pool.
func (r *Runtime) applyAttributeDerivedStatsLocked(player *entityState) {
	if player == nil || player.entityType != "player" || player.progression == nil {
		return
	}
	newMax := attributeMaxHealth(player.progression)
	if newMax <= 0 {
		newMax = basePlayerMaxHealth
	}
	// Preserve the current health ratio so a max-health change scales fairly: a full-health player
	// (e.g. fresh on attach) ends full at the new max; a wounded one keeps the same percentage.
	ratio := 1.0
	if player.maxHealth > 0 && player.health > 0 {
		ratio = player.health / player.maxHealth
	}
	if ratio > 1 {
		ratio = 1
	}
	player.maxHealth = newMax
	player.health = ratio * newMax
}

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
					r.applyLevelProgressionLocked(player)
				}
			}
		}
	}
	r.despawnCreatureLocked(creature)
}

// applyLevelProgressionLocked raises the player's level for as long as their cumulative experience
// crosses the next threshold (up to the v1 cap), granting attribute points per level. Milestone
// passive picks (1 of 3 at lv 10/15/20/30) need authored data + a client choice and land in a later
// slice. Marks dirty so the gains persist.
func (r *Runtime) applyLevelProgressionLocked(player *entityState) {
	if player == nil || player.progression == nil {
		return
	}
	prog := player.progression
	for prog.level >= 1 && prog.level < characterLevelCapV1 {
		next := prog.level + 1
		if int(next) >= len(cumulativeCharacterXP) {
			break
		}
		if prog.experience < cumulativeCharacterXP[next] {
			break
		}
		prog.level = next
		prog.attributePoints += attributePointsPerLevel
		prog.dirty = true
	}
}

// playerProgressionSnapshot builds the HUD progression payload for a player entity (nil otherwise),
// so the client can show level, XP bar, attributes, points and coin (Slice 6).
func playerProgressionSnapshot(e *entityState) *gamev1.PlayerProgressionState {
	if e == nil || e.entityType != "player" || e.progression == nil {
		return nil
	}
	p := e.progression
	intoLevel, span := characterXPBar(p.level, p.experience)
	return &gamev1.PlayerProgressionState{
		Level:                  p.level,
		Experience:             p.experience,
		ExperienceIntoLevel:    intoLevel,
		ExperienceForNextLevel: span,
		AttributePoints:        p.attributePoints,
		Strength:               p.strength,
		Dexterity:              p.dexterity,
		Intelligence:           p.intelligence,
		Endurance:              p.endurance,
		Coin:                   p.coin,
		LevelCap:               characterLevelCapV1,
	}
}

// characterXPBar returns experience earned into the current level and that level's span (0 at the
// cap), for the HUD XP bar.
func characterXPBar(level int32, experience int64) (intoLevel int64, span int64) {
	if level < 1 {
		level = 1
	}
	if int(level)+1 >= len(cumulativeCharacterXP) { // at/above cap: no next level
		return 0, 0
	}
	base := cumulativeCharacterXP[level]
	next := cumulativeCharacterXP[level+1]
	intoLevel = experience - base
	if intoLevel < 0 {
		intoLevel = 0
	}
	return intoLevel, next - base
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
