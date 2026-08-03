package hostit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientRegisterStatusRemove(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/status", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, StatusResponse{Connected: true, Server: "wss://x:7000", Version: "3.1.1"})
	})
	mux.HandleFunc("GET /api/v1/server", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, ServerInfoResponse{PublicAddr: "203.0.113.10", Connected: true})
	})
	mux.HandleFunc("POST /api/v1/register", func(w http.ResponseWriter, r *http.Request) {
		var req RegisterRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Name != "openinfer-api" || req.LocalPort != 1235 {
			t.Fatalf("bad register: %+v", req)
		}
		writeEnv(w, RegisterResponse{
			Status: "active", RouteName: req.Name,
			PublicAddr: ":9999", LocalAddr: "127.0.0.1:1235", Proto: "tcp",
		})
	})
	mux.HandleFunc("DELETE /api/v1/routes/openinfer-api", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, map[string]any{"ok": true})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := NewClient(srv.URL)
	st, err := c.Status(context.Background())
	if err != nil || !st.Connected {
		t.Fatalf("status: %+v err=%v", st, err)
	}
	info, err := c.ServerInfo(context.Background())
	if err != nil || info.PublicAddr != "203.0.113.10" {
		t.Fatalf("server info: %+v err=%v", info, err)
	}
	reg, err := c.Register(context.Background(), RegisterRequest{
		Name: DefaultRouteName, Proto: "tcp", LocalPort: 1235,
	})
	if err != nil || reg.PublicAddr == "" {
		t.Fatalf("register: %+v err=%v", reg, err)
	}
	if got := JoinPublicAddr(info.PublicAddr, reg.PublicAddr); got != "203.0.113.10:9999" {
		t.Fatalf("join: got %q", got)
	}
	if err := c.RemoveRoute(context.Background(), DefaultRouteName); err != nil {
		t.Fatal(err)
	}
}

func TestJoinPublicAddr(t *testing.T) {
	cases := []struct{ server, route, want string }{
		{"203.0.113.10", ":12345", "203.0.113.10:12345"},
		{"", ":12345", ":12345"},
		{"203.0.113.10", "1.2.3.4:99", "1.2.3.4:99"},
		{"203.0.113.10", "", ""},
	}
	for _, c := range cases {
		if got := JoinPublicAddr(c.server, c.route); got != c.want {
			t.Fatalf("JoinPublicAddr(%q,%q)=%q want %q", c.server, c.route, got, c.want)
		}
	}
}

func TestClientAgentDown(t *testing.T) {
	c := NewClient("http://127.0.0.1:1")
	_, err := c.Status(context.Background())
	if err == nil || !strings.Contains(err.Error(), "unreachable") {
		t.Fatalf("want unreachable, got %v", err)
	}
}

func writeEnv(w http.ResponseWriter, data any) {
	b, _ := json.Marshal(map[string]any{"status": "ok", "data": data})
	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
}
