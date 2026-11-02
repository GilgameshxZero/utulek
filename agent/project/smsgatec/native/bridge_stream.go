package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

func startBridgeStream(parent context.Context, g *Gateway) (*pushListener, error) {
	delay := 300 * time.Second
	if g.config.PushRetry > 0 {
		delay = time.Duration(g.config.PushRetry) * time.Second
	}
	return startBridgeStreamWithRetry(parent, g, delay)
}
func startBridgeStreamWithRetry(parent context.Context, g *Gateway, delay time.Duration) (*pushListener, error) {
	ctx, cancel := context.WithCancel(parent)
	transport := g.client.Transport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = 10 * time.Second
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	connect := func() (*http.Response, error) {
		req, e := http.NewRequestWithContext(ctx, "GET", g.config.Server+"/events", nil)
		if e != nil {
			return nil, e
		}
		req.SetBasicAuth(g.config.Username, g.config.Password)
		req.Header.Set("Accept", "text/event-stream")
		response, e := client.Do(req)
		if e != nil {
			return nil, e
		}
		if response.StatusCode != 200 || !strings.HasPrefix(response.Header.Get("Content-Type"), "text/event-stream") {
			response.Body.Close()
			return nil, errors.New("bridge event stream unavailable")
		}
		return response, nil
	}
	first, e := connect()
	if e != nil {
		cancel()
		transport.CloseIdleConnections()
		return nil, e
	}
	p := &pushListener{gateway: g, events: make(chan pushEvent, 64), done: make(chan error, 1), streamCancel: cancel}
	go func() {
		defer transport.CloseIdleConnections()
		response := first
		for {
			if response != nil {
				scanner := bufio.NewScanner(response.Body)
				scanner.Buffer(make([]byte, 4096), 32*1024*1024)
				for scanner.Scan() {
					line := scanner.Text()
					if !strings.HasPrefix(line, "data: ") {
						continue
					}
					var event pushEvent
					if json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event) != nil || (event.Event != "bridge:reset" && event.Event != "bridge:messages") {
						break
					}
					select {
					case p.events <- event:
					case <-ctx.Done():
						response.Body.Close()
						p.done <- nil
						return
					}
				}
				response.Body.Close()
				response = nil
			}
			select {
			case <-ctx.Done():
				p.done <- nil
				return
			case <-time.After(delay):
			}
			response, _ = connect()
		}
	}()
	return p, nil
}
