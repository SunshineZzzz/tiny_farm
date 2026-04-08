package timeutil

import "time"

type Clock struct {
	last time.Time
}

func NewClock() *Clock {
	return &Clock{last: time.Now()}
}

func (c *Clock) Update() float64 {
	now := time.Now()
	delta := now.Sub(c.last).Seconds()
	c.last = now
	return delta
}
