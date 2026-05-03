package handlers

import (
	"errors"
	"net/http"

	"github.com/exndiver/shopping-backend/internal/httpx"
	"github.com/exndiver/shopping-backend/internal/service"
)

func writeSvcErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
	case errors.Is(err, service.ErrConflict):
		httpx.WriteError(w, http.StatusConflict, "CONFLICT", err.Error())
	case errors.Is(err, service.ErrBadRequest),
		errors.Is(err, service.ErrDepthExceeded):
		httpx.WriteError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
	default:
		httpx.WriteError(w, http.StatusInternalServerError, "INTERNAL", "internal error")
	}
}
