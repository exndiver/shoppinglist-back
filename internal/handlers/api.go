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

	mux.HandleFunc("POST /list-items", a.postListItem)
	mux.HandleFunc("PATCH /list-items/{id}", a.patchListItem)
	mux.HandleFunc("GET /list-items", a.getListItems)

	mux.HandleFunc("POST /good-identities", a.postGoodIdentity)

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
	s := r.URL.Query().Get(key)
	if s == "" {
		return time.Time{}, true
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid timestamp format, use RFC3339")
		return time.Time{}, false
	}
	return t, true
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

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"exact":    mapSnippets(buckets.Exact),
		"prefix":   mapSnippets(buckets.Prefix),
		"contains": mapSnippets(buckets.Contains),
		"others":   mapSnippets(buckets.Others),
	})
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

func offerRespFrom(o models.Offer) map[string]any {
	return map[string]any{
		"id":         o.ID,
		"owner_id":   o.OwnerID,
		"good_id":    o.GoodID,
		"store_id":   o.StoreID,
		"created_at": o.CreatedAt,
		"updated_at": o.UpdatedAt,
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

	if since.IsZero() {
		httpx.WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "updated_since is required for offers list")
		return
	}

	items, err := a.svc.ListOffersSince(r.Context(), ownerID, since)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	out := make([]map[string]any, 0, len(items))
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
	out := make([]map[string]string, 0, len(items))
	for _, it := range items {
		out = append(out, map[string]string{
			"source":      it.Source,
			"external_id": it.ExternalID,
		})
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

func priceRecordRespFrom(pr models.PriceRecord) map[string]any {
	out := map[string]any{
		"id":          pr.ID,
		"owner_id":    pr.OwnerID,
		"offer_id":    pr.OfferID,
		"price":       pr.Price,
		"recorded_at": pr.RecordedAt,
		"created_at":  pr.CreatedAt,
	}
	if pr.PackSize != nil {
		out["pack_size"] = *pr.PackSize
	}
	if pr.Unit != nil {
		out["unit"] = *pr.Unit
	}
	return out
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
	out := make([]map[string]any, 0, len(items))
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
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"latest_price": nil})
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"latest_price": priceSnapDTO{
			Price:    pr.Price,
			PackSize: pr.PackSize,
			Unit:     pr.Unit,
		},
	})
}

func (a *API) getPriceRecords(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := a.owner(w, r)
	if !ok {
		return
	}

	since, ok := a.parseSince(w, r, "since")
	if !ok {
		return
	}

	if since.IsZero() {
		httpx.WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "since is required for price records")
		return
	}

	items, err := a.svc.ListPriceRecordsSince(r.Context(), ownerID, since)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	out := make([]map[string]any, 0, len(items))
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

func listRespFrom(l models.ShoppingList) map[string]any {
	return map[string]any{
		"id":         l.ID,
		"owner_id":   l.OwnerID,
		"name":       l.Name,
		"created_at": l.CreatedAt,
		"updated_at": l.UpdatedAt,
	}
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
		out := make([]map[string]any, 0, len(items))
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
	out := make([]map[string]any, 0, len(items))
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

	items := make([]map[string]any, 0, len(detail.Items))
	for _, it := range detail.Items {
		m := map[string]any{
			"id":           it.ID,
			"owner_id":     it.OwnerID,
			"list_id":      it.ListID,
			"good_id":      it.GoodID,
			"quantity":     it.Quantity,
			"is_purchased": it.IsPurchased,
			"good_name":    it.GoodName,
			"created_at":   it.CreatedAt,
			"updated_at":   it.UpdatedAt,
		}
		if it.OfferID != nil {
			m["offer_id"] = *it.OfferID
		}
		if it.PriceSnapshot != nil {
			m["price_snapshot"] = *it.PriceSnapshot
		}
		items = append(items, m)
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"id":         detail.List.ID,
		"owner_id":   detail.List.OwnerID,
		"name":       detail.List.Name,
		"created_at": detail.List.CreatedAt,
		"updated_at": detail.List.UpdatedAt,
		"items":      items,
	})
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
	it, err := a.svc.AddListItem(r.Context(), ownerID, req.ID, req.ListID, req.GoodID, req.OfferID, req.Quantity, req.PriceSnapshot, ptrCreatedBy(r))
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"id":             it.ID,
		"owner_id":       it.OwnerID,
		"list_id":        it.ListID,
		"good_id":        it.GoodID,
		"offer_id":       it.OfferID,
		"quantity":       it.Quantity,
		"price_snapshot": it.PriceSnapshot,
		"is_purchased":   it.IsPurchased,
		"good_name":      it.GoodName,
		"created_at":     it.CreatedAt,
		"updated_at":     it.UpdatedAt,
	})
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

	if since.IsZero() {
		httpx.WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "updated_since is required for list items")
		return
	}

	items, err := a.svc.ListListItemsSince(r.Context(), ownerID, since)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		out = append(out, map[string]any{
			"id":             it.ID,
			"owner_id":       it.OwnerID,
			"list_id":        it.ListID,
			"good_id":        it.GoodID,
			"offer_id":       it.OfferID,
			"quantity":       it.Quantity,
			"price_snapshot": it.PriceSnapshot,
			"is_purchased":   it.IsPurchased,
			"created_at":     it.CreatedAt,
			"updated_at":     it.UpdatedAt,
		})
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

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"id":             it.ID,
		"owner_id":       it.OwnerID,
		"list_id":        it.ListID,
		"good_id":        it.GoodID,
		"offer_id":       it.OfferID,
		"quantity":       it.Quantity,
		"price_snapshot": it.PriceSnapshot,
		"is_purchased":   it.IsPurchased,
		"good_name":      it.GoodName,
		"created_at":     it.CreatedAt,
		"updated_at":     it.UpdatedAt,
	})
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

func (a *API) postSyncBatch(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := a.owner(w, r)
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

	goods, err := a.svc.ListGoodsSince(r.Context(), ownerID, since)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	stores, err := a.svc.ListStoresSince(r.Context(), ownerID, since)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	offers, err := a.svc.ListOffersSince(r.Context(), ownerID, since)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	lists, err := a.svc.ListListsSince(r.Context(), ownerID, since)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	listItems, err := a.svc.ListListItemsSince(r.Context(), ownerID, since)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	categories, err := a.svc.ListCategoriesSince(r.Context(), ownerID, since)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	prices, err := a.svc.ListPriceRecordsSince(r.Context(), ownerID, since)
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
	respOffers := make([]map[string]any, 0, len(offers))
	for _, o := range offers {
		respOffers = append(respOffers, offerRespFrom(o))
	}
	respLists := make([]map[string]any, 0, len(lists))
	for _, l := range lists {
		respLists = append(respLists, listRespFrom(l))
	}
	respListItems := make([]map[string]any, 0, len(listItems))
	for _, it := range listItems {
		respListItems = append(respListItems, map[string]any{
			"id":             it.ID,
			"owner_id":       it.OwnerID,
			"list_id":        it.ListID,
			"good_id":        it.GoodID,
			"offer_id":       it.OfferID,
			"quantity":       it.Quantity,
			"price_snapshot": it.PriceSnapshot,
			"is_purchased":   it.IsPurchased,
			"created_at":     it.CreatedAt,
			"updated_at":     it.UpdatedAt,
		})
	}
	respCategories := make([]categoryResp, 0, len(categories))
	for _, c := range categories {
		respCategories = append(respCategories, categoryRespFrom(c))
	}
	respPrices := make([]map[string]any, 0, len(prices))
	for _, pr := range prices {
		respPrices = append(respPrices, priceRecordRespFrom(pr))
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"categories":    respCategories,
		"goods":         respGoods,
		"stores":        respStores,
		"offers":        respOffers,
		"lists":         respLists,
		"list_items":    respListItems,
		"price_records": respPrices,
		"server_time":   time.Now().UTC(),
	})
}
