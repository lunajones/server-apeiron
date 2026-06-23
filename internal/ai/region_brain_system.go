package ai

type RegionBrainSystem struct {
	memories map[string]Memory
}

func NewRegionBrainSystem() *RegionBrainSystem {
	return &RegionBrainSystem{memories: map[string]Memory{}}
}

func (s *RegionBrainSystem) Decide(agentID string, policy Policy, input Input) Decision {
	if s == nil {
		brain := NewBrain(policy)
		return brain.Decide(input)
	}
	if s.memories == nil {
		s.memories = map[string]Memory{}
	}
	brain := NewBrain(policy)
	brain.Memory = s.memories[agentID]
	decision := brain.Decide(input)
	s.memories[agentID] = brain.Memory
	return decision
}

func (s *RegionBrainSystem) Memory(agentID string) Memory {
	if s == nil || s.memories == nil {
		return Memory{}
	}
	return s.memories[agentID]
}

func (s *RegionBrainSystem) Forget(agentID string) {
	if s == nil || s.memories == nil {
		return
	}
	delete(s.memories, agentID)
}
