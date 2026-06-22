package handlers

import (
	"testing"
	"time"

	"github.com/exndiver/shopping-backend/internal/models"
	"github.com/google/uuid"
)

func TestInvitationRespFrom_buildsUniversalLink(t *testing.T) {
	inv := models.ListInvitation{
		Token:   "Ab3x9K2mAb3x9K2mAb3x9K",
		ListID:  uuid.New(),
		OwnerID: uuid.New(),
		Access:  models.ShareAccessEdit,
		Status:  models.InvitationPending,
	}
	got := invitationRespFrom(inv)
	want := "https://exndiver.ovh/i/" + inv.Token
	if got.URL != want {
		t.Errorf("URL = %q, want %q", got.URL, want)
	}
	if got.Access != "edit" {
		t.Errorf("Access = %q, want edit", got.Access)
	}
	if got.Status != "pending" {
		t.Errorf("Status = %q, want pending", got.Status)
	}
}

func TestShareRespFrom_optionalFields(t *testing.T) {
	name := "WishList Alex"
	revoked := time.Now().UTC()
	s := models.ListShare{
		ID:            uuid.New(),
		ListID:        uuid.New(),
		OwnerID:       uuid.New(),
		MemberOwnerID: uuid.New(),
		Access:        models.ShareAccessView,
		DisplayName:   &name,
		RevokedAt:     &revoked,
	}
	got := shareRespFrom(s)
	if got.DisplayName == nil || *got.DisplayName != name {
		t.Errorf("DisplayName = %v, want %q", got.DisplayName, name)
	}
	if got.RevokedAt == nil {
		t.Error("RevokedAt should be carried through")
	}
	if got.Access != "view" {
		t.Errorf("Access = %q, want view", got.Access)
	}

	// Nil optionals stay nil.
	bare := shareRespFrom(models.ListShare{Access: models.ShareAccessEdit})
	if bare.DisplayName != nil || bare.RevokedAt != nil {
		t.Error("nil optionals must remain nil")
	}
}
