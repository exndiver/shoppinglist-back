package repository

import (
	"context"

	"github.com/exndiver/shopping-backend/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const invitationCols = `token, list_id, owner_id, access, status, accepted_by, accepted_at, created_at, expires_at`

func scanInvitation(row RowScanner) (*models.ListInvitation, error) {
	var inv models.ListInvitation
	var access, status string
	var acceptedBy pgtype.UUID
	var acceptedAt, expiresAt pgtype.Timestamptz
	err := row.Scan(&inv.Token, &inv.ListID, &inv.OwnerID, &access, &status,
		&acceptedBy, &acceptedAt, &inv.CreatedAt, &expiresAt)
	if err != nil {
		return nil, err
	}
	inv.Access = models.ShareAccess(access)
	inv.Status = models.InvitationStatus(status)
	if acceptedBy.Valid {
		u := uuid.UUID(acceptedBy.Bytes)
		inv.AcceptedBy = &u
	}
	if acceptedAt.Valid {
		t := acceptedAt.Time
		inv.AcceptedAt = &t
	}
	if expiresAt.Valid {
		t := expiresAt.Time
		inv.ExpiresAt = &t
	}
	return &inv, nil
}

func InsertInvitation(ctx context.Context, db DBTX, inv models.ListInvitation) error {
	var expires any
	if inv.ExpiresAt != nil {
		expires = *inv.ExpiresAt
	}
	_, err := db.Exec(ctx, `
		INSERT INTO list_invitations (token, list_id, owner_id, access, status, expires_at)
		VALUES ($1, $2, $3, $4, 'pending', $5)
	`, inv.Token, inv.ListID, inv.OwnerID, string(inv.Access), expires)
	return err
}

func GetInvitation(ctx context.Context, db DBTX, token string) (*models.ListInvitation, error) {
	row := db.QueryRow(ctx, `SELECT `+invitationCols+` FROM list_invitations WHERE token = $1`, token)
	inv, err := scanInvitation(row)
	if err == pgx.ErrNoRows {
		return nil, ErrNotFound
	}
	return inv, err
}

// AcceptInvitationTx atomically flips a pending invitation to accepted and returns
// the updated row. If the token is missing the result is ErrNotFound; if it was
// already consumed/revoked or expired the result is ErrConflict. Must run inside
// the same tx that creates the membership.
func AcceptInvitationTx(ctx context.Context, db DBTX, token string, acceptedBy uuid.UUID) (*models.ListInvitation, error) {
	row := db.QueryRow(ctx, `
		UPDATE list_invitations
		SET status = 'accepted', accepted_by = $2, accepted_at = now()
		WHERE token = $1
		  AND status = 'pending'
		  AND (expires_at IS NULL OR expires_at > now())
		RETURNING `+invitationCols+`
	`, token, acceptedBy)
	inv, err := scanInvitation(row)
	if err == nil {
		return inv, nil
	}
	if err != pgx.ErrNoRows {
		return nil, err
	}
	// No row updated — distinguish "missing" from "already used/expired".
	existing, gerr := GetInvitation(ctx, db, token)
	if gerr != nil {
		return nil, gerr // ErrNotFound when truly absent
	}
	_ = existing
	return nil, ErrConflict
}

func RevokeInvitation(ctx context.Context, db DBTX, ownerID uuid.UUID, token string) error {
	tag, err := db.Exec(ctx, `
		UPDATE list_invitations SET status = 'revoked'
		WHERE token = $1 AND owner_id = $2 AND status = 'pending'
	`, token, ownerID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func ListInvitationsForList(ctx context.Context, db DBTX, listID uuid.UUID) ([]models.ListInvitation, error) {
	rows, err := db.Query(ctx, `SELECT `+invitationCols+` FROM list_invitations
		WHERE list_id = $1 ORDER BY created_at DESC`, listID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.ListInvitation
	for rows.Next() {
		inv, err := scanInvitation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *inv)
	}
	return out, rows.Err()
}
