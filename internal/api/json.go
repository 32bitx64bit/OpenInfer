package api

import (
	"encoding/json"
	"errors"
	"net/http"
)

const maxRequestBody = 4 << 20 // 4 MiB; generation params & prompts, not weights

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// apiError is the stable error envelope returned by every endpoint.
type apiError struct {
	Error  string `json:"error"`
	Detail string `json:"detail,omitempty"`
}

func writeErr(w http.ResponseWriter, status int, msg string, err error) {
	ae := apiError{Error: msg}
	if err != nil {
		ae.Detail = err.Error()
	}
	writeJSON(w, status, ae)
}

// decodeJSON reads a bounded JSON body and rejects trailing garbage.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body", err)
		return false
	}
	if dec.More() {
		writeErr(w, http.StatusBadRequest, "unexpected trailing data", errors.New("multiple JSON values"))
		return false
	}
	return true
}
