package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func (s *Bridge) eventStream(w http.ResponseWriter, r *http.Request) {
	controller := http.NewResponseController(w)
	if e := controller.SetWriteDeadline(time.Time{}); e != nil {
		apiError(w, 500, "streaming unavailable")
		return
	}
	updates, unsubscribe := s.store.subscribe()
	defer unsubscribe()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	write := func(event string, rows []Message) bool {
		frame := struct {
			Event    string    `json:"event"`
			Messages []Message `json:"messages,omitempty"`
		}{event, rows}
		data, e := json.Marshal(frame)
		if e != nil {
			return false
		}
		if _, e = fmt.Fprintf(w, "data: %s\n\n", data); e != nil {
			return false
		}
		return controller.Flush() == nil
	}
	// Every new connection reconciles the local journal; this covers disconnects
	// and process restarts without needing to retain an unbounded event log.
	if !write("bridge:reset", nil) {
		return
	}
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case rows, ok := <-updates:
			if !ok {
				return
			}
			if !write("bridge:messages", rows) {
				return
			}
		case <-ticker.C:
			if _, e := fmt.Fprint(w, ": keepalive\n\n"); e != nil {
				return
			}
			if controller.Flush() != nil {
				return
			}
		}
	}
}
