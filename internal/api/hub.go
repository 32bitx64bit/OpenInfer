package api

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// Envelope is the versioned event frame sent over the event WebSocket.
type Envelope struct {
	Version   int       `json:"version"`
	Event     string    `json:"event"`
	Timestamp time.Time `json:"timestamp"`
	Payload   any       `json:"payload"`
}

// Hub fans events out to all connected WebSocket clients.
type Hub struct {
	mu   sync.Mutex
	subs map[chan Envelope]struct{}
}

func NewHub() *Hub { return &Hub{subs: map[chan Envelope]struct{}{}} }

// Publish broadcasts an event; slow consumers drop events rather than block
// the publisher (clients reload authoritative state after reconnect anyway).
func (h *Hub) Publish(event string, payload any) {
	env := Envelope{Version: 1, Event: event, Timestamp: time.Now().UTC(), Payload: payload}
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- env:
		default:
		}
	}
}

// Subscribe registers a buffered listener.
func (h *Hub) Subscribe() (chan Envelope, func()) {
	ch := make(chan Envelope, 512)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		delete(h.subs, ch)
		h.mu.Unlock()
		close(ch)
	}
}

func newID() string { return uuid.NewString() }
