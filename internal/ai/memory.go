package ai

type Memory struct {
	OrbitSide              string
	OrbitSideChangedAtTick uint64
	LastSelectedSkill      string
	LastDecisionReason     string
	LastTacticalState      string
	LastDecisionPhase      string
}

func (m *Memory) ensure() {
	if m.OrbitSide == "" {
		m.OrbitSide = "left"
	}
}

func (m *Memory) remember(decision Decision, tick uint64) {
	if m == nil {
		return
	}
	if decision.OrbitSide != "" && decision.OrbitSide != m.OrbitSide {
		m.OrbitSide = decision.OrbitSide
		m.OrbitSideChangedAtTick = tick
	}
	m.LastSelectedSkill = decision.SelectedSkill
	m.LastDecisionReason = decision.Reason
	m.LastTacticalState = decision.TacticalState
	m.LastDecisionPhase = decision.DecisionPhase
}
