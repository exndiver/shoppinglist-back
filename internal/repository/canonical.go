package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ResolveGoodCanonical walks the merge chain in a single recursive query.
// The depth cap (c.depth < 64) terminates the recursion even if the data is
// accidentally cyclic; each step stays within the owner and excludes soft-deleted rows.
func ResolveGoodCanonical(ctx context.Context, db DBTX, ownerID, goodID uuid.UUID) (uuid.UUID, error) {
	const q = `
		WITH RECURSIVE chain AS (
			SELECT id, merged_into, 0 AS depth
			FROM goods
			WHERE owner_id = $1 AND id = $2 AND ` + sqlActive + `
			UNION ALL
			SELECT g.id, g.merged_into, c.depth + 1
			FROM goods g
			JOIN chain c ON g.id = c.merged_into
			WHERE g.owner_id = $1 AND g.` + sqlActiveFrag + ` AND c.depth < 64
		)
		SELECT id FROM chain WHERE merged_into IS NULL LIMIT 1
	`
	var id uuid.UUID
	err := db.QueryRow(ctx, q, ownerID, goodID).Scan(&id)
	if err == pgx.ErrNoRows {
		return uuid.Nil, ErrNotFound
	}
	return id, err
}

// ResolveGoodCanonicalAny walks the merge chain by id, regardless of owner.
// Used on shared lists, where a collaborator's item references a good owned by
// a different participant; merge chains stay within a single owner, so walking
// by id alone is safe.
func ResolveGoodCanonicalAny(ctx context.Context, db DBTX, goodID uuid.UUID) (uuid.UUID, error) {
	const q = `
		WITH RECURSIVE chain AS (
			SELECT id, merged_into, 0 AS depth
			FROM goods
			WHERE id = $1 AND ` + sqlActive + `
			UNION ALL
			SELECT g.id, g.merged_into, c.depth + 1
			FROM goods g
			JOIN chain c ON g.id = c.merged_into
			WHERE g.` + sqlActiveFrag + ` AND c.depth < 64
		)
		SELECT id FROM chain WHERE merged_into IS NULL LIMIT 1
	`
	var id uuid.UUID
	err := db.QueryRow(ctx, q, goodID).Scan(&id)
	if err == pgx.ErrNoRows {
		return uuid.Nil, ErrNotFound
	}
	return id, err
}

// ResolveStoreCanonicalAny walks the store merge chain by id, regardless of
// owner. Shared-list participants are equals: a collaborator may price a good
// at a store another participant created, so the store must resolve without
// being scoped to the caller. Merge chains stay within a single owner, so
// walking by id alone is safe.
func ResolveStoreCanonicalAny(ctx context.Context, db DBTX, storeID uuid.UUID) (uuid.UUID, error) {
	const q = `
		WITH RECURSIVE chain AS (
			SELECT id, merged_into, 0 AS depth
			FROM stores
			WHERE id = $1 AND ` + sqlActive + `
			UNION ALL
			SELECT s.id, s.merged_into, c.depth + 1
			FROM stores s
			JOIN chain c ON s.id = c.merged_into
			WHERE s.` + sqlActiveFrag + ` AND c.depth < 64
		)
		SELECT id FROM chain WHERE merged_into IS NULL LIMIT 1
	`
	var id uuid.UUID
	err := db.QueryRow(ctx, q, storeID).Scan(&id)
	if err == pgx.ErrNoRows {
		return uuid.Nil, ErrNotFound
	}
	return id, err
}

// ResolveStoreCanonical walks the merge chain in a single recursive query.
func ResolveStoreCanonical(ctx context.Context, db DBTX, ownerID, storeID uuid.UUID) (uuid.UUID, error) {
	const q = `
		WITH RECURSIVE chain AS (
			SELECT id, merged_into, 0 AS depth
			FROM stores
			WHERE owner_id = $1 AND id = $2 AND ` + sqlActive + `
			UNION ALL
			SELECT s.id, s.merged_into, c.depth + 1
			FROM stores s
			JOIN chain c ON s.id = c.merged_into
			WHERE s.owner_id = $1 AND s.` + sqlActiveFrag + ` AND c.depth < 64
		)
		SELECT id FROM chain WHERE merged_into IS NULL LIMIT 1
	`
	var id uuid.UUID
	err := db.QueryRow(ctx, q, ownerID, storeID).Scan(&id)
	if err == pgx.ErrNoRows {
		return uuid.Nil, ErrNotFound
	}
	return id, err
}
