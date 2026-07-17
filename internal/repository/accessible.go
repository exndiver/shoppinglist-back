package repository

import (
	"context"
	"time"

	"github.com/exndiver/shopping-backend/internal/models"
	"github.com/google/uuid"
)

// List-scoped sync: a single source of truth for "content of every list the
// caller can access" — lists they OWN plus lists shared TO them via an active
// list_shares row. Every query here decides visibility by LIST access, never by
// the owner_id of an individual row, so a collaborator's item/good/offer is
// delivered to everyone on the list regardless of which owner stamped the row.
//
// The shared predicate, repeated below, is:
//
//	JOIN shopping_lists sl ON sl.id = <row>.list_id AND sl.deleted_at IS NULL
//	LEFT JOIN list_shares sh
//	    ON sh.list_id = sl.id AND sh.member_owner_id = $1 AND sh.revoked_at IS NULL
//	WHERE (sl.owner_id = $1 OR sh.id IS NOT NULL)        -- caller can access the list
//	  AND (<row>.updated_at > $2 OR sh.updated_at > $2)  -- changed, or membership is fresh
//
// The `sh.updated_at > $2` term makes a freshly-accepted membership pull a full
// snapshot of the list even when the underlying rows are old.

// AccessibleList is a list the caller can see, with the caller's effective
// access ("owner" | "edit" | "view").
type AccessibleList struct {
	models.ShoppingList
	Access string
}

// ListAccessibleListsSince returns lists the caller owns or is an active member
// of, changed since `since` (or whose membership changed since).
func ListAccessibleListsSince(ctx context.Context, db DBTX, callerID uuid.UUID, since time.Time) ([]AccessibleList, error) {
	rows, err := db.Query(ctx, `
		SELECT sl.id, sl.owner_id, sl.name, sl.created_by, sl.created_at, sl.updated_at,
		       CASE WHEN sl.owner_id = $1 THEN 'owner' ELSE COALESCE(sh.access, 'view') END AS access
		FROM shopping_lists sl
		LEFT JOIN list_shares sh
		    ON sh.list_id = sl.id AND sh.member_owner_id = $1 AND sh.revoked_at IS NULL
		WHERE sl.deleted_at IS NULL
		  AND (sl.owner_id = $1 OR sh.id IS NOT NULL)
		  AND (sl.updated_at > $2 OR sh.updated_at > $2)
		ORDER BY sl.updated_at ASC, sl.id ASC
	`, callerID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AccessibleList
	for rows.Next() {
		var l models.ShoppingList
		var access string
		if err := rows.Scan(&l.ID, &l.OwnerID, &l.Name, &l.CreatedBy, &l.CreatedAt, &l.UpdatedAt, &access); err != nil {
			return nil, err
		}
		out = append(out, AccessibleList{ShoppingList: l, Access: access})
	}
	return out, rows.Err()
}

// ListAccessibleListItemsSince returns items on every list the caller can access.
func ListAccessibleListItemsSince(ctx context.Context, db DBTX, callerID uuid.UUID, since time.Time) ([]models.ListItem, error) {
	rows, err := db.Query(ctx, `
		SELECT li.id, li.owner_id, li.list_id, li.good_id, li.offer_id, li.quantity,
		       li.price_snapshot, li.is_purchased, li.created_by, li.created_at, li.updated_at
		FROM list_items li
		JOIN shopping_lists sl ON sl.id = li.list_id AND sl.deleted_at IS NULL
		LEFT JOIN list_shares sh
		    ON sh.list_id = sl.id AND sh.member_owner_id = $1 AND sh.revoked_at IS NULL
		WHERE li.deleted_at IS NULL
		  AND (sl.owner_id = $1 OR sh.id IS NOT NULL)
		  AND (li.updated_at > $2 OR sh.updated_at > $2)
		ORDER BY li.updated_at ASC, li.id ASC
	`, callerID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.ListItem
	for rows.Next() {
		it, err := scanListItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *it)
	}
	return out, rows.Err()
}

// ListDeletedListIDsForCallerSince returns ids of tombstoned lists the caller
// owned or was a member of, so every participant's device drops the list (and
// its items) locally. Membership rows are intentionally matched regardless of
// revocation: the deletion must still reach a member whose share row was
// revoked at the same time.
func ListDeletedListIDsForCallerSince(ctx context.Context, db DBTX, callerID uuid.UUID, since time.Time) ([]uuid.UUID, error) {
	rows, err := db.Query(ctx, `
		SELECT DISTINCT sl.id
		FROM shopping_lists sl
		LEFT JOIN list_shares sh
		    ON sh.list_id = sl.id AND sh.member_owner_id = $1
		WHERE sl.deleted_at IS NOT NULL
		  AND (sl.owner_id = $1 OR sh.id IS NOT NULL)
		  AND (sl.updated_at > $2 OR sh.updated_at > $2)
		ORDER BY sl.id ASC
	`, callerID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanUUIDs(rows)
}

// ListDeletedItemIDsForAccessibleListsSince returns ids of items tombstoned on
// any list the caller can access, so every participant drops them locally.
func ListDeletedItemIDsForAccessibleListsSince(ctx context.Context, db DBTX, callerID uuid.UUID, since time.Time) ([]uuid.UUID, error) {
	rows, err := db.Query(ctx, `
		SELECT li.id
		FROM list_items li
		JOIN shopping_lists sl ON sl.id = li.list_id
		LEFT JOIN list_shares sh
		    ON sh.list_id = sl.id AND sh.member_owner_id = $1 AND sh.revoked_at IS NULL
		WHERE li.deleted_at IS NOT NULL
		  AND (sl.owner_id = $1 OR sh.id IS NOT NULL)
		  AND (li.updated_at > $2 OR sh.updated_at > $2)
		ORDER BY li.updated_at ASC, li.id ASC
	`, callerID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanUUIDs(rows)
}

// ── Foreign catalog referenced by accessible-list content ──────────────────
//
// The caller's own catalog (goods/stores/offers/prices) arrives via the normal
// owner-scoped queries. These deliver, in addition, the catalog rows owned by
// OTHER participants that the caller's accessible-list items reference, so those
// items render. Gated on the row, its linking item, or the membership changing.

func ListForeignGoodsForAccessibleListsSince(ctx context.Context, db DBTX, callerID uuid.UUID, since time.Time) ([]models.Good, error) {
	rows, err := db.Query(ctx, `
		SELECT DISTINCT g.id, g.owner_id, g.category_id, g.name, g.normalized_name,
		       g.merged_into, g.created_by, g.created_at, g.updated_at
		FROM goods g
		JOIN list_items li ON li.good_id = g.id AND li.deleted_at IS NULL
		JOIN shopping_lists sl ON sl.id = li.list_id AND sl.deleted_at IS NULL
		LEFT JOIN list_shares sh
		    ON sh.list_id = sl.id AND sh.member_owner_id = $1 AND sh.revoked_at IS NULL
		WHERE g.owner_id <> $1 AND g.deleted_at IS NULL
		  AND (sl.owner_id = $1 OR sh.id IS NOT NULL)
		  AND (g.updated_at > $2 OR li.updated_at > $2 OR sh.updated_at > $2)
		ORDER BY g.updated_at ASC, g.id ASC
	`, callerID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Good
	for rows.Next() {
		g, err := scanGood(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *g)
	}
	return out, rows.Err()
}

func ListForeignOffersForAccessibleListsSince(ctx context.Context, db DBTX, callerID uuid.UUID, since time.Time) ([]models.Offer, error) {
	rows, err := db.Query(ctx, `
		SELECT DISTINCT o.id, o.owner_id, o.good_id, o.store_id, o.created_by, o.created_at, o.updated_at, o.deleted_at
		FROM offers o
		JOIN list_items li ON li.good_id = o.good_id AND li.deleted_at IS NULL
		JOIN shopping_lists sl ON sl.id = li.list_id AND sl.deleted_at IS NULL
		LEFT JOIN list_shares sh
		    ON sh.list_id = sl.id AND sh.member_owner_id = $1 AND sh.revoked_at IS NULL
		WHERE o.owner_id <> $1 AND o.deleted_at IS NULL
		  AND (sl.owner_id = $1 OR sh.id IS NOT NULL)
		  AND (o.updated_at > $2 OR li.updated_at > $2 OR sh.updated_at > $2)
		ORDER BY o.updated_at ASC, o.id ASC
	`, callerID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Offer
	for rows.Next() {
		o, err := scanOffer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *o)
	}
	return out, rows.Err()
}

func ListForeignStoresForAccessibleListsSince(ctx context.Context, db DBTX, callerID uuid.UUID, since time.Time) ([]models.Store, error) {
	rows, err := db.Query(ctx, `
		SELECT DISTINCT s.id, s.owner_id, s.name, s.normalized_name, s.merged_into, s.created_by, s.created_at, s.updated_at
		FROM stores s
		JOIN offers o ON o.store_id = s.id AND o.deleted_at IS NULL
		JOIN list_items li ON li.good_id = o.good_id AND li.deleted_at IS NULL
		JOIN shopping_lists sl ON sl.id = li.list_id AND sl.deleted_at IS NULL
		LEFT JOIN list_shares sh
		    ON sh.list_id = sl.id AND sh.member_owner_id = $1 AND sh.revoked_at IS NULL
		WHERE s.owner_id <> $1 AND s.deleted_at IS NULL
		  AND (sl.owner_id = $1 OR sh.id IS NOT NULL)
		  AND (s.updated_at > $2 OR li.updated_at > $2 OR sh.updated_at > $2)
		ORDER BY s.updated_at ASC, s.id ASC
	`, callerID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Store
	for rows.Next() {
		s, err := scanStore(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *s)
	}
	return out, rows.Err()
}

func ListForeignPriceRecordsForAccessibleListsSince(ctx context.Context, db DBTX, callerID uuid.UUID, since time.Time) ([]models.PriceRecord, error) {
	rows, err := db.Query(ctx, `
		SELECT DISTINCT pr.id, pr.owner_id, pr.offer_id, pr.price, pr.pack_size, pr.unit, pr.recorded_at, pr.recorded_by, pr.created_at
		FROM price_records pr
		JOIN offers o ON o.id = pr.offer_id AND o.deleted_at IS NULL
		JOIN list_items li ON li.good_id = o.good_id AND li.deleted_at IS NULL
		JOIN shopping_lists sl ON sl.id = li.list_id AND sl.deleted_at IS NULL
		LEFT JOIN list_shares sh
		    ON sh.list_id = sl.id AND sh.member_owner_id = $1 AND sh.revoked_at IS NULL
		WHERE pr.owner_id <> $1
		  AND (sl.owner_id = $1 OR sh.id IS NOT NULL)
		  AND (pr.created_at > $2 OR li.updated_at > $2 OR sh.updated_at > $2)
		ORDER BY pr.created_at ASC, pr.id ASC
	`, callerID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.PriceRecord
	for rows.Next() {
		pr, err := scanPriceRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *pr)
	}
	return out, rows.Err()
}
