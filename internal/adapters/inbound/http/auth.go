package http

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	"github.com/vinihss/quewe/services/quewe/internal/ports/auth"
)

type principalContextKey struct{}

// PrincipalFromContext returns the authenticated principal, if present.
func PrincipalFromContext(ctx context.Context) (auth.Principal, bool) {
	p, ok := ctx.Value(principalContextKey{}).(auth.Principal)
	return p, ok
}

func contextWithPrincipal(ctx context.Context, p auth.Principal) context.Context {
	return context.WithValue(ctx, principalContextKey{}, p)
}

type httpRequestContextKey struct{}

func contextWithRequest(ctx context.Context, r *http.Request) context.Context {
	return context.WithValue(ctx, httpRequestContextKey{}, r)
}

// DebugStubAuthResolver resolves a principal from X-Debug-Principal-Id.
// Config-gated, dev-only; wiring must only enable when config.AllowDebugAuthStub().
type DebugStubAuthResolver struct {
	header string
}

// NewDebugStubAuthResolver creates the dev-only stub resolver.
func NewDebugStubAuthResolver() DebugStubAuthResolver {
	return DebugStubAuthResolver{header: "X-Debug-Principal-Id"}
}

func (r DebugStubAuthResolver) Resolve(ctx context.Context) (auth.Principal, error) {
	req, _ := ctx.Value(httpRequestContextKey{}).(*http.Request)
	if req == nil {
		return auth.Principal{}, errAuthRequired
	}
	id := strings.TrimSpace(req.Header.Get(r.header))
	if id == "" {
		return auth.Principal{}, errAuthRequired
	}
	// Convert non-UUID debug IDs to deterministic UUIDs so they satisfy
	// the principals FK constraint in dev mode.
	uuid := toUUID(id)
	kind := "user"
	if strings.HasPrefix(id, "worker") {
		kind = "worker"
	}
	return auth.Principal{ID: uuid, Kind: kind}, nil
}

// toUUID converts an arbitrary string to a deterministic UUID v5 (SHA-1).
// If the input is already a valid UUID, it is returned as-is.
func toUUID(s string) string {
	// Check if already a valid UUID (8-4-4-4-12 hex format).
	if len(s) == 36 && s[8] == '-' && s[13] == '-' && s[18] == '-' && s[23] == '-' {
		return s
	}
	h := sha1.Sum([]byte(s))
	hexStr := hex.EncodeToString(h[:])
	// Format as UUID: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hexStr[0:8], hexStr[8:12], hexStr[12:16], hexStr[16:20], hexStr[20:32])
}

// FailClosedAuthResolver requires Authorization: Bearer but performs no token
// verification (WP2.2). It fails closed until a real resolver is configured.
type FailClosedAuthResolver struct{}

// NewFailClosedAuthResolver creates the safe default resolver.
func NewFailClosedAuthResolver() FailClosedAuthResolver { return FailClosedAuthResolver{} }

func (FailClosedAuthResolver) Resolve(ctx context.Context) (auth.Principal, error) {
	req, _ := ctx.Value(httpRequestContextKey{}).(*http.Request)
	if req == nil {
		return auth.Principal{}, errAuthRequired
	}
	h := strings.TrimSpace(req.Header.Get("Authorization"))
	if len(h) < 8 || !strings.EqualFold(h[:7], "bearer ") || strings.TrimSpace(h[7:]) == "" {
		return auth.Principal{}, errAuthRequired
	}
	// No real token verification in WP2.2 (ADR-001). Fail closed.
	return auth.Principal{}, errAuthNotConfigured
}
