package gameapi

// Character progression — XP on kill (Slice 2). See docs/roadmap/aaa-character-progression-roadmap.md.
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
