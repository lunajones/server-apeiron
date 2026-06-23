package ai

type Memory struct {
	OrbitSide              string
	OrbitSideChangedAtTick uint64
	LastSelectedSkill      string
	LastSelectedSkillTick  uint64
	LastSetupPolicyID      string
	LastSetupStartedTick   uint64
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
	if decision.SelectedSkill != "" && decision.SelectedSkill != m.LastSelectedSkill {
		m.LastSelectedSkillTick = tick
	}
	if decision.SetupPolicyID != "" && decision.SetupPolicyID != m.LastSetupPolicyID {
		m.LastSetupPolicyID = decision.SetupPolicyID
		m.LastSetupStartedTick = tick
	}
	if decision.SelectedSkill != "" {
		m.LastSelectedSkill = decision.SelectedSkill
	}
	m.LastDecisionReason = decision.Reason
	m.LastTacticalState = decision.TacticalState
	m.LastDecisionPhase = decision.DecisionPhase
}
