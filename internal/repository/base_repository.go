package repository

import (
	"context"

	"gorm.io/gorm"
)

// BaseRepository is the single shared root every concrete repository embeds. It
// owns the one *gorm.DB connection for the process and is the only place a
// repository reaches for a handle, so the connection is no longer scattered
// across repository dependency lists.
//
// It also resolves transaction coherence: the promoted db field is the base
// connection, while DB(ctx) returns the in-flight transaction when the caller is
// inside a WithinTx block (and the base connection otherwise). WithinTx is the
// "run these repository calls in one transaction" capability the service uses to
// make several repository method calls atomic without ever holding a *gorm.DB
// itself.
//
// Embedding: concrete repositories embed *baseRepository as an anonymous field,
// which promotes its `db` field so existing repository methods keep using `r.db`
// unchanged. New transaction-aware methods use r.DB(ctx) (or the equivalent
// dbFromContext) so they enlist in the caller's transaction when there is one.
//
//go:generate mockery --name=BaseRepository --structname=BaseRepository --output=../mocks/repositorymocks --outpkg=repositorymocks
type BaseRepository interface {
	// DB returns the transaction carried by ctx when the caller is inside a
	// WithinTx block, otherwise the base connection. Pass the result a
	// WithContext(ctx) before use, as the existing methods do.
	DB(ctx context.Context) *gorm.DB
	// Conn returns the base (non-transactional) connection. Used by methods that
	// open their own gorm transaction.
	Conn() *gorm.DB
	// WithinTx executes fn inside a single transaction. The context passed to fn
	// carries the transaction handle; every repository call made with that
	// context (via DB(ctx)/dbFromContext) enlists in the same transaction and is
	// committed or rolled back together. Nested calls reuse the in-flight tx.
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// baseRepository is the single concrete BaseRepository. Concrete repositories
// embed *baseRepository anonymously; its exported-via-promotion `db` field backs
// every existing `r.db` reference, so no existing repository method changes.
type baseRepository struct {
	db *gorm.DB
}

// NewBaseRepository constructs the one BaseRepository instance for the process.
// Pass the returned value to every repository constructor so they all share this
// single connection root.
func NewBaseRepository(db *gorm.DB) BaseRepository {
	return &baseRepository{db: db}
}

func (b *baseRepository) DB(ctx context.Context) *gorm.DB {
	return dbFromContext(ctx, b.db)
}

func (b *baseRepository) Conn() *gorm.DB {
	return b.db
}

func (b *baseRepository) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	// Already inside a transaction (nested call): reuse it so the work stays in
	// one atomic unit rather than opening a second, independent transaction.
	if existing := dbFromContext(ctx, nil); existing != nil {
		return fn(ctx)
	}
	return b.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(contextWithTx(ctx, tx))
	})
}

// asBase returns the concrete *baseRepository backing the given BaseRepository so
// it can be embedded as an anonymous field (which promotes the `db` field used by
// existing repository methods). All repositories share the one instance created
// by NewBaseRepository, so this assertion always holds.
func asBase(b BaseRepository) *baseRepository {
	if br, ok := b.(*baseRepository); ok {
		return br
	}
	// Defensive: wrap any alternative implementation around its connection so the
	// promoted db field and accessors stay consistent.
	return &baseRepository{db: b.Conn()}
}
