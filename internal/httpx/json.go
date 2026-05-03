package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(v)
}

func WriteError(w http.ResponseWriter, status int, code, msg string) {
	WriteJSON(w, status, ErrorBody{Code: code, Message: msg})
}

func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var syntax *json.SyntaxError
		var ut *json.UnmarshalTypeError
		switch {
		case errors.Is(err, io.EOF):
			WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "empty body")
		case errors.As(err, &syntax), errors.As(err, &ut):
			WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid json")
		default:
			WriteError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		}
		return false
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "trailing json")
		return false
	}
	return true
}

func ParseUUID(w http.ResponseWriter, s string) (uuid.UUID, bool) {
	id, err := uuid.Parse(strings.TrimSpace(s))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid uuid")
		return uuid.Nil, false
	}
	return id, true
}
