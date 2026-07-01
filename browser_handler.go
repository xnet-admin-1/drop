package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"box/browser"
	"github.com/gorilla/websocket"
)

//go:embed browser_view.html
var browserViewHTML []byte

var brow *browser.Browser

// Browser events channel for user actions
var browserEvents = make(chan string, 100)

// getBrowserEvents returns browser events as SSE
func getBrowserEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	for {
		select {
		case ev := <-browserEvents:
			fmt.Fprintf(w, "data: %s\n\n", ev)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

// handleBrowserEvent receives user actions from browser_view and notifies chat
func handleBrowserEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var data map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	action, _ := data["action"].(string)
	var msg string

	switch action {
	case "click":
		x, _ := data["x"].(float64)
		y, _ := data["y"].(float64)
		button, _ := data["button"].(string)
		msg = fmt.Sprintf("🖱 You clicked at (%.0f, %.0f) [%s]", x, y, button)
	case "navigate":
		url, _ := data["url"].(string)
		msg = fmt.Sprintf("🌐 You navigated to: %s", url)
	case "back":
		msg = "◀️ You went back"
	case "forward":
		msg = "▶️ You went forward"
	case "reload":
		msg = "🔄 You reloaded the page"
	case "scroll":
		deltaY, _ := data["deltaY"].(float64)
		direction := "down"
		if deltaY < 0 {
			direction = "up"
		}
		msg = fmt.Sprintf("📜 You scrolled %s", direction)
	case "keydown", "keyup":
		key, _ := data["key"].(string)
		msg = fmt.Sprintf("⌨️ You pressed: %s", key)
	case "type":
		text, _ := data["text"].(string)
		msg = fmt.Sprintf("⌨️ You typed: %s", text)
	case "mousemove":
		// Too noisy, skip
		w.Write([]byte(`{"ok":true}`))
		return
	default:
		msg = fmt.Sprintf("You performed action: %s", action)
	}

	// Send to event channel
	select {
	case browserEvents <- msg:
	default:
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": msg})
}

func initBrowser() {
	var err error
	brow, err = browser.LaunchBrowser()
	if err != nil {
		log.Printf("Browser launch failed: %v", err)
	}
}

func handleBrowserView(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(browserViewHTML)
}

func handleBrowserWS(w http.ResponseWriter, r *http.Request) {
	log.Printf("browser WS connect from %s", r.RemoteAddr)
	if brow == nil {
		http.Error(w, "browser not running", 503)
		return
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	var wmu sync.Mutex

	go func() {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			resp, err := brow.HandleInput(msg)
			if err != nil {
				continue
			}
			if resp != nil {
				wmu.Lock()
				conn.WriteMessage(websocket.TextMessage, resp)
				wmu.Unlock()
			}
		}
	}()

	frameCh := brow.Broadcaster.Subscribe()
	defer brow.Broadcaster.Unsubscribe(frameCh)
	for frame := range frameCh {
		wmu.Lock()
		err := conn.WriteMessage(websocket.BinaryMessage, frame)
		wmu.Unlock()
		if err != nil {
			return
		}
	}
}

func handleBrowserScreenshot(w http.ResponseWriter, r *http.Request) {
	if brow == nil {
		http.Error(w, "browser not running", 503)
		return
	}
	buf, err := brow.Screenshot()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Write(buf)
}

func ensureBrowser() {
	if brow != nil {
		// Test if browser is still alive
		if _, err := brow.GetURL(); err == nil {
			return
		}
		brow.Close()
	}
	initBrowser()
}
