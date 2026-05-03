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

	mux.HandleFunc("POST /products", a.postUpsertProduct)
	mux.HandleFunc("GET /products", a.getProductsSearch)
	mux.HandleFunc("POST /products/merge", a.postMergeProducts)
	mux.HandleFunc("GET /products/{id}/merge-candidates", a.getMergeCandidates)
	mux.HandleFunc("GET /products/{id}/offers", a.getProductOffers)
	mux.HandleFunc("GET /products/{id}", a.getProduct)

	mux.HandleFunc("POST /stores", a.postUpsertStore)
	mux.HandleFunc("GET /stores", a.getStoresSearch)

	mux.HandleFunc("POST /offers", a.postOffer)

	mux.HandleFunc("POST /price-records", a.postPriceRecord)
	mux.HandleFunc("GET /offers/{id}/prices", a.getOfferPrices)
	mux.HandleFunc("GET /offers/{id}/price/latest", a.getOfferLatestPrice)

	mux.HandleFunc("POST /lists", a.postUpsertList)
	mux.HandleFunc("GET /lists", a.getLists)
	mux.HandleFunc("GET /lists/{id}", a.getList)

	mux.HandleFunc("POST /list-items", a.postListItem)
	mux.HandleFunc("PATCH /list-items/{id}", a.patchListItem)

	mux.HandleFunc("POST /product-identities", a.postProductIdentity)

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

type productUpsertReq struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

func (a *API) postUpsertProduct(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := a.owner(w, r)
	if !ok {
		return
	}
	var req productUpsertReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	p, err := a.svc.UpsertProduct(r.Context(), ownerID, req.ID, req.Name, ptrCreatedBy(r))
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, productRespFrom(*p))
}

func (a *API) getProductsSearch(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := a.owner(w, r)
	if !ok {
		return
	}
	q := r.URL.Query().Get("q")
	items, err := a.svc.SearchProducts(r.Context(), ownerID, q)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	out := make([]productSnippet, 0, len(items))
	for _, p := range items {
		out = append(out, productSnippet{ID: p.ID, Name: p.Name})
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (a *API) getProduct(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := a.owner(w, r)
	if !ok {
		return
	}
	id, ok := httpx.ParseUUID(w, r.PathValue("id"))
	if !ok {
		return
	}
	p, err := a.svc.GetProductCanonical(r.Context(), ownerID, id)
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, productRespFrom(*p))
}

type mergeProductsReq struct {
	SourceProductID uuid.UUID `json:"source_product_id"`
	TargetProductID uuid.UUID `json:"target_product_id"`
}

func (a *API) postMergeProducts(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := a.owner(w, r)
	if !ok {
		return
	}
	var req mergeProductsReq
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if err := a.svc.MergeProducts(r.Context(), ownerID, req.SourceProductID, req.TargetProductID); err != nil {
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

	mapSnippets := func(ps []models.Product) []productSnippet {
		out := make([]productSnippet, 0, len(ps))
		for _, p := range ps {
			out = append(out, productSnippet{ID: p.ID, Name: p.Name})
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

type productSnippet struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type productResp struct {
	ID             uuid.UUID  `json:"id"`
	OwnerID        uuid.UUID  `json:"owner_id"`
	Name           string     `json:"name"`
	NormalizedName string     `json:"normalized_name"`
	MergedInto     *uuid.UUID `json:"merged_into,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func productRespFrom(p models.Product) productResp {
	return productResp{
		ID:             p.ID,
		OwnerID:        p.OwnerID,
		Name:           p.Name,
		NormalizedName: p.NormalizedName,
		MergedInto:     p.MergedInto,
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
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

func (a *API) getStoresSearch(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := a.owner(w, r)
	if !ok {
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
	ID        uuid.UUID `json:"id"`
	ProductID uuid.UUID `json:"product_id"`
	StoreID   uuid.UUID `json:"store_id"`
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
	o, err := a.svc.CreateOffer(r.Context(), ownerID, req.ID, req.ProductID, req.StoreID, ptrCreatedBy(r))
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
		"product_id": o.ProductID,
		"store_id":   o.StoreID,
		"created_at": o.CreatedAt,
		"updated_at": o.UpdatedAt,
	}
}

func (a *API) getProductOffers(w http.ResponseWriter, r *http.Request) {
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
			"product_id":   it.ProductID,
			"quantity":     it.Quantity,
			"is_purchased": it.IsPurchased,
			"product_name": it.ProductName,
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
	ProductID     uuid.UUID  `json:"product_id"`
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
	it, err := a.svc.AddListItem(r.Context(), ownerID, req.ID, req.ListID, req.ProductID, req.OfferID, req.Quantity, req.PriceSnapshot, ptrCreatedBy(r))
	if err != nil {
		writeSvcErr(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"id":             it.ID,
		"owner_id":       it.OwnerID,
		"list_id":        it.ListID,
		"product_id":     it.ProductID,
		"offer_id":       it.OfferID,
		"quantity":       it.Quantity,
		"price_snapshot": it.PriceSnapshot,
		"is_purchased":   it.IsPurchased,
		"product_name":   it.ProductName,
		"created_at":     it.CreatedAt,
		"updated_at":     it.UpdatedAt,
	})
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
		"product_id":     it.ProductID,
		"offer_id":       it.OfferID,
		"quantity":       it.Quantity,
		"price_snapshot": it.PriceSnapshot,
		"is_purchased":   it.IsPurchased,
		"product_name":   it.ProductName,
		"created_at":     it.CreatedAt,
		"updated_at":     it.UpdatedAt,
	})
}

type identityReq struct {
	ProductID  uuid.UUID `json:"product_id"`
	ExternalID string    `json:"external_id"`
	Source     string    `json:"source"`
}

func (a *API) postProductIdentity(w http.ResponseWriter, r *http.Request) {
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
	if err := a.svc.UpsertProductIdentity(r.Context(), ownerID, req.ProductID, req.ExternalID, req.Source); err != nil {
		writeSvcErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
