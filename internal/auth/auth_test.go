package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGenerateUnique(t *testing.T) {
	a, err1 := Generate()
	b, err2 := Generate()
	if err1 != nil || err2 != nil {
		t.Fatal(err1, err2)
	}
	if a == b {
		t.Fatal("tokens must be unique")
	}
	if len(a) != 64 {
		t.Fatalf("token length = %d", len(a))
	}
}

func TestValid(t *testing.T) {
	tok, _ := Generate()
	if !tok.Valid(string(tok)) {
		t.Error("own token rejected")
	}
	if tok.Valid("wrong") || tok.Valid("") {
		t.Error("invalid token accepted")
	}
	var empty Token
	if empty.Valid("") {
		t.Error("empty token must never validate")
	}
}

func TestMiddleware(t *testing.T) {
	tok, _ := Generate()
	ok := false
	h := tok.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ok = true
	}))

	// No header.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusUnauthorized || ok {
		t.Error("unauthenticated request not rejected")
	}

	// Wrong token.
	rec = httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer nope")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Error("wrong token accepted")
	}

	// Correct token.
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+string(tok))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK && rec.Code != 0 {
		// handler didn't write; status recorder default is 200
	}
	if !ok {
		t.Error("valid token rejected")
	}
}
