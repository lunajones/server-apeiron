package fsm

import "time"

type Transition struct {
	From   StateID
	To     StateID
	Reason string
	At     time.Time
	Forced bool
}

type TransitionRule func(Context) (StateID, string, bool)
