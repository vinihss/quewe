// Package clock implements the clock port with the system clock.
package clock

import "time"

// System is the production Clock.
type System struct{}

// Now returns the current instant in UTC.
func (System) Now() time.Time { return time.Now().UTC() }
