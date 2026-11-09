// Package id declares the identity generation port.
//
// Guide rule (P2/G5): geração de identidade é infraestrutura, não regra de
// negócio. Casos de uso dependem da porta, não de uuid/crypto diretamente.
package id

// Generator produces stable, unique identities.
type Generator interface {
	New() string
}
