// Package clock declares the time port.
//
// Guide rule (P2/G4): tempo é uma dependency explícita. Domain e application
// nunca chamam time.Now; recebem um Clock para manter testes determinísticos.
package clock

import "time"

// Clock abstracts the passage of time.
type Clock interface {
	Now() time.Time
}
