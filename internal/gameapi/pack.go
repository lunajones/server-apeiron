package gameapi

import (
	"fmt"
	"sort"
)

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
