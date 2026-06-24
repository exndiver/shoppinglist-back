package repository

import (
	"context"
	"time"

	"github.com/exndiver/shopping-backend/internal/models"
	"github.com/google/uuid"
)

// Shared-data delta queries for a member (recipient) of one or more lists.
//
// These complement the owner-scoped sync: they expose lists owned by OTHER
// people that the member was granted access to, the items of those lists
// (regardless of which member added them), and the goods those items reference
// (so the client can import them into its local catalog).
//
// Membership is resolved through active rows in list_shares.

// ListSharedListsForMemberSince returns lists the member has active access to,
// changed since the given time (excludes the member's own lists).
func ListSharedListsForMemberSince(ctx context.Context, db DBTX, memberOwnerID uuid.UUID, since time.Time) ([]models.ShoppingList, error) {
	rows, err := db.Query(ctx, `
		SELECT sl.id, sl.owner_id, sl.name, sl.created_by, sl.created_at, sl.updated_at
		FROM shopping_lists sl
		INNER JOIN list_shares sh
		    ON sh.list_id = sl.id
		   AND sh.member_owner_id = $1
		   AND sh.revoked_at IS NULL
		-- Either the list changed, or the membership itself changed (e.g. a
		-- fresh accept) — in which case the member needs the full list.
		WHERE (sl.updated_at > $2 OR sh.updated_at > $2) AND sl.`+sqlActiveFrag+`
		ORDER BY sl.updated_at ASC, sl.id ASC
	`, memberOwnerID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.ShoppingList
	for rows.Next() {
		l, err := scanShoppingList(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *l)
	}
	return out, rows.Err()
}

// ListSharedListItemsForMemberSince returns items of every list the member has
// active access to — list-scoped, so items added by any participant are included.
func ListSharedListItemsForMemberSince(ctx context.Context, db DBTX, memberOwnerID uuid.UUID, since time.Time) ([]models.ListItem, error) {
	rows, err := db.Query(ctx, `
		SELECT li.id, li.owner_id, li.list_id, li.good_id, li.offer_id, li.quantity,
		       li.price_snapshot, li.is_purchased, li.created_by, li.created_at, li.updated_at
		FROM list_items li
		INNER JOIN list_shares sh
		    ON sh.list_id = li.list_id
		   AND sh.member_owner_id = $1
		   AND sh.revoked_at IS NULL
		INNER JOIN shopping_lists sl
		    ON sl.id = li.list_id AND sl.`+sqlActiveFrag+`
		-- Full dump of a list's items when the membership is fresh/changed.
		WHERE (li.updated_at > $2 OR sh.updated_at > $2) AND li.deleted_at IS NULL
		ORDER BY li.updated_at ASC, li.id ASC
	`, memberOwnerID, since)
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

// ListForeignGoodsForOwnedListsSince returns goods referenced by items on lists
// the given owner OWNS, but which the owner does not own themselves — i.e. goods
// a collaborator brought in by adding an item. The owner imports these into its
// catalog so the collaborator's items render. Gated on either the good or its
// linking item changing since, so a freshly-linked but old good is still sent.
func ListForeignGoodsForOwnedListsSince(ctx context.Context, db DBTX, ownerID uuid.UUID, since time.Time) ([]models.Good, error) {
	rows, err := db.Query(ctx, `
		SELECT DISTINCT g.id, g.owner_id, g.category_id, g.name, g.normalized_name,
		       g.merged_into, g.created_by, g.created_at, g.updated_at
		FROM goods g
		INNER JOIN list_items li ON li.good_id = g.id AND li.deleted_at IS NULL
		INNER JOIN shopping_lists sl
		    ON sl.id = li.list_id AND sl.owner_id = $1 AND sl.`+sqlActiveFrag+`
		WHERE g.owner_id <> $1
		  AND (g.updated_at > $2 OR li.updated_at > $2)
		  AND g.`+sqlActiveFrag+`
		ORDER BY g.updated_at ASC, g.id ASC
	`, ownerID, since)
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

// ListSharedGoodsForMemberSince returns the goods referenced by items of the
// member's shared lists, changed since the given time. These goods may be owned
// by any participant; the client imports them into its own catalog.
func ListSharedGoodsForMemberSince(ctx context.Context, db DBTX, memberOwnerID uuid.UUID, since time.Time) ([]models.Good, error) {
	rows, err := db.Query(ctx, `
		SELECT DISTINCT g.id, g.owner_id, g.category_id, g.name, g.normalized_name,
		       g.merged_into, g.created_by, g.created_at, g.updated_at
		FROM goods g
		INNER JOIN list_items li ON li.good_id = g.id AND li.deleted_at IS NULL
		INNER JOIN list_shares sh
		    ON sh.list_id = li.list_id
		   AND sh.member_owner_id = $1
		   AND sh.revoked_at IS NULL
		-- Full dump of referenced goods when the membership is fresh/changed.
		WHERE (g.updated_at > $2 OR sh.updated_at > $2) AND g.`+sqlActiveFrag+`
		ORDER BY g.updated_at ASC, g.id ASC
	`, memberOwnerID, since)
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

// ListSharedOffersForMemberSince returns offers for goods that appear on the
// member's shared lists, so the member can render per-store prices for shared
// items (and pick stores). Gated on a change to the offer or the membership.
func ListSharedOffersForMemberSince(ctx context.Context, db DBTX, memberOwnerID uuid.UUID, since time.Time) ([]models.Offer, error) {
	rows, err := db.Query(ctx, `
		SELECT DISTINCT o.id, o.owner_id, o.good_id, o.store_id, o.created_by, o.created_at, o.updated_at, o.deleted_at
		FROM offers o
		INNER JOIN list_items li ON li.good_id = o.good_id AND li.deleted_at IS NULL
		INNER JOIN list_shares sh
		    ON sh.list_id = li.list_id AND sh.member_owner_id = $1 AND sh.revoked_at IS NULL
		INNER JOIN shopping_lists sl ON sl.id = li.list_id AND sl.`+sqlActiveFrag+`
		WHERE (o.updated_at > $2 OR sh.updated_at > $2) AND o.`+sqlActiveFrag+`
		ORDER BY o.updated_at ASC, o.id ASC
	`, memberOwnerID, since)
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

// ListSharedStoresForMemberSince returns stores referenced by offers for goods
// on the member's shared lists, so store names render on shared items.
func ListSharedStoresForMemberSince(ctx context.Context, db DBTX, memberOwnerID uuid.UUID, since time.Time) ([]models.Store, error) {
	rows, err := db.Query(ctx, `
		SELECT DISTINCT s.id, s.owner_id, s.name, s.normalized_name, s.merged_into, s.created_by, s.created_at, s.updated_at
		FROM stores s
		INNER JOIN offers o ON o.store_id = s.id AND o.deleted_at IS NULL
		INNER JOIN list_items li ON li.good_id = o.good_id AND li.deleted_at IS NULL
		INNER JOIN list_shares sh
		    ON sh.list_id = li.list_id AND sh.member_owner_id = $1 AND sh.revoked_at IS NULL
		WHERE (s.updated_at > $2 OR sh.updated_at > $2) AND s.`+sqlActiveFrag+`
		ORDER BY s.updated_at ASC, s.id ASC
	`, memberOwnerID, since)
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

// ListSharedPriceRecordsForMemberSince returns price records of offers for goods
// on the member's shared lists, so shared items show a price.
func ListSharedPriceRecordsForMemberSince(ctx context.Context, db DBTX, memberOwnerID uuid.UUID, since time.Time) ([]models.PriceRecord, error) {
	rows, err := db.Query(ctx, `
		SELECT DISTINCT pr.id, pr.owner_id, pr.offer_id, pr.price, pr.pack_size, pr.unit, pr.recorded_at, pr.recorded_by, pr.created_at
		FROM price_records pr
		INNER JOIN offers o ON o.id = pr.offer_id AND o.deleted_at IS NULL
		INNER JOIN list_items li ON li.good_id = o.good_id AND li.deleted_at IS NULL
		INNER JOIN list_shares sh
		    ON sh.list_id = li.list_id AND sh.member_owner_id = $1 AND sh.revoked_at IS NULL
		WHERE (pr.created_at > $2 OR sh.updated_at > $2)
		ORDER BY pr.created_at ASC, pr.id ASC
	`, memberOwnerID, since)
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

// ListDeletedSharedListItemIDsForMemberSince returns ids of items tombstoned on
// lists the member has active access to, so the member's device drops them.
func ListDeletedSharedListItemIDsForMemberSince(ctx context.Context, db DBTX, memberOwnerID uuid.UUID, since time.Time) ([]uuid.UUID, error) {
	rows, err := db.Query(ctx, `
		SELECT li.id
		FROM list_items li
		INNER JOIN list_shares sh
		    ON sh.list_id = li.list_id
		   AND sh.member_owner_id = $1
		   AND sh.revoked_at IS NULL
		WHERE li.deleted_at IS NOT NULL AND li.updated_at > $2
		ORDER BY li.updated_at ASC, li.id ASC
	`, memberOwnerID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanUUIDs(rows)
}
