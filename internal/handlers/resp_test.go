package handlers

import (
	"testing"
	"time"

	"github.com/exndiver/shopping-backend/internal/models"
	"github.com/exndiver/shopping-backend/internal/service"
	"github.com/google/uuid"
)

func TestOfferRespFrom_deletedAtIncluded(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	o := models.Offer{
		ID:        uuid.New(),
		OwnerID:   uuid.New(),
		GoodID:    uuid.New(),
		StoreID:   uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
		DeletedAt: &now,
	}
	resp := offerRespFrom(o)
	if resp.DeletedAt == nil {
		t.Fatal("expected DeletedAt to be set")
	}
	if !resp.DeletedAt.Equal(now) {
		t.Fatalf("DeletedAt mismatch: got %v want %v", resp.DeletedAt, now)
	}
}

func TestOfferRespFrom_deletedAtNil(t *testing.T) {
	o := models.Offer{ID: uuid.New(), OwnerID: uuid.New()}
	resp := offerRespFrom(o)
	if resp.DeletedAt != nil {
		t.Fatal("expected DeletedAt to be nil for active offer")
	}
}

func TestListItemRespFrom_fields(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	qty := 3.5
	detail := service.ListItemDetail{
		ListItem: models.ListItem{
			ID:          uuid.New(),
			OwnerID:     uuid.New(),
			ListID:      uuid.New(),
			GoodID:      uuid.New(),
			Quantity:    qty,
			IsPurchased: true,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		GoodName: "Молоко",
	}
	resp := listItemRespFrom(detail)
	if resp.GoodName != "Молоко" {
		t.Fatalf("GoodName mismatch: %q", resp.GoodName)
	}
	if resp.Quantity != qty {
		t.Fatalf("Quantity mismatch: %v", resp.Quantity)
	}
	if !resp.IsPurchased {
		t.Fatal("IsPurchased should be true")
	}
}

func TestPriceRecordRespFrom_optionalFields(t *testing.T) {
	ps := 500.0
	unit := "g"
	pr := models.PriceRecord{
		ID:       uuid.New(),
		OwnerID:  uuid.New(),
		OfferID:  uuid.New(),
		Price:    99.9,
		PackSize: &ps,
		Unit:     &unit,
	}
	resp := priceRecordRespFrom(pr)
	if resp.PackSize == nil || *resp.PackSize != ps {
		t.Fatal("PackSize mismatch")
	}
	if resp.Unit == nil || *resp.Unit != unit {
		t.Fatal("Unit mismatch")
	}

	// nil optional fields
	pr2 := models.PriceRecord{ID: uuid.New(), Price: 10.0}
	resp2 := priceRecordRespFrom(pr2)
	if resp2.PackSize != nil || resp2.Unit != nil {
		t.Fatal("expected nil optional fields")
	}
}
