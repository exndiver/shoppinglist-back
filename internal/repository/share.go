package repository

import (
	"context"
	"time"

	"github.com/exndiver/shopping-backend/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func scanListShare(row RowScanner) (*models.ListShare, error) {
	var s models.ListShare
	var access string
	var dn pgtype.Text
	var revoked pgtype.Timestamptz
	err := row.Scan(&s.ID, &s.ListID, &s.OwnerID, &s.MemberOwnerID, &access, &dn,
		&s.CreatedAt, &s.UpdatedAt, &revoked)
	if err != nil {
		return nil, err
	}
	s.Access = models.ShareAccess(access)
	if dn.Valid {
		v := dn.String
		s.DisplayName = &v
	}
	if revoked.Valid {
		t := revoked.Time
		s.RevokedAt = &t
	}
	return &s, nil
}

const shareCols = `id, list_id, owner_id, member_owner_id, access, display_name, created_at, updated_at, revoked_at`

// UpsertListShare creates or re-activates a membership. On a repeated accept it
// refreshes the access level and clears any prior revocation.
func UpsertListShare(ctx context.Context, db DBTX, s models.ListShare) error {
	_, err := db.Exec(ctx, `
		INSERT INTO list_shares (id, list_id, owner_id, member_owner_id, access)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (list_id, member_owner_id) DO UPDATE
		SET access = EXCLUDED.access,
		    revoked_at = NULL,
		    updated_at = now()
	`, s.ID, s.ListID, s.OwnerID, s.MemberOwnerID, string(s.Access))
	return err
}

// GetListShare returns the membership row regardless of revocation state.
func GetListShare(ctx context.Context, db DBTX, listID, memberOwnerID uuid.UUID) (*models.ListShare, error) {
	row := db.QueryRow(ctx, `SELECT `+shareCols+` FROM list_shares
		WHERE list_id = $1 AND member_owner_id = $2`, listID, memberOwnerID)
	s, err := scanListShare(row)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	return s, err
}

// ListSharesForList returns active members of a list (owner-facing view).
func ListSharesForList(ctx context.Context, db DBTX, listID uuid.UUID) ([]models.ListShare, error) {
	rows, err := db.Query(ctx, `SELECT `+shareCols+` FROM list_shares
		WHERE list_id = $1 AND revoked_at IS NULL
		ORDER BY created_at ASC`, listID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectShares(rows)
}

// ListSharesForMember returns the active memberships of a recipient.
func ListSharesForMember(ctx context.Context, db DBTX, memberOwnerID uuid.UUID) ([]models.ListShare, error) {
	rows, err := db.Query(ctx, `SELECT `+shareCols+` FROM list_shares
		WHERE member_owner_id = $1 AND revoked_at IS NULL
		ORDER BY created_at ASC`, memberOwnerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectShares(rows)
}

// ListSharesForMemberSince returns membership rows (including revoked ones, so the
// client can drop access) changed since the given time.
func ListSharesForMemberSince(ctx context.Context, db DBTX, memberOwnerID uuid.UUID, since time.Time) ([]models.ListShare, error) {
	rows, err := db.Query(ctx, `SELECT `+shareCols+` FROM list_shares
		WHERE member_owner_id = $1 AND updated_at > $2
		ORDER BY updated_at ASC, id ASC`, memberOwnerID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectShares(rows)
}

func collectShares(rows pgx.Rows) ([]models.ListShare, error) {
	var out []models.ListShare
	for rows.Next() {
		s, err := scanListShare(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *s)
	}
	return out, rows.Err()
}

// UpdateShareAccess changes a member's permission. Owner-scoped: only the list
// author (ownerID) may call it.
func UpdateShareAccess(ctx context.Context, db DBTX, ownerID, listID, memberOwnerID uuid.UUID, access models.ShareAccess) error {
	tag, err := db.Exec(ctx, `
		UPDATE list_shares SET access = $4, updated_at = now()
		WHERE list_id = $2 AND member_owner_id = $3 AND owner_id = $1 AND revoked_at IS NULL
	`, ownerID, listID, memberOwnerID, string(access))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// RevokeShare ends a membership (owner-scoped).
func RevokeShare(ctx context.Context, db DBTX, ownerID, listID, memberOwnerID uuid.UUID) error {
	tag, err := db.Exec(ctx, `
		UPDATE list_shares SET revoked_at = now(), updated_at = now()
		WHERE list_id = $2 AND member_owner_id = $3 AND owner_id = $1 AND revoked_at IS NULL
	`, ownerID, listID, memberOwnerID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// LeaveListShare lets a member end their own membership (member-initiated,
// e.g. when they delete their copy of a shared list).
func LeaveListShare(ctx context.Context, db DBTX, memberOwnerID, listID uuid.UUID) error {
	tag, err := db.Exec(ctx, `
		UPDATE list_shares SET revoked_at = now(), updated_at = now()
		WHERE list_id = $2 AND member_owner_id = $1 AND revoked_at IS NULL
	`, memberOwnerID, listID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateShareDisplayName lets a member rename their copy of a shared list.
func UpdateShareDisplayName(ctx context.Context, db DBTX, memberOwnerID, listID uuid.UUID, name *string) error {
	var dn any
	if name != nil {
		dn = *name
	}
	tag, err := db.Exec(ctx, `
		UPDATE list_shares SET display_name = $3, updated_at = now()
		WHERE list_id = $2 AND member_owner_id = $1 AND revoked_at IS NULL
	`, memberOwnerID, listID, dn)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// AccessForOwner reports the effective access an owner has to a list:
// the list author has implicit edit access; otherwise an active share decides.
func AccessForOwner(ctx context.Context, db DBTX, listID, ownerID uuid.UUID) (models.ShareAccess, bool, error) {
	// Author?
	var authorOwner uuid.UUID
	err := db.QueryRow(ctx, `SELECT owner_id FROM shopping_lists WHERE id = $1 AND `+sqlActive, listID).Scan(&authorOwner)
	if err == pgx.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if authorOwner == ownerID {
		return models.ShareAccessEdit, true, nil
	}
	// Member?
	var access string
	err = db.QueryRow(ctx, `SELECT access FROM list_shares
		WHERE list_id = $1 AND member_owner_id = $2 AND revoked_at IS NULL`, listID, ownerID).Scan(&access)
	if err == pgx.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return models.ShareAccess(access), true, nil
}
