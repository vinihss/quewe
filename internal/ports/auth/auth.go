package auth

import "context"

// Principal identifies the authenticated caller for authorization and ownership.
// ADR-001 defines kind/scope/capability semantics; WP2.1 only needs owner isolation.
type Principal struct {
	ID   string
	Kind string
}

// Resolver authenticates a request context and returns the caller principal.
// WP2.2 uses it for HTTP inbound auth; implementations may be a dev-only
// config-gated stub or a real token verifier (ADR-001, WP2.3+).
type Resolver interface {
	Resolve(ctx context.Context) (Principal, error)
}
