// Package api exposes the authenticated local control API used by the QML
// desktop: REST for state and actions, a single WebSocket for live events.
package api

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
	"github.com/openinfer/openinfer-studio/internal/auth"
)

// Server is the loopback-only control API. The public OpenAI-compatible
// model API lives in internal/proxy and is deliberately separate.
type Server struct {
	token auth.Token
	hub   *Hub
	log   *slog.Logger
	mux   *http.ServeMux
	http  *http.Server
	port  int
}

func NewServer(token auth.Token, hub *Hub, log *slog.Logger) *Server {
	s := &Server{token: token, hub: hub, log: log, mux: http.NewServeMux()}
	return s
}

// Handle registers an authenticated REST route, e.g. "GET /api/v1/models".
func (s *Server) Handle(pattern string, h http.HandlerFunc) {
	s.mux.Handle(pattern, s.token.Middleware(withTimeouts(h)))
}

func withTimeouts(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Per-request ceiling; streaming endpoints manage their own contexts.
		ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
		defer cancel()
		h(w, r.WithContext(ctx))
	}
}

// wsAuthMessage is the required first WebSocket frame.
type wsAuthMessage struct {
	Token string `json:"token"`
}

var upgrader = websocket.Upgrader{
	// Loopback-only server; origin checks still applied defensively.
	CheckOrigin: func(r *http.Request) bool {
		h := r.Host
		host, _, err := net.SplitHostPort(h)
		if err != nil {
			host = h
		}
		return host == "127.0.0.1" || host == "::1" || host == "localhost"
	},
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
}

// HandleEvents registers the event WebSocket. The client must send
// {"token": "..."} as its first message within 5 seconds.
func (s *Server) HandleEvents(pattern string) {
	s.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			s.log.Debug("ws upgrade failed", "err", err)
			return
		}
		defer conn.Close()

		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		var authMsg wsAuthMessage
		if err := conn.ReadJSON(&authMsg); err != nil || !s.token.Valid(authMsg.Token) {
			_ = conn.WriteJSON(apiError{Error: "unauthorized"})
			return
		}
		_ = conn.SetReadDeadline(time.Time{})
		_ = conn.WriteJSON(Envelope{Version: 1, Event: "backend.ready", Timestamp: time.Now().UTC(), Payload: map[string]any{}})

		ch, unsub := s.hub.Subscribe()
		defer unsub()

		// Reader: only keeps the connection alive / detects close.
		done := make(chan struct{})
		go func() {
			defer close(done)
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					return
				}
			}
		}()

		for {
			select {
			case env, ok := <-ch:
				if !ok {
					return
				}
				_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := conn.WriteJSON(env); err != nil {
					return
				}
			case <-done:
				return
			}
		}
	})
}

// Start binds the server on 127.0.0.1:port only.
func (s *Server) Start(port int) error {
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return err
	}
	s.port = ln.Addr().(*net.TCPAddr).Port
	s.http = &http.Server{
		Handler:           s.mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		if err := s.http.Serve(ln); err != nil && err != http.ErrServerClosed {
			s.log.Error("control API server failed", "err", err)
		}
	}()
	return nil
}

// BoundPort returns the actual bound port (differs from the requested port
// when started with 0).
func (s *Server) BoundPort() int { return s.port }

func (s *Server) Shutdown(ctx context.Context) error {
	if s.http == nil {
		return nil
	}
	return s.http.Shutdown(ctx)
}
