package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// tombstoneRetention is how long soft-deleted rows are kept so offline devices
// can still receive the deletion on their next delta sync. Devices offline
// longer than this may retain stale rows until a full re-sync; that trade-off
// keeps the tombstone tables (and the deleted_* arrays in /sync/batch) bounded.
const tombstoneRetention = 30 * 24 * time.Hour

// pruneEvery is the sweep interval.
const pruneEvery = 24 * time.Hour

// StartTombstonePruner runs a background sweep deleting expired tombstones and
// stale invitations until ctx is cancelled. One sweep runs at startup.
func StartTombstonePruner(ctx context.Context, pool *pgxpool.Pool) {
	go func() {
		pruneTombstones(ctx, pool)
		t := time.NewTicker(pruneEvery)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				pruneTombstones(ctx, pool)
			}
		}
	}()
}

func pruneTombstones(ctx context.Context, pool *pgxpool.Pool) {
	cutoff := time.Now().UTC().Add(-tombstoneRetention)
	stmts := []struct {
		name string
		sql  string
	}{
		// Deleting a tombstoned list cascades its list_items rows (FK ON DELETE CASCADE).
		{"lists", `DELETE FROM shopping_lists WHERE deleted_at IS NOT NULL AND deleted_at < $1`},
		{"list_items", `DELETE FROM list_items WHERE deleted_at IS NOT NULL AND deleted_at < $1`},
		// Consumed/expired invitations have no further use once old.
		{"invitations", `DELETE FROM list_invitations
			WHERE created_at < $1 AND (status <> 'pending' OR (expires_at IS NOT NULL AND expires_at < now()))`},
		// Old revocations: members learn about a revoke via delta well within the window.
		{"revoked_shares", `DELETE FROM list_shares WHERE revoked_at IS NOT NULL AND revoked_at < $1`},
	}
	for _, s := range stmts {
		tag, err := pool.Exec(ctx, s.sql, cutoff)
		if err != nil {
			slog.WarnContext(ctx, "prune.failed", slog.String("what", s.name), slog.Any("error", err))
			continue
		}
		if n := tag.RowsAffected(); n > 0 {
			slog.InfoContext(ctx, "prune.done", slog.String("what", s.name), slog.Int64("rows", n))
		}
	}
}
