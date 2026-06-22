package gamefsm

import (
	"fmt"

	"server-apeiron/internal/fsm"
)

type Machine struct {
	name    string
	state   fsm.StateID
	allowed map[fsm.StateID]struct{}
	rules   []transitionRule
}

type transitionRule struct {
	from   fsm.StateID
	key    string
	to     fsm.StateID
	reason string
}

func NewMachine(name string, initial fsm.StateID, allowed ...fsm.StateID) (*Machine, error) {
	if initial == "" {
		return nil, fmt.Errorf("fsm %s initial state is empty", name)
	}
	m := &Machine{name: name, state: initial, allowed: make(map[fsm.StateID]struct{}, len(allowed)+1)}
	m.allowed[initial] = struct{}{}
	for _, state := range allowed {
		m.allowed[state] = struct{}{}
	}
	return m, nil
}

func Must(machine *Machine, err error) *Machine {
	if err != nil {
		panic(err)
	}
	return machine
}

func (m *Machine) State() fsm.StateID {
	if m == nil {
		return ""
	}
	return m.state
}

func (m *Machine) Apply(ctx fsm.Context) (fsm.StateID, string, bool) {
	if m == nil {
		return "", "", false
	}
	for _, rule := range m.rules {
		if rule.from != "" && rule.from != m.state {
			continue
		}
		value, ok := ctx[rule.key].(bool)
		if !ok || !value {
			continue
		}
		m.state = rule.to
		return rule.to, rule.reason, true
	}
	return m.state, "", false
}

func addBoolTransition(m *Machine, from fsm.StateID, key string, to fsm.StateID, reason string) {
	if m == nil {
		return
	}
	m.rules = append(m.rules, transitionRule{from: from, key: key, to: to, reason: reason})
}
