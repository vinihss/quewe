package tx

import "context"

// Transactor runs fn within a transactional boundary when supported by adapters.
// In tests, a no-op implementation can execute fn directly.
type Transactor interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}
