package repository

import (
	"context"

	"gorm.io/gorm"
)

// txContextKey is the context key for an in-flight *gorm.DB transaction.
type txContextKey struct{}

// contextWithTx returns a child context carrying the transaction handle.
func contextWithTx(ctx context.Context, tx *gorm.DB) context.Context {
	return context.WithValue(ctx, txContextKey{}, tx)
}

// dbFromContext returns the transaction handle carried by ctx, or the fallback
// handle when none is present.
func dbFromContext(ctx context.Context, fallback *gorm.DB) *gorm.DB {
	if tx, ok := ctx.Value(txContextKey{}).(*gorm.DB); ok && tx != nil {
		return tx
	}
	return fallback
}

// runInTx runs fn in the caller's ambient transaction when present, otherwise
// opens its own one-shot transaction.
func runInTx(ctx context.Context, base *gorm.DB, fn func(tx *gorm.DB) error) error {
	if tx := dbFromContext(ctx, nil); tx != nil {
		return fn(tx.WithContext(ctx))
	}
	return base.WithContext(ctx).Transaction(fn)
}
