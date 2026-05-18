package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestParseSinceKeys_absentMeansFullSync(t *testing.T) {
	api := &API{}
	req := httptest.NewRequest(http.MethodGet, "/offers", nil)
	w := httptest.NewRecorder()

	got, ok := api.parseSinceKeys(w, req, "updated_since")
	if !ok {
		t.Fatal("expected ok")
	}
	if !got.IsZero() {
		t.Fatalf("expected zero time, got %v", got)
	}
	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status %d", w.Code)
	}
}

func TestParseSinceKeys_incremental(t *testing.T) {
	api := &API{}
	since := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	req := httptest.NewRequest(http.MethodGet, "/offers?updated_since="+since.Format(time.RFC3339), nil)
	w := httptest.NewRecorder()

	got, ok := api.parseSinceKeys(w, req, "updated_since")
	if !ok {
		t.Fatal("expected ok")
	}
	if !got.Equal(since) {
		t.Fatalf("got %v want %v", got, since)
	}
}

func TestParseSinceKeys_priceAlias(t *testing.T) {
	api := &API{}
	since := time.Date(2021, 6, 1, 12, 0, 0, 0, time.UTC)
	req := httptest.NewRequest(http.MethodGet, "/price-records?updated_since="+since.Format(time.RFC3339), nil)
	w := httptest.NewRecorder()

	got, ok := api.parseSinceKeys(w, req, "since", "updated_since")
	if !ok || !got.Equal(since) {
		t.Fatalf("got %v ok=%v", got, ok)
	}
}
