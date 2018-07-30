// Simple eventsource handler, for publishing Server-sent events (https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events).
package eventsource

import (
	"bytes"
	"fmt"
	"net/http"
	"sync"
)

type Source struct {
	lock        sync.Mutex
	subscribers map[chan []byte]bool
}

// New creates a new source which can be used to publish events
func New() *Source {
	return &Source{
		subscribers: make(map[chan []byte]bool),
	}
}

func (s *Source) subscribe() chan []byte {
	ch := make(chan []byte, 100)
	s.lock.Lock()
	s.subscribers[ch] = true
	s.lock.Unlock()
	return ch
}

func (s *Source) unsubscribe(ch chan []byte) {
	s.lock.Lock()
	delete(s.subscribers, ch)
	s.lock.Unlock()
}

// Send a message to all connected clients
func (s *Source) Publish(msg []byte) {
	msg = bytes.TrimSpace(msg)
	msg = bytes.Replace(msg, []byte("\n"), []byte("\ndata: "), -1)
	s.lock.Lock()
	for ch := range s.subscribers {
		select {
		case ch <- msg:
		default: // Dead or flooded client? discard!
		}
	}
	s.lock.Unlock()
}

// Source implements http.Handler. All connected clients will receive the
// published messages.
func (s *Source) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	f, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := s.subscribe()
	defer s.unsubscribe(ch)

	for {
		msg, ok := <-ch
		if !ok {
			return
		}

		_, err := fmt.Fprintf(w, "data: %s\n\n", msg)
		if err != nil {
			return
		}

		f.Flush()
	}
}
