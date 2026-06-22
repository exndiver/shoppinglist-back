package models

import (
	"time"

	"github.com/google/uuid"
)

// Good — товар в каталоге владельца (каноническая запись с учётом merge).
type Good struct {
	ID             uuid.UUID
	OwnerID        uuid.UUID
	CategoryID     *uuid.UUID
	Name           string
	NormalizedName string
	MergedInto     *uuid.UUID
	CreatedBy      *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Category struct {
	ID             uuid.UUID
	OwnerID        uuid.UUID
	Name           string
	NormalizedName string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Store struct {
	ID             uuid.UUID
	OwnerID        uuid.UUID
	Name           string
	NormalizedName string
	MergedInto     *uuid.UUID
	CreatedBy      *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Offer struct {
	ID        uuid.UUID
	OwnerID   uuid.UUID
	GoodID    uuid.UUID
	StoreID   uuid.UUID
	CreatedBy *string
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

type PriceRecord struct {
	ID         uuid.UUID
	OwnerID    uuid.UUID
	OfferID    uuid.UUID
	Price      float64
	PackSize   *float64
	Unit       *string
	RecordedAt time.Time
	RecordedBy *string
	CreatedAt  time.Time
}

type ShoppingList struct {
	ID        uuid.UUID
	OwnerID   uuid.UUID
	Name      string
	CreatedBy *string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ListItem struct {
	ID            uuid.UUID
	OwnerID       uuid.UUID
	ListID        uuid.UUID
	GoodID        uuid.UUID
	OfferID       *uuid.UUID
	Quantity      float64
	PriceSnapshot *float64
	IsPurchased   bool
	CreatedBy     *string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type OfferWithLatestPrice struct {
	OfferID uuid.UUID
	Store   Store
	Latest  *PriceSnapshot
}

type PriceSnapshot struct {
	Price    float64
	PackSize *float64
	Unit     *string
}

type GoodIdentity struct {
	Source     string
	ExternalID string
}

// ── List sharing ──────────────────────────────────────────────────────

// ShareAccess is the permission level granted to a list member.
type ShareAccess string

const (
	ShareAccessView ShareAccess = "view"
	ShareAccessEdit ShareAccess = "edit"
)

func (a ShareAccess) Valid() bool {
	return a == ShareAccessView || a == ShareAccessEdit
}

// InvitationStatus is the lifecycle state of a one-time invitation token.
type InvitationStatus string

const (
	InvitationPending  InvitationStatus = "pending"
	InvitationAccepted InvitationStatus = "accepted"
	InvitationRevoked  InvitationStatus = "revoked"
)

// ListShare grants MemberOwnerID access to the list owned by OwnerID.
type ListShare struct {
	ID            uuid.UUID
	ListID        uuid.UUID
	OwnerID       uuid.UUID // list author (sharer)
	MemberOwnerID uuid.UUID // recipient
	Access        ShareAccess
	DisplayName   *string // member's local rename; nil = use list.Name
	CreatedAt     time.Time
	UpdatedAt     time.Time
	RevokedAt     *time.Time
}

// ListInvitation is a single-use token that grants access to a list when accepted.
type ListInvitation struct {
	Token      string
	ListID     uuid.UUID
	OwnerID    uuid.UUID // creator (list author)
	Access     ShareAccess
	Status     InvitationStatus
	AcceptedBy *uuid.UUID
	AcceptedAt *time.Time
	CreatedAt  time.Time
	ExpiresAt  *time.Time
}
