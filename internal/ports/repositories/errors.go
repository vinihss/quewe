// Package repositories declares outbound persistence ports and their errors.
//
// Guide rule (P1/P6): erros de porta são neutros de tecnologia. O adapter
// traduz o erro do driver para estes sentinelas; o caso de uso nunca vê pgx/SQL.
package repositories

import "errors"

var (
	// ErrNotFound is returned when the requested record does not exist.
	ErrNotFound = errors.New("repositories: not found")
	// ErrConflict is returned when a uniqueness or version constraint fails.
	ErrConflict = errors.New("repositories: conflict")
)
