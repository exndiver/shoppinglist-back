package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/exndiver/shopping-backend/internal/httpx"
	"github.com/exndiver/shopping-backend/internal/models"
	"github.com/exndiver/shopping-backend/internal/service"
	"github.com/google/uuid"
)

// inviteBaseURL is the universal-link prefix for invitation tokens. The full
// link (e.g. https://exndiver.ovh/i/<token>) opens the app when installed and
// falls back to a web page otherwise.
const inviteBaseURL = "https://exndiver.ovh/i"

// ── DTOs ──────────────────────────────────────────────────────────────

type shareResp struct {
	ID            uuid.UUID  `json:"id"`
	ListID        uuid.UUID  `json:"list_id"`
	OwnerID       uuid.UUID  `json:"owner_id"`
	MemberOwnerID uuid.UUID  `json:"member_owner_id"`
	Access        string     `json:"access"`
	DisplayName   *string    `json:"display_name,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	RevokedAt     *time.Time `json:"revoked_at,omitempty"`
}

func shareRespFrom(s models.ListShare) shareResp {
	return shareResp{
		ID:            s.ID,
		ListID:        s.ListID,
		OwnerID:       s.OwnerID,
		MemberOwnerID: s.MemberOwnerID,
		Access:        string(s.Access),
		DisplayName:   s.DisplayName,
		CreatedAt:     s.CreatedAt,
		UpdatedAt:     s.UpdatedAt,
		RevokedAt:     s.RevokedAt,
	}
}

type invitationResp struct {
	Token      string     `json:"token"`
	URL        string     `json:"url"`
	ListID     uuid.UUID  `json:"list_id"`
	Access     string     `json:"access"`
	Status     string     `json:"status"`
	AcceptedBy *uuid.UUID `json:"accepted_by,omitempty"`
	AcceptedAt *time.Time `json:"accepted_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
}

func invitationRespFrom(inv models.ListInvitation) invitationResp {
	return invitationResp{
		Token:      inv.Token,
		URL:        inviteBaseURL + "/" + inv.Token,
		ListID:     inv.ListID,
		Access:     string(inv.Access),
		Status:     string(inv.Status),
		AcceptedBy: inv.AcceptedBy,
		AcceptedAt: inv.AcceptedAt,
		CreatedAt:  inv.CreatedAt,
		ExpiresAt:  inv.ExpiresAt,
	}
}

type acceptResultResp struct {
	Share shareResp `json:"share"`
	List  listResp  `json:"list"`
}

type createInvitationReq struct {
	Access string `json:"access"`
}

type changeAccessReq struct {
	Access string `json:"access"`
}

type renameSharedReq struct {
	Name *string `json:"name"`
}

// ── Handlers ──────────────────────────────────────────────────────────

func (a *API) postCreateInvitation(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := a.owner(w, r)
	if !ok {
		return
	}
	listID, ok := httpx.ParseUUID(w, r.PathValue("id"))
	if !ok {
		return
	}
	var req createInvitationReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	inv, err := a.svc.CreateInvitation(r.Context(), ownerID, listID, models.ShareAccess(req.Access))
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, invitationRespFrom(*inv))
}

func (a *API) getListShares(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := a.owner(w, r)
	if !ok {
		return
	}
	listID, ok := httpx.ParseUUID(w, r.PathValue("id"))
	if !ok {
		return
	}
	shares, err := a.svc.ListShares(r.Context(), ownerID, listID)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	out := make([]shareResp, 0, len(shares))
	for _, s := range shares {
		out = append(out, shareRespFrom(s))
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (a *API) getListInvitations(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := a.owner(w, r)
	if !ok {
		return
	}
	listID, ok := httpx.ParseUUID(w, r.PathValue("id"))
	if !ok {
		return
	}
	invs, err := a.svc.ListInvitations(r.Context(), ownerID, listID)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	out := make([]invitationResp, 0, len(invs))
	for _, inv := range invs {
		out = append(out, invitationRespFrom(inv))
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (a *API) patchShareAccess(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := a.owner(w, r)
	if !ok {
		return
	}
	listID, ok := httpx.ParseUUID(w, r.PathValue("id"))
	if !ok {
		return
	}
	memberID, ok := httpx.ParseUUID(w, r.PathValue("memberId"))
	if !ok {
		return
	}
	var req changeAccessReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if err := a.svc.ChangeShareAccess(r.Context(), ownerID, listID, memberID, models.ShareAccess(req.Access)); err != nil {
		writeSvcErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) deleteShare(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := a.owner(w, r)
	if !ok {
		return
	}
	listID, ok := httpx.ParseUUID(w, r.PathValue("id"))
	if !ok {
		return
	}
	memberID, ok := httpx.ParseUUID(w, r.PathValue("memberId"))
	if !ok {
		return
	}
	if err := a.svc.RevokeShare(r.Context(), ownerID, listID, memberID); err != nil {
		writeSvcErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) patchSharedListName(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := a.owner(w, r)
	if !ok {
		return
	}
	listID, ok := httpx.ParseUUID(w, r.PathValue("id"))
	if !ok {
		return
	}
	var req renameSharedReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if err := a.svc.RenameSharedList(r.Context(), ownerID, listID, req.Name); err != nil {
		writeSvcErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) postAcceptInvitation(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := a.owner(w, r)
	if !ok {
		return
	}
	token := r.PathValue("token")
	if token == "" {
		httpx.WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "missing token")
		return
	}
	res, err := a.svc.AcceptInvitation(r.Context(), ownerID, token)
	if err != nil {
		// A consumed/expired token is a 409 with a stable, client-localizable code.
		if errors.Is(err, service.ErrConflict) {
			httpx.WriteError(w, http.StatusConflict, "INVITATION_ALREADY_USED",
				"this invitation has already been accepted; please request a new one")
			return
		}
		writeSvcErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, acceptResultResp{
		Share: shareRespFrom(res.Share),
		List:  listRespFrom(res.List),
	})
}

func (a *API) deleteMembership(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := a.owner(w, r)
	if !ok {
		return
	}
	listID, ok := httpx.ParseUUID(w, r.PathValue("id"))
	if !ok {
		return
	}
	if err := a.svc.LeaveList(r.Context(), ownerID, listID); err != nil {
		writeSvcErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) deleteInvitation(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := a.owner(w, r)
	if !ok {
		return
	}
	token := r.PathValue("token")
	if token == "" {
		httpx.WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "missing token")
		return
	}
	if err := a.svc.RevokeInvitation(r.Context(), ownerID, token); err != nil {
		writeSvcErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
