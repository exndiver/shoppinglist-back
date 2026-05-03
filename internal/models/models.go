package models

import (
	"time"

	"github.com/google/uuid"
)

type Product struct {
	ID             uuid.UUID
	OwnerID        uuid.UUID
	Name           string
	NormalizedName string
	MergedInto     *uuid.UUID
	CreatedBy      *string
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
	ProductID uuid.UUID
	StoreID   uuid.UUID
	CreatedBy *string
	CreatedAt time.Time
	UpdatedAt time.Time
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
	ProductID     uuid.UUID
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
