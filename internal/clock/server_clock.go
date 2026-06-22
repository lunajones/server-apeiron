package clock

import "time"

type ServerClock struct {
	start time.Time
}

func NewServerClock() ServerClock {
	return ServerClock{start: time.Now()}
}

func (c ServerClock) Now() time.Time {
	return time.Now()
}

func (c ServerClock) SinceStart() time.Duration {
	return time.Since(c.start)
}
