package gameapi

import (
	"fmt"
	"sort"
	"time"

	creatureai "server-apeiron/internal/ai"
	"server-apeiron/internal/movement"
)

// packCommitMinDistanceCM is the root-displacement threshold above which a skill counts as a
// committed action for the pack commit budget (lunge/maul qualify; a bite does not).
const packCommitMinDistanceCM = 200.0

// packRuntime is the group-level coordinator that sits above the per-creature brains. It owns
// group intent (membership now; roles/slots/commit budget/focus in later slices) and never moves
// a creature directly. See docs/roadmap/aaa-pack-coordination-runtime-roadmap.md.
type packRuntime struct {
	PackID    string
	ProfileID string
	MemberIDs []uint64
}

func (r *Runtime) creaturePackProfile(_ *entityState) PackRuntimeProfile {
	return r.contracts.WolfPolicy.Pack
}

// formCreaturePacksLocked clusters nearby creatures of the same template into packs (Pack Slice 1).
// A creature joins an existing pack when within join_radius of a member (up to max_members), else
// it starts its own. A lone creature forms a pack of one whose membership is itself — behaviorally
// identical to today, since nothing reads pack membership yet. Pack id is the smallest member id
// for stability. Rebuilt each tick from current positions.
func (r *Runtime) formCreaturePacksLocked() {
	wolves := make([]*entityState, 0, len(r.entities))
	for _, c := range r.entities {
		if c != nil && c.entityType == "creature" && c.templateID == "steppe_wolf" && c.health > 0 {
			wolves = append(wolves, c)
		}
	}
	sort.Slice(wolves, func(i, j int) bool { return wolves[i].id < wolves[j].id })

	profile := r.creaturePackProfile(nil)
	joinRadius := profile.JoinRadiusCM
	maxMembers := int(profile.MaxMembers)

	var clusters [][]*entityState
	for _, w := range wolves {
		placed := false
		if joinRadius > 0 {
			for ci := range clusters {
				if maxMembers > 0 && len(clusters[ci]) >= maxMembers {
					continue
				}
				for _, m := range clusters[ci] {
					if distance(w.position, m.position) <= joinRadius {
						clusters[ci] = append(clusters[ci], w)
						placed = true
						break
					}
				}
				if placed {
					break
				}
			}
		}
		if !placed {
			clusters = append(clusters, []*entityState{w})
		}
	}

	r.packs = make(map[string]*packRuntime, len(clusters))
	for _, cluster := range clusters {
		minID := cluster[0].id
		for _, m := range cluster {
			if m.id < minID {
				minID = m.id
			}
		}
		packID := fmt.Sprintf("pack:%d", minID)
		members := make([]uint64, 0, len(cluster))
		for _, m := range cluster {
			m.packID = packID
			members = append(members, m.id)
		}
		r.packs[packID] = &packRuntime{PackID: packID, ProfileID: r.contracts.WolfPolicy.ContractID, MemberIDs: members}
	}
}

func (r *Runtime) packMembersLocked(pack *packRuntime) []*entityState {
	members := make([]*entityState, 0, len(pack.MemberIDs))
	for _, id := range pack.MemberIDs {
		if e := r.entities[id]; e != nil && e.health > 0 {
			members = append(members, e)
		}
	}
	sort.Slice(members, func(i, j int) bool { return members[i].id < members[j].id })
	return members
}

// assignPackRingSlotsLocked distributes pack members around their target's ring at distinct
// bearings so they surround instead of clump (Pack Slice 2). Slots fan out, centered on the
// bearing from the target to the pack's current centroid, spaced by surround_spacing_deg (capped
// to 360/n so they never overlap). A pack of one is NOT slotted (packSlotKnown stays false), which
// preserves the single-creature identity guarantee. The assigned bearing is later used to steer
// tactical orbit movement; it never moves the creature directly.
func (r *Runtime) assignPackRingSlotsLocked(now time.Time) {
	profile := r.creaturePackProfile(nil)
	for _, pack := range r.packs {
		members := r.packMembersLocked(pack)
		n := len(members)
		if n <= 1 {
			for _, m := range members {
				m.packSlotKnown = false
			}
			continue
		}
		target := r.resolveCreatureTargetLocked(members[0], now)
		if target == nil {
			for _, m := range members {
				m.packSlotKnown = false
			}
			continue
		}
		var cx, cy float64
		for _, m := range members {
			cx += m.position.x
			cy += m.position.y
		}
		cx /= float64(n)
		cy /= float64(n)
		base := vectorYaw(vector{x: cx - target.position.x, y: cy - target.position.y})
		step := profile.SurroundSpacingDeg
		if step <= 0 || float64(n)*step > 360 {
			step = 360.0 / float64(n)
		}
		for i, m := range members {
			offset := (float64(i) - float64(n-1)/2.0) * step
			m.packRingSlotDeg = normalizeYaw(base + offset)
			m.packSlotKnown = true
		}
	}
}

// packSlotSteerDirection returns a unit direction that moves a slotted member toward its assigned
// bearing on the target's ring, keeping its current radius (it slides around the ring to its slot).
func packSlotSteerDirection(creature *entityState, target *entityState) vector {
	if creature == nil || target == nil {
		return vector{}
	}
	radius := distance(vector{x: creature.position.x, y: creature.position.y}, vector{x: target.position.x, y: target.position.y})
	if radius <= 0 {
		return vector{}
	}
	slotPoint := add(target.position, scale(yawVector(creature.packRingSlotDeg), radius))
	return normalize(vector{x: slotPoint.x - creature.position.x, y: slotPoint.y - creature.position.y})
}

// applyPackSlotSteeringLocked biases a member's tactical orbit movement toward its pack ring slot,
// so the pack surrounds the target. It only touches circling/orbit tactics (where slotting matters)
// and leaves approach/retreat/commit to the brain. Returns the decision unchanged when the member
// is not slotted (incl. packs of one) or is not orbiting.
func (r *Runtime) applyPackSlotSteeringLocked(creature *entityState, target *entityState, decision creatureai.Decision) creatureai.Decision {
	if creature == nil || target == nil || !creature.packSlotKnown {
		return decision
	}
	if !creatureDecisionUsesSideOnBody(decision) {
		return decision
	}
	steer := packSlotSteerDirection(creature, target)
	if steer == (vector{}) {
		return decision
	}
	brainDir := fromDomainVector(flattenDomainDirection(decision.Direction))
	const slotWeight = 0.6
	blended := normalize(add(scale(brainDir, 1-slotWeight), scale(steer, slotWeight)))
	if blended == (vector{}) {
		return decision
	}
	decision.Direction = toDomainVector(blended)
	return decision
}

// isCommittingSkill reports whether a skill is a committed action the pack must budget — one that
// carries a phase envelope or moves the creature root a meaningful distance (lunge, maul), as
// opposed to a quick in-place attack (bite). Data-driven, not a wolf-count branch.
func (r *Runtime) isCommittingSkill(skillID string) bool {
	if skillID == "" {
		return false
	}
	contract := r.contracts.skillContract(skillID)
	if contract.Envelope != nil {
		return true
	}
	return movement.ActionDistance(contract.MovementAction, 0) >= packCommitMinDistanceCM
}

// creatureIsCommittingLocked reports whether a creature is currently in (or exiting) a committed
// action, so it is holding a commit token.
func (r *Runtime) creatureIsCommittingLocked(creature *entityState) bool {
	if creature == nil {
		return false
	}
	if creature.actionInstance != nil && r.isCommittingSkill(creature.actionInstance.SkillID.String()) {
		return true
	}
	if t := creature.creatureActionTransition; t != nil && r.isCommittingSkill(t.SkillID) {
		return true
	}
	return false
}

// packMayCommitLocked enforces the commit budget (Pack Slice 3): at most max_committed_members of
// a pack may be in a committed action at once. A member already committing keeps its token; a new
// committer is allowed only while the pack is under budget. A solo creature (pack of one) is always
// allowed — identity preserved.
func (r *Runtime) packMayCommitLocked(creature *entityState) bool {
	if creature == nil {
		return false
	}
	if r.creatureIsCommittingLocked(creature) {
		return true
	}
	if creature.packID == "" {
		return true
	}
	pack := r.packs[creature.packID]
	if pack == nil || len(pack.MemberIDs) <= 1 {
		return true
	}
	maxCommit := int(r.creaturePackProfile(nil).MaxCommittedMembers)
	if maxCommit <= 0 {
		maxCommit = 1
	}
	committing := 0
	for _, id := range pack.MemberIDs {
		if id == creature.id {
			continue
		}
		if m := r.entities[id]; m != nil && r.creatureIsCommittingLocked(m) {
			committing++
		}
	}
	if committing >= maxCommit {
		return false
	}
	// Rotation (Slice 4): a member that just committed yields its next turn while another member
	// is free to take it, so commits rotate around the pack instead of one wolf hogging the token.
	cooldown := time.Duration(r.creaturePackProfile(nil).CommitTokenCooldownMS) * time.Millisecond
	if cooldown > 0 && !creature.lastCommitAt.IsZero() && time.Since(creature.lastCommitAt) < cooldown {
		if r.packHasOtherAvailableCommitterLocked(creature, pack, cooldown) {
			return false
		}
	}
	return true
}

// packHasOtherAvailableCommitterLocked reports whether another pack member is free to take the
// commit turn (not committing and not itself on its post-commit cooldown).
func (r *Runtime) packHasOtherAvailableCommitterLocked(creature *entityState, pack *packRuntime, cooldown time.Duration) bool {
	for _, id := range pack.MemberIDs {
		if id == creature.id {
			continue
		}
		m := r.entities[id]
		if m == nil || m.health <= 0 || r.creatureIsCommittingLocked(m) {
			continue
		}
		if cooldown > 0 && !m.lastCommitAt.IsZero() && time.Since(m.lastCommitAt) < cooldown {
			continue
		}
		return true
	}
	return false
}

// assignPackRolesLocked labels each member by its current commit state (Slice 4): the committing
// member is the engager, a member inside its post-commit cooldown is recovering, everyone else
// harasses. Roles are presentational (debug/snapshot) plus the rotation cooldown that drives them.
func (r *Runtime) assignPackRolesLocked() {
	cooldown := time.Duration(r.creaturePackProfile(nil).CommitTokenCooldownMS) * time.Millisecond
	for _, pack := range r.packs {
		for _, id := range pack.MemberIDs {
			m := r.entities[id]
			if m == nil {
				continue
			}
			switch {
			case r.creatureIsCommittingLocked(m):
				m.packRole = "engager"
			case cooldown > 0 && !m.lastCommitAt.IsZero() && time.Since(m.lastCommitAt) < cooldown:
				m.packRole = "recoverer"
			default:
				m.packRole = "harasser"
			}
		}
	}
}

// suppressCommitDecision converts a committing decision into the member's tactical movement, so a
// member without a commit token harasses/repositions instead of attacking this turn.
func suppressCommitDecision(decision creatureai.Decision) creatureai.Decision {
	tactic := decision.MovementTactic
	if tactic == "" {
		tactic = "circle"
	}
	decision.Action = tactic
	decision.SelectedSkill = ""
	return decision
}
