package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"time"

	"github.com/exndiver/shopping-backend/internal/models"
	"github.com/exndiver/shopping-backend/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// AcceptResult is returned to a recipient who consumes an invitation.
type AcceptResult struct {
	Share models.ListShare    `json:"share"`
	List  models.ShoppingList `json:"list"`
}

// newInviteToken returns a 22-char url-safe opaque token (128 bits of entropy).
func newInviteToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// requireOwner verifies that ownerID is the author of listID.
func (s *Service) requireOwner(ctx context.Context, ownerID, listID uuid.UUID) error {
	if _, err := repository.GetShoppingList(ctx, s.Pool, ownerID, listID); err != nil {
		return err // ErrNotFound hides existence of others' lists
	}
	return nil
}

// invitationTTL bounds how long an unused invite link stays valid. Links are
// one-time already; the TTL closes the "link forgotten in a chat months ago"
// window. Accept checks expires_at atomically.
const invitationTTL = 7 * 24 * time.Hour

// CreateInvitation issues a one-time token for a list the caller owns.
func (s *Service) CreateInvitation(ctx context.Context, ownerID, listID uuid.UUID, access models.ShareAccess) (*models.ListInvitation, error) {
	if !access.Valid() {
		return nil, ErrBadRequest
	}
	if err := s.requireOwner(ctx, ownerID, listID); err != nil {
		return nil, err
	}
	token, err := newInviteToken()
	if err != nil {
		return nil, err
	}
	expires := time.Now().UTC().Add(invitationTTL)
	inv := models.ListInvitation{
		Token:     token,
		ListID:    listID,
		OwnerID:   ownerID,
		Access:    access,
		Status:    models.InvitationPending,
		ExpiresAt: &expires,
	}
	if err := repository.InsertInvitation(ctx, s.Pool, inv); err != nil {
		return nil, err
	}
	return repository.GetInvitation(ctx, s.Pool, token)
}

// AcceptInvitation consumes a token and grants the caller membership. The token
// is single-use: a second accept yields ErrConflict.
func (s *Service) AcceptInvitation(ctx context.Context, acceptedBy uuid.UUID, token string) (*AcceptResult, error) {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	inv, err := repository.AcceptInvitationTx(ctx, tx, token, acceptedBy)
	if err != nil {
		return nil, err // ErrNotFound | ErrConflict
	}

	// The author cannot accept their own invitation.
	if inv.OwnerID == acceptedBy {
		return nil, ErrBadRequest
	}

	share := models.ListShare{
		ID:            uuid.New(),
		ListID:        inv.ListID,
		OwnerID:       inv.OwnerID,
		MemberOwnerID: acceptedBy,
		Access:        inv.Access,
	}
	if err := repository.UpsertListShare(ctx, tx, share); err != nil {
		return nil, err
	}

	// List snapshot for the response (read as the author, bypassing member scope).
	list, err := repository.GetShoppingList(ctx, tx, inv.OwnerID, inv.ListID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	got, err := repository.GetListShare(ctx, s.Pool, inv.ListID, acceptedBy)
	if err != nil {
		return nil, err
	}
	return &AcceptResult{Share: *got, List: *list}, nil
}

// ListShares returns the active members of a list the caller owns.
func (s *Service) ListShares(ctx context.Context, ownerID, listID uuid.UUID) ([]models.ListShare, error) {
	if err := s.requireOwner(ctx, ownerID, listID); err != nil {
		return nil, err
	}
	return repository.ListSharesForList(ctx, s.Pool, listID)
}

// ListInvitations returns the invitations of a list the caller owns.
func (s *Service) ListInvitations(ctx context.Context, ownerID, listID uuid.UUID) ([]models.ListInvitation, error) {
	if err := s.requireOwner(ctx, ownerID, listID); err != nil {
		return nil, err
	}
	return repository.ListInvitationsForList(ctx, s.Pool, listID)
}

// ChangeShareAccess updates a member's permission (owner only).
func (s *Service) ChangeShareAccess(ctx context.Context, ownerID, listID, memberOwnerID uuid.UUID, access models.ShareAccess) error {
	if !access.Valid() {
		return ErrBadRequest
	}
	if err := s.requireOwner(ctx, ownerID, listID); err != nil {
		return err
	}
	return repository.UpdateShareAccess(ctx, s.Pool, ownerID, listID, memberOwnerID, access)
}

// RevokeShare ends a member's access (owner only).
func (s *Service) RevokeShare(ctx context.Context, ownerID, listID, memberOwnerID uuid.UUID) error {
	if err := s.requireOwner(ctx, ownerID, listID); err != nil {
		return err
	}
	return repository.RevokeShare(ctx, s.Pool, ownerID, listID, memberOwnerID)
}

// LeaveList lets a member remove themselves from a shared list. The owner's
// member list updates on next sync (the share row is revoked).
func (s *Service) LeaveList(ctx context.Context, memberOwnerID, listID uuid.UUID) error {
	return repository.LeaveListShare(ctx, s.Pool, memberOwnerID, listID)
}

// RevokeInvitation cancels a still-pending invitation (owner only).
func (s *Service) RevokeInvitation(ctx context.Context, ownerID uuid.UUID, token string) error {
	return repository.RevokeInvitation(ctx, s.Pool, ownerID, token)
}

// RenameSharedList sets a member's local display name for a shared list.
// Passing an empty name clears it (falls back to the canonical name).
func (s *Service) RenameSharedList(ctx context.Context, memberOwnerID, listID uuid.UUID, name *string) error {
	if _, _, err := s.accessOf(ctx, listID, memberOwnerID); err != nil {
		return err
	}
	return repository.UpdateShareDisplayName(ctx, s.Pool, memberOwnerID, listID, name)
}

// ── List-scoped sync (unified owner + member delivery) ────────────────────

func (s *Service) AccessibleListsSince(ctx context.Context, callerID uuid.UUID, since time.Time) ([]repository.AccessibleList, error) {
	return repository.ListAccessibleListsSince(ctx, s.Pool, callerID, since)
}

func (s *Service) AccessibleListItemsSince(ctx context.Context, callerID uuid.UUID, since time.Time) ([]models.ListItem, error) {
	return repository.ListAccessibleListItemsSince(ctx, s.Pool, callerID, since)
}

func (s *Service) DeletedListIDsForCallerSince(ctx context.Context, callerID uuid.UUID, since time.Time) ([]uuid.UUID, error) {
	return repository.ListDeletedListIDsForCallerSince(ctx, s.Pool, callerID, since)
}

func (s *Service) DeletedItemIDsForAccessibleListsSince(ctx context.Context, callerID uuid.UUID, since time.Time) ([]uuid.UUID, error) {
	return repository.ListDeletedItemIDsForAccessibleListsSince(ctx, s.Pool, callerID, since)
}

func (s *Service) ForeignGoodsForAccessibleListsSince(ctx context.Context, callerID uuid.UUID, since time.Time) ([]models.Good, error) {
	return repository.ListForeignGoodsForAccessibleListsSince(ctx, s.Pool, callerID, since)
}

func (s *Service) ForeignOffersForAccessibleListsSince(ctx context.Context, callerID uuid.UUID, since time.Time) ([]models.Offer, error) {
	return repository.ListForeignOffersForAccessibleListsSince(ctx, s.Pool, callerID, since)
}

func (s *Service) ForeignStoresForAccessibleListsSince(ctx context.Context, callerID uuid.UUID, since time.Time) ([]models.Store, error) {
	return repository.ListForeignStoresForAccessibleListsSince(ctx, s.Pool, callerID, since)
}

func (s *Service) ForeignPriceRecordsForAccessibleListsSince(ctx context.Context, callerID uuid.UUID, since time.Time) ([]models.PriceRecord, error) {
	return repository.ListForeignPriceRecordsForAccessibleListsSince(ctx, s.Pool, callerID, since)
}

// accessOf resolves an owner's effective access to a list, mapping "no access"
// to ErrNotFound so callers don't leak existence.
func (s *Service) accessOf(ctx context.Context, listID, ownerID uuid.UUID) (models.ShareAccess, bool, error) {
	access, ok, err := repository.AccessForOwner(ctx, s.Pool, listID, ownerID)
	if err != nil {
		return "", false, err
	}
	if !ok {
		return "", false, ErrNotFound
	}
	return access, true, nil
}

// SharesForMemberSince returns the caller's membership rows (incl. revocations)
// so a member learns when a list was un-shared and drops it locally.
func (s *Service) SharesForMemberSince(ctx context.Context, memberOwnerID uuid.UUID, since time.Time) ([]models.ListShare, error) {
	return repository.ListSharesForMemberSince(ctx, s.Pool, memberOwnerID, since)
}
