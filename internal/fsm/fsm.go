package fsm

type StateID string

type Context map[string]any

type Machine struct {
	state StateID
	rules []TransitionRule
}

func NewMachine(initial StateID) *Machine {
	return &Machine{state: initial}
}

func (m *Machine) State() StateID {
	if m == nil {
		return ""
	}
	return m.state
}

func (m *Machine) AddRule(rule TransitionRule) {
	if m == nil || rule == nil {
		return
	}
	m.rules = append(m.rules, rule)
}

func (m *Machine) Tick(ctx Context) (StateID, string, bool) {
	if m == nil {
		return "", "", false
	}
	for _, rule := range m.rules {
		next, reason, ok := rule(ctx)
		if !ok {
			continue
		}
		m.state = next
		return next, reason, true
	}
	return m.state, "", false
}
