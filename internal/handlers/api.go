package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/exndiver/shopping-backend/internal/httpx"
	"github.com/exndiver/shopping-backend/internal/middleware"
	"github.com/exndiver/shopping-backend/internal/models"
	"github.com/exndiver/shopping-backend/internal/repository"
	"github.com/exndiver/shopping-backend/internal/service"
	"github.com/google/uuid"
)

type API struct {
	svc *service.Service
}

func NewAPI(svc *service.Service) http.Handler {
	a := &API{svc: svc}
	mux := http.NewServeMux()

	mux.HandleFunc("POST /goods", a.postUpsertGood)
	mux.HandleFunc("GET /goods", a.getGoods)
	mux.HandleFunc("POST /goods/merge", a.postMergeGoods)
	mux.HandleFunc("GET /goods/{id}/merge-candidates", a.getMergeCandidates)
	mux.HandleFunc("GET /goods/{id}/offers", a.getGoodOffers)
	mux.HandleFunc("GET /goods/{id}/identities", a.getGoodIdentities)
	mux.HandleFunc("GET /goods/{id}", a.getGood)

	mux.HandleFunc("POST /stores", a.postUpsertStore)
	mux.HandleFunc("GET /stores", a.getStores)

	mux.HandleFunc("POST /offers", a.postOffer)
	mux.HandleFunc("GET /offers", a.getOffers)

	mux.HandleFunc("POST /categories", a.postCategory)
	mux.HandleFunc("GET /categories", a.getCategories)
	mux.HandleFunc("GET /categories/{id}", a.getCategory)

	mux.HandleFunc("POST /price-records", a.postPriceRecord)
	mux.HandleFunc("GET /price-records", a.getPriceRecords)
	mux.HandleFunc("GET /offers/{id}/prices", a.getOfferPrices)
	mux.HandleFunc("GET /offers/{id}/price/latest", a.getOfferLatestPrice)

	mux.HandleFunc("POST /lists", a.postUpsertList)
	mux.HandleFunc("GET /lists", a.getLists)
	mux.HandleFunc("GET /lists/{id}", a.getList)
	mux.HandleFunc("DELETE /lists/{id}", a.deleteList)

	mux.HandleFunc("POST /list-items", a.postListItem)
	mux.HandleFunc("PATCH /list-items/{id}", a.patchListItem)
	mux.HandleFunc("DELETE /list-items/{id}", a.deleteListItem)
	mux.HandleFunc("GET /list-items", a.getListItems)

	mux.HandleFunc("POST /good-identities", a.postGoodIdentity)

	// List sharing.
	mux.HandleFunc("POST /lists/{id}/invitations", a.postCreateInvitation)
	mux.HandleFunc("GET /lists/{id}/invitations", a.getListInvitations)
	mux.HandleFunc("GET /lists/{id}/shares", a.getListShares)
	mux.HandleFunc("PATCH /lists/{id}/shares/{memberId}", a.patchShareAccess)
	mux.HandleFunc("DELETE /lists/{id}/shares/{memberId}", a.deleteShare)
	mux.HandleFunc("PATCH /lists/{id}/display-name", a.patchSharedListName)
	mux.HandleFunc("DELETE /lists/{id}/membership", a.deleteMembership)
	mux.HandleFunc("POST /invitations/{token}/accept", a.postAcceptInvitation)
	mux.HandleFunc("DELETE /invitations/{token}", a.deleteInvitation)

	mux.HandleFunc("POST /sync/batch", a.postSyncBatch)

	return mux
}

func (a *API) owner(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, ok := middleware.OwnerFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing owner context")
		return uuid.Nil, false
	}
	return id, true
}

func ptrCreatedBy(r *http.Request) *string {
	v := strings.TrimSpace(r.Header.Get(middleware.HeaderDeviceID))
	if v == "" {
		return nil
	}
	return &v
}

func (a *API) parseSince(w http.ResponseWriter, r *http.Request, key string) (time.Time, bool) {
	return a.parseSinceKeys(w, r, key)
}

// parseSinceKeys parses the first non-empty query param among keys (RFC3339).
// If all keys are absent, returns zero time and ok=true (full sync).
func (a *API) parseSinceKeys(w http.ResponseWriter, r *http.Request, keys ...string) (time.Time, bool) {
	for _, key := range keys {
		s := strings.TrimSpace(r.URL.Query().Get(key))
		if s == "" {
			continue
		}
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid timestamp format, use RFC3339")
			return time.Time{}, false
		}
		return t, true
	}
	return time.Time{}, true
}

type goodUpsertReq struct {
	ID         uuid.UUID  `json:"id"`
	CategoryID *uuid.UUID `json:"category_id"`
	Name       string     `json:"name"`
}

func (a *API) postUpsertGood(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := a.owner(w, r)
	if !ok {
		return
	}
	var req goodUpsertReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	g, err := a.svc.UpsertGood(r.Context(), ownerID, req.ID, req.CategoryID, req.Name, ptrCreatedBy(r))
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, goodRespFrom(*g))
}

func (a *API) getGoods(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := a.owner(w, r)
	if !ok {
		return
	}

	since, ok := a.parseSince(w, r, "updated_since")
	if !ok {
		return
	}

	if !since.IsZero() {
		items, err := a.svc.ListGoodsSince(r.Context(), ownerID, since)
		if err != nil {
			writeSvcErr(w, err)
			return
		}
		out := make([]goodResp, 0, len(items))
		for _, g := range items {
			out = append(out, goodRespFrom(g))
		}
		httpx.WriteJSON(w, http.StatusOK, out)
		return
	}

	q := r.URL.Query().Get("q")
	items, err := a.svc.SearchGoods(r.Context(), ownerID, q)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	out := make([]goodSnippet, 0, len(items))
	for _, g := range items {
		out = append(out, goodSnippet{ID: g.ID, Name: g.Name, CategoryID: g.CategoryID})
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (a *API) getGood(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := a.owner(w, r)
	if !ok {
		return
	}
	id, ok := httpx.ParseUUID(w, r.PathValue("id"))
	if !ok {
		return
	}
	g, err := a.svc.GetGoodCanonical(r.Context(), ownerID, id)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, goodRespFrom(*g))
}

type mergeGoodsReq struct {
	SourceGoodID uuid.UUID `json:"source_good_id"`
	TargetGoodID uuid.UUID `json:"target_good_id"`
}

func (a *API) postMergeGoods(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := a.owner(w, r)
	if !ok {
		return
	}
	var req mergeGoodsReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if err := a.svc.MergeGoods(r.Context(), ownerID, req.SourceGoodID, req.TargetGoodID); err != nil {
		writeSvcErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) getMergeCandidates(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := a.owner(w, r)
	if !ok {
		return
	}
	id, ok := httpx.ParseUUID(w, r.PathValue("id"))
	if !ok {
		return
	}
	q := r.URL.Query().Get("q")
	buckets, err := a.svc.MergeCandidates(r.Context(), ownerID, id, q)
	if err != nil {
		writeSvcErr(w, err)
		return
	}

	mapSnippets := func(gs []models.Good) []goodSnippet {
		out := make([]goodSnippet, 0, len(gs))
		for _, g := range gs {
			out = append(out, goodSnippet{ID: g.ID, Name: g.Name, CategoryID: g.CategoryID})
		}
		return out
	}

	httpx.WriteJSON(w, http.StatusOK, mergeCandidatesResp{
		Exact:    mapSnippets(buckets.Exact),
		Prefix:   mapSnippets(buckets.Prefix),
		Contains: mapSnippets(buckets.Contains),
		Others:   mapSnippets(buckets.Others),
	})
}

type mergeCandidatesResp struct {
	Exact    []goodSnippet `json:"exact"`
	Prefix   []goodSnippet `json:"prefix"`
	Contains []goodSnippet `json:"contains"`
	Others   []goodSnippet `json:"others"`
}

type goodSnippet struct {
	ID         uuid.UUID  `json:"id"`
	Name       string     `json:"name"`
	CategoryID *uuid.UUID `json:"category_id"`
}

type goodResp struct {
	ID             uuid.UUID  `json:"id"`
	OwnerID        uuid.UUID  `json:"owner_id"`
	CategoryID     *uuid.UUID `json:"category_id"`
	Name           string     `json:"name"`
	NormalizedName string     `json:"normalized_name"`
	MergedInto     *uuid.UUID `json:"merged_into,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func goodRespFrom(g models.Good) goodResp {
	return goodResp{
		ID:             g.ID,
		OwnerID:        g.OwnerID,
		CategoryID:     g.CategoryID,
		Name:           g.Name,
		NormalizedName: g.NormalizedName,
		MergedInto:     g.MergedInto,
		CreatedAt:      g.CreatedAt,
		UpdatedAt:      g.UpdatedAt,
	}
}

type storeUpsertReq struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

func (a *API) postUpsertStore(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := a.owner(w, r)
	if !ok {
		return
	}
	var req storeUpsertReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	st, err := a.svc.UpsertStore(r.Context(), ownerID, req.ID, req.Name, ptrCreatedBy(r))
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, storeRespFrom(*st))
}

func (a *API) getStores(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := a.owner(w, r)
	if !ok {
		return
	}

	since, ok := a.parseSince(w, r, "updated_since")
	if !ok {
		return
	}

	if !since.IsZero() {
		items, err := a.svc.ListStoresSince(r.Context(), ownerID, since)
		if err != nil {
			writeSvcErr(w, err)
			return
		}
		out := make([]storeResp, 0, len(items))
		for _, s := range items {
			out = append(out, storeRespFrom(s))
		}
		httpx.WriteJSON(w, http.StatusOK, out)
		return
	}

	q := r.URL.Query().Get("q")
	items, err := a.svc.SearchStores(r.Context(), ownerID, q)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	out := make([]storeSnippet, 0, len(items))
	for _, s := range items {
		out = append(out, storeSnippet{ID: s.ID, Name: s.Name})
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

type storeSnippet struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type storeResp struct {
	ID             uuid.UUID  `json:"id"`
	OwnerID        uuid.UUID  `json:"owner_id"`
	Name           string     `json:"name"`
	NormalizedName string     `json:"normalized_name"`
	MergedInto     *uuid.UUID `json:"merged_into,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func storeRespFrom(s models.Store) storeResp {
	return storeResp{
		ID:             s.ID,
		OwnerID:        s.OwnerID,
		Name:           s.Name,
		NormalizedName: s.NormalizedName,
		MergedInto:     s.MergedInto,
		CreatedAt:      s.CreatedAt,
		UpdatedAt:      s.UpdatedAt,
	}
}

type offerCreateReq struct {
	ID      uuid.UUID `json:"id"`
	GoodID  uuid.UUID `json:"good_id"`
	StoreID uuid.UUID `json:"store_id"`
}

func (a *API) postOffer(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := a.owner(w, r)
	if !ok {
		return
	}
	var req offerCreateReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	o, err := a.svc.UpsertOffer(r.Context(), ownerID, req.ID, req.GoodID, req.StoreID, ptrCreatedBy(r))
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, offerRespFrom(*o))
}

type offerResp struct {
	ID        uuid.UUID  `json:"id"`
	OwnerID   uuid.UUID  `json:"owner_id"`
	GoodID    uuid.UUID  `json:"good_id"`
	StoreID   uuid.UUID  `json:"store_id"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

func offerRespFrom(o models.Offer) offerResp {
	return offerResp{
		ID:        o.ID,
		OwnerID:   o.OwnerID,
		GoodID:    o.GoodID,
		StoreID:   o.StoreID,
		DeletedAt: o.DeletedAt,
		CreatedAt: o.CreatedAt,
		UpdatedAt: o.UpdatedAt,
	}
}

func (a *API) getOffers(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := a.owner(w, r)
	if !ok {
		return
	}

	since, ok := a.parseSince(w, r, "updated_since")
	if !ok {
		return
	}

	items, err := a.svc.ListOffersSince(r.Context(), ownerID, since)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	out := make([]offerResp, 0, len(items))
	for _, o := range items {
		out = append(out, offerRespFrom(o))
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (a *API) getGoodIdentities(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := a.owner(w, r)
	if !ok {
		return
	}
	id, ok := httpx.ParseUUID(w, r.PathValue("id"))
	if !ok {
		return
	}
	items, err := a.svc.ListGoodIdentities(r.Context(), ownerID, id)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	out := make([]identityResp, 0, len(items))
	for _, it := range items {
		out = append(out, identityResp{Source: it.Source, ExternalID: it.ExternalID})
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (a *API) getGoodOffers(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := a.owner(w, r)
	if !ok {
		return
	}
	id, ok := httpx.ParseUUID(w, r.PathValue("id"))
	if !ok {
		return
	}
	items, err := a.svc.ListOffersWithLatestPrices(r.Context(), ownerID, id)
	if err != nil {
		writeSvcErr(w, err)
		return
	}

	out := make([]offerPublicDTO, 0, len(items))
	for _, it := range items {
		dto := offerPublicDTO{
			OfferID: it.OfferID,
			Store: storeSnippet{
				ID:   it.Store.ID,
				Name: it.Store.Name,
			},
		}
		if it.Latest != nil {
			dto.LatestPrice = &priceSnapDTO{
				Price:    it.Latest.Price,
				PackSize: it.Latest.PackSize,
				Unit:     it.Latest.Unit,
			}
		}
		out = append(out, dto)
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

type offerPublicDTO struct {
	OfferID     uuid.UUID     `json:"offer_id"`
	Store       storeSnippet  `json:"store"`
	LatestPrice *priceSnapDTO `json:"latest_price,omitempty"`
}

type priceSnapDTO struct {
	Price    float64  `json:"price"`
	PackSize *float64 `json:"pack_size,omitempty"`
	Unit     *string  `json:"unit,omitempty"`
}

type priceRecordReq struct {
	ID         uuid.UUID `json:"id"`
	OfferID    uuid.UUID `json:"offer_id"`
	Price      float64   `json:"price"`
	PackSize   *float64  `json:"pack_size"`
	Unit       *string   `json:"unit"`
	RecordedAt time.Time `json:"recorded_at"`
}

func (a *API) postPriceRecord(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := a.owner(w, r)
	if !ok {
		return
	}
	var req priceRecordReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.RecordedAt.IsZero() {
		req.RecordedAt = time.Now().UTC()
	}
	pr, err := a.svc.AddPriceRecord(r.Context(), ownerID, req.ID, req.OfferID, req.Price, req.PackSize, req.Unit, req.RecordedAt, ptrCreatedBy(r))
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, priceRecordRespFrom(*pr))
}

type priceRecordResp struct {
	ID         uuid.UUID `json:"id"`
	OwnerID    uuid.UUID `json:"owner_id"`
	OfferID    uuid.UUID `json:"offer_id"`
	Price      float64   `json:"price"`
	PackSize   *float64  `json:"pack_size,omitempty"`
	Unit       *string   `json:"unit,omitempty"`
	RecordedAt time.Time `json:"recorded_at"`
	CreatedAt  time.Time `json:"created_at"`
}

func priceRecordRespFrom(pr models.PriceRecord) priceRecordResp {
	return priceRecordResp{
		ID:         pr.ID,
		OwnerID:    pr.OwnerID,
		OfferID:    pr.OfferID,
		Price:      pr.Price,
		PackSize:   pr.PackSize,
		Unit:       pr.Unit,
		RecordedAt: pr.RecordedAt,
		CreatedAt:  pr.CreatedAt,
	}
}

func (a *API) getOfferPrices(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := a.owner(w, r)
	if !ok {
		return
	}
	id, ok := httpx.ParseUUID(w, r.PathValue("id"))
	if !ok {
		return
	}
	items, err := a.svc.ListPriceHistory(r.Context(), ownerID, id)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	out := make([]priceRecordResp, 0, len(items))
	for _, pr := range items {
		out = append(out, priceRecordRespFrom(pr))
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (a *API) getOfferLatestPrice(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := a.owner(w, r)
	if !ok {
		return
	}
	id, ok := httpx.ParseUUID(w, r.PathValue("id"))
	if !ok {
		return
	}
	pr, ok, err := a.svc.LatestPrice(r.Context(), ownerID, id)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	if !ok {
		httpx.WriteJSON(w, http.StatusOK, latestPriceResp{})
		return
	}
	httpx.WriteJSON(w, http.StatusOK, latestPriceResp{
		LatestPrice: &priceSnapDTO{
			Price:    pr.Price,
			PackSize: pr.PackSize,
			Unit:     pr.Unit,
		},
	})
}

type latestPriceResp struct {
	LatestPrice *priceSnapDTO `json:"latest_price"`
}

func (a *API) getPriceRecords(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := a.owner(w, r)
	if !ok {
		return
	}

	since, ok := a.parseSinceKeys(w, r, "since", "updated_since")
	if !ok {
		return
	}

	items, err := a.svc.ListPriceRecordsSince(r.Context(), ownerID, since)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	out := make([]priceRecordResp, 0, len(items))
	for _, pr := range items {
		out = append(out, priceRecordRespFrom(pr))
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

type categoryReq struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

func (a *API) postCategory(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := a.owner(w, r)
	if !ok {
		return
	}
	var req categoryReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	c, err := a.svc.UpsertCategory(r.Context(), ownerID, req.ID, req.Name)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, categoryRespFrom(*c))
}

func (a *API) getCategories(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := a.owner(w, r)
	if !ok {
		return
	}

	since, ok := a.parseSince(w, r, "updated_since")
	if !ok {
		return
	}

	if !since.IsZero() {
		items, err := a.svc.ListCategoriesSince(r.Context(), ownerID, since)
		if err != nil {
			writeSvcErr(w, err)
			return
		}
		out := make([]categoryResp, 0, len(items))
		for _, c := range items {
			out = append(out, categoryRespFrom(c))
		}
		httpx.WriteJSON(w, http.StatusOK, out)
		return
	}

	q := r.URL.Query().Get("q")
	items, err := a.svc.SearchCategories(r.Context(), ownerID, q)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	out := make([]categorySnippet, 0, len(items))
	for _, c := range items {
		out = append(out, categorySnippet{ID: c.ID, Name: c.Name})
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

type categorySnippet struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

func (a *API) getCategory(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := a.owner(w, r)
	if !ok {
		return
	}
	id, ok := httpx.ParseUUID(w, r.PathValue("id"))
	if !ok {
		return
	}
	c, err := a.svc.GetCategory(r.Context(), ownerID, id)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, categoryRespFrom(*c))
}

type categoryResp struct {
	ID             uuid.UUID `json:"id"`
	OwnerID        uuid.UUID `json:"owner_id"`
	Name           string    `json:"name"`
	NormalizedName string    `json:"normalized_name"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func categoryRespFrom(c models.Category) categoryResp {
	return categoryResp{
		ID:             c.ID,
		OwnerID:        c.OwnerID,
		Name:           c.Name,
		NormalizedName: c.NormalizedName,
		CreatedAt:      c.CreatedAt,
		UpdatedAt:      c.UpdatedAt,
	}
}

type listUpsertReq struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

func (a *API) postUpsertList(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := a.owner(w, r)
	if !ok {
		return
	}
	var req listUpsertReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	l, err := a.svc.UpsertShoppingList(r.Context(), ownerID, req.ID, req.Name, ptrCreatedBy(r))
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, listRespFrom(*l))
}

type listResp struct {
	ID      uuid.UUID `json:"id"`
	OwnerID uuid.UUID `json:"owner_id"`
	Name    string    `json:"name"`
	// Caller's effective access: "owner" | "edit" | "view". Empty on endpoints
	// that don't compute it. Lets the client flag a list as shared-to-me
	// (owner_id != caller) and read-only (view).
	Access    string    `json:"access,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type listDetailResp struct {
	listResp
	Items []listItemResp `json:"items"`
}

type listItemResp struct {
	ID            uuid.UUID  `json:"id"`
	OwnerID       uuid.UUID  `json:"owner_id"`
	ListID        uuid.UUID  `json:"list_id"`
	GoodID        uuid.UUID  `json:"good_id"`
	OfferID       *uuid.UUID `json:"offer_id,omitempty"`
	Quantity      float64    `json:"quantity"`
	PriceSnapshot *float64   `json:"price_snapshot,omitempty"`
	IsPurchased   bool       `json:"is_purchased"`
	// Owner id of the participant who added the line (attribution on shared
	// lists). Absent on legacy rows stamped with a device id.
	CreatedBy *string   `json:"created_by,omitempty"`
	GoodName  string    `json:"good_name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func listItemRespFrom(it service.ListItemDetail) listItemResp {
	return listItemResp{
		ID:            it.ID,
		OwnerID:       it.OwnerID,
		ListID:        it.ListID,
		GoodID:        it.GoodID,
		OfferID:       it.OfferID,
		Quantity:      it.Quantity,
		PriceSnapshot: it.PriceSnapshot,
		IsPurchased:   it.IsPurchased,
		CreatedBy:     it.CreatedBy,
		GoodName:      it.GoodName,
		CreatedAt:     it.CreatedAt,
		UpdatedAt:     it.UpdatedAt,
	}
}

func listRespFrom(l models.ShoppingList) listResp {
	return listResp{
		ID:        l.ID,
		OwnerID:   l.OwnerID,
		Name:      l.Name,
		CreatedAt: l.CreatedAt,
		UpdatedAt: l.UpdatedAt,
	}
}

func listRespFromAccessible(l repository.AccessibleList) listResp {
	r := listRespFrom(l.ShoppingList)
	r.Access = l.Access
	return r
}

func (a *API) getLists(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := a.owner(w, r)
	if !ok {
		return
	}

	since, ok := a.parseSince(w, r, "updated_since")
	if !ok {
		return
	}

	if !since.IsZero() {
		items, err := a.svc.ListListsSince(r.Context(), ownerID, since)
		if err != nil {
			writeSvcErr(w, err)
			return
		}
		out := make([]listResp, 0, len(items))
		for _, l := range items {
			out = append(out, listRespFrom(l))
		}
		httpx.WriteJSON(w, http.StatusOK, out)
		return
	}

	items, err := a.svc.ListShoppingLists(r.Context(), ownerID)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	out := make([]listResp, 0, len(items))
	for _, l := range items {
		out = append(out, listRespFrom(l))
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (a *API) getList(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := a.owner(w, r)
	if !ok {
		return
	}
	id, ok := httpx.ParseUUID(w, r.PathValue("id"))
	if !ok {
		return
	}
	detail, err := a.svc.GetListDetail(r.Context(), ownerID, id)
	if err != nil {
		writeSvcErr(w, err)
		return
	}

	items := make([]listItemResp, 0, len(detail.Items))
	for _, it := range detail.Items {
		items = append(items, listItemRespFrom(it))
	}
	httpx.WriteJSON(w, http.StatusOK, listDetailResp{
		listResp: listRespFrom(detail.List),
		Items:    items,
	})
}

func (a *API) deleteList(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := a.owner(w, r)
	if !ok {
		return
	}
	id, ok := httpx.ParseUUID(w, r.PathValue("id"))
	if !ok {
		return
	}
	if err := a.svc.DeleteList(r.Context(), ownerID, id); err != nil {
		writeSvcErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type listItemCreateReq struct {
	ID            uuid.UUID  `json:"id"`
	ListID        uuid.UUID  `json:"list_id"`
	GoodID        uuid.UUID  `json:"good_id"`
	OfferID       *uuid.UUID `json:"offer_id"`
	Quantity      float64    `json:"quantity"`
	PriceSnapshot *float64   `json:"price_snapshot"`
}

func (a *API) postListItem(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := a.owner(w, r)
	if !ok {
		return
	}
	var req listItemCreateReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	// Attribute the line to the CALLER's owner id (not the device header):
	// collaborators map created_by onto their member labels, which are keyed by
	// owner id.
	cb := ownerID.String()
	it, err := a.svc.AddListItem(r.Context(), ownerID, req.ID, req.ListID, req.GoodID, req.OfferID, req.Quantity, req.PriceSnapshot, &cb)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, listItemRespFrom(*it))
}

func (a *API) getListItems(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := a.owner(w, r)
	if !ok {
		return
	}

	since, ok := a.parseSince(w, r, "updated_since")
	if !ok {
		return
	}

	items, err := a.svc.ListListItemsSince(r.Context(), ownerID, since)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	out := make([]listItemResp, 0, len(items))
	for _, it := range items {
		out = append(out, listItemRespFrom(service.ListItemDetail{ListItem: it}))
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (a *API) patchListItem(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := a.owner(w, r)
	if !ok {
		return
	}
	id, ok := httpx.ParseUUID(w, r.PathValue("id"))
	if !ok {
		return
	}

	raw := map[string]json.RawMessage{}
	if !httpx.DecodeJSON(w, r, &raw) {
		return
	}

	patch := repository.ListItemPatch{}
	if v, ok := raw["quantity"]; ok {
		var q float64
		if err := json.Unmarshal(v, &q); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid quantity")
			return
		}
		patch.Quantity = &q
	}
	if v, ok := raw["is_purchased"]; ok {
		var b bool
		if err := json.Unmarshal(v, &b); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid is_purchased")
			return
		}
		patch.IsPurchased = &b
	}
	if v, ok := raw["offer_id"]; ok {
		patch.OfferIDPresent = true
		if string(v) == "null" {
			patch.OfferID = nil
		} else {
			var oid uuid.UUID
			if err := json.Unmarshal(v, &oid); err != nil {
				httpx.WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid offer_id")
				return
			}
			patch.OfferID = &oid
		}
	}

	if err := a.svc.PatchListItem(r.Context(), ownerID, id, patch); err != nil {
		writeSvcErr(w, err)
		return
	}

	it, err := a.svc.GetListItemDetail(r.Context(), ownerID, id)
	if err != nil {
		writeSvcErr(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, listItemRespFrom(*it))
}

func (a *API) deleteListItem(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := a.owner(w, r)
	if !ok {
		return
	}
	id, ok := httpx.ParseUUID(w, r.PathValue("id"))
	if !ok {
		return
	}
	if err := a.svc.DeleteListItem(r.Context(), ownerID, id); err != nil {
		writeSvcErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type identityResp struct {
	Source     string `json:"source"`
	ExternalID string `json:"external_id"`
}

type identityReq struct {
	GoodID     uuid.UUID `json:"good_id"`
	ExternalID string    `json:"external_id"`
	Source     string    `json:"source"`
}

func (a *API) postGoodIdentity(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := a.owner(w, r)
	if !ok {
		return
	}
	var req identityReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	req.ExternalID = strings.TrimSpace(req.ExternalID)
	req.Source = strings.TrimSpace(req.Source)
	if req.ExternalID == "" || req.Source == "" {
		httpx.WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "external_id and source are required")
		return
	}
	if err := a.svc.UpsertGoodIdentity(r.Context(), ownerID, req.GoodID, req.ExternalID, req.Source); err != nil {
		writeSvcErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type syncBatchResp struct {
	// Caller's own catalog plus catalog rows owned by others that are referenced
	// by items on lists the caller can access (read-through imports).
	Categories   []categoryResp    `json:"categories"`
	Goods        []goodResp        `json:"goods"`
	Stores       []storeResp       `json:"stores"`
	Offers       []offerResp       `json:"offers"`
	PriceRecords []priceRecordResp `json:"price_records"`
	// Content of EVERY list the caller can access — lists they own and lists
	// shared to them — scoped by list access, not row owner_id. Each list row
	// carries the caller's access level.
	Lists     []listResp     `json:"lists"`
	ListItems []listItemResp `json:"list_items"`
	// The caller's membership rows (incl. revocations) so a member learns when a
	// list was un-shared and drops it locally.
	Shares []shareResp `json:"shares"`
	// Ids of items tombstoned on accessible lists so the client drops them.
	DeletedListItemIDs []string `json:"deleted_list_item_ids"`
	// Ids of tombstoned lists the caller owned or participated in, so every
	// device drops the list (and its items) locally.
	DeletedListIDs []string  `json:"deleted_list_ids"`
	ServerTime     time.Time `json:"server_time"`
}

func (a *API) postSyncBatch(w http.ResponseWriter, r *http.Request) {
	callerID, ok := a.owner(w, r)
	if !ok {
		return
	}

	since, ok := a.parseSince(w, r, "since")
	if !ok {
		return
	}

	if since.IsZero() {
		httpx.WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "since is required for batch sync")
		return
	}

	// Capture the watermark BEFORE running the queries: a row committed while
	// the batch is being assembled would carry updated_at < a post-query
	// timestamp yet be invisible to the queries — advancing the client past it
	// forever. Taking the time first means such rows are re-sent next sync
	// instead (upserts are idempotent, so the small overlap is harmless).
	serverTime := time.Now().UTC()

	// ── Catalog: caller's own, plus rows owned by others referenced by items on
	// the caller's accessible lists (so shared items render their good/price). ──
	goods, err := a.svc.ListGoodsSince(r.Context(), callerID, since)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	foreignGoods, err := a.svc.ForeignGoodsForAccessibleListsSince(r.Context(), callerID, since)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	goods = append(goods, foreignGoods...)

	stores, err := a.svc.ListStoresSince(r.Context(), callerID, since)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	foreignStores, err := a.svc.ForeignStoresForAccessibleListsSince(r.Context(), callerID, since)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	stores = append(stores, foreignStores...)

	offers, err := a.svc.ListOffersSince(r.Context(), callerID, since)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	foreignOffers, err := a.svc.ForeignOffersForAccessibleListsSince(r.Context(), callerID, since)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	offers = append(offers, foreignOffers...)

	prices, err := a.svc.ListPriceRecordsSince(r.Context(), callerID, since)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	foreignPrices, err := a.svc.ForeignPriceRecordsForAccessibleListsSince(r.Context(), callerID, since)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	prices = append(prices, foreignPrices...)

	categories, err := a.svc.ListCategoriesSince(r.Context(), callerID, since)
	if err != nil {
		writeSvcErr(w, err)
		return
	}

	// ── List content for every accessible list (own + shared), list-scoped. ──
	lists, err := a.svc.AccessibleListsSince(r.Context(), callerID, since)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	listItems, err := a.svc.AccessibleListItemsSince(r.Context(), callerID, since)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	deletedItemIDs, err := a.svc.DeletedItemIDsForAccessibleListsSince(r.Context(), callerID, since)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	deletedListIDs, err := a.svc.DeletedListIDsForCallerSince(r.Context(), callerID, since)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	// Membership rows for the caller (incl. revocations) so a member drops a list
	// that was un-shared from them.
	shares, err := a.svc.SharesForMemberSince(r.Context(), callerID, since)
	if err != nil {
		writeSvcErr(w, err)
		return
	}

	respGoods := make([]goodResp, 0, len(goods))
	for _, g := range goods {
		respGoods = append(respGoods, goodRespFrom(g))
	}
	respStores := make([]storeResp, 0, len(stores))
	for _, s := range stores {
		respStores = append(respStores, storeRespFrom(s))
	}
	respOffers := make([]offerResp, 0, len(offers))
	for _, o := range offers {
		respOffers = append(respOffers, offerRespFrom(o))
	}
	respPrices := make([]priceRecordResp, 0, len(prices))
	for _, pr := range prices {
		respPrices = append(respPrices, priceRecordRespFrom(pr))
	}
	respCategories := make([]categoryResp, 0, len(categories))
	for _, c := range categories {
		respCategories = append(respCategories, categoryRespFrom(c))
	}
	respLists := make([]listResp, 0, len(lists))
	for _, l := range lists {
		respLists = append(respLists, listRespFromAccessible(l))
	}
	respListItems := make([]listItemResp, 0, len(listItems))
	for _, it := range listItems {
		respListItems = append(respListItems, listItemRespFrom(service.ListItemDetail{ListItem: it}))
	}
	respShares := make([]shareResp, 0, len(shares))
	for _, s := range shares {
		respShares = append(respShares, shareRespFrom(s))
	}
	respDeletedItemIDs := make([]string, 0, len(deletedItemIDs))
	for _, id := range deletedItemIDs {
		respDeletedItemIDs = append(respDeletedItemIDs, id.String())
	}
	respDeletedListIDs := make([]string, 0, len(deletedListIDs))
	for _, id := range deletedListIDs {
		respDeletedListIDs = append(respDeletedListIDs, id.String())
	}

	httpx.WriteJSON(w, http.StatusOK, syncBatchResp{
		Categories:         respCategories,
		Goods:              respGoods,
		Stores:             respStores,
		Offers:             respOffers,
		PriceRecords:       respPrices,
		Lists:              respLists,
		ListItems:          respListItems,
		Shares:             respShares,
		DeletedListItemIDs: respDeletedItemIDs,
		DeletedListIDs:     respDeletedListIDs,
		ServerTime:         serverTime,
	})
}
