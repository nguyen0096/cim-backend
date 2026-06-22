package repository

import (
	"context"

	"gorm.io/gorm"
)

// txContextKey is the unexported context key under which an in-flight *gorm.DB
// transaction is carried. Using a dedicated unexported type prevents collisions
// with any other context value in the process.
type txContextKey struct{}

// contextWithTx returns a child context that carries the given transaction
// handle. Repository methods resolve their *gorm.DB via dbFromContext (or
// BaseRepository.DB), so any method invoked with the returned context
// automatically runs inside this tx. Set by BaseRepository.WithinTx.
func contextWithTx(ctx context.Context, tx *gorm.DB) context.Context {
	return context.WithValue(ctx, txContextKey{}, tx)
}

// dbFromContext returns the transaction handle carried by ctx if one is present
// (i.e. the caller is inside a BaseRepository.WithinTx block), otherwise it falls
// back to the provided non-transactional handle. This lets a single repository
// method participate in a service-orchestrated transaction without changing its
// signature: when no tx is in context it behaves exactly as before.
func dbFromContext(ctx context.Context, fallback *gorm.DB) *gorm.DB {
	if tx, ok := ctx.Value(txContextKey{}).(*gorm.DB); ok && tx != nil {
		return tx
	}
	return fallback
}
