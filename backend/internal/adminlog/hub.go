package adminlog

import (
	"sync"
)

const clientBuffer = 64

// Hub is an in-memory SSE event broker (single Railway instance).
type Hub struct {
	mu      sync.RWMutex
	clients map[chan Event]struct{}
}

func NewHub() *Hub {
	return &Hub{clients: make(map[chan Event]struct{})}
}

// Subscribe returns a receive-only channel and an unsubscribe function.
func (h *Hub) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, clientBuffer)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	unsub := func() {
		h.mu.Lock()
		delete(h.clients, ch)
		close(ch)
		h.mu.Unlock()
	}
	return ch, unsub
}

// Broadcast delivers an event to all connected clients (non-blocking per client).
func (h *Hub) Broadcast(ev Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.clients {
		select {
		case ch <- ev:
		default:
			// slow client — drop rather than block everyone
		}
	}
}
