package main

import (
	_ "embed"
	"log"
	"net/http"

	"box/browser"
	"github.com/gorilla/websocket"
)

//go:embed browser_view.html
var browserViewHTML []byte

var brow *browser.Browser

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
log.Printf("browser/ws request from %s auth=%v", r.RemoteAddr, r.Header.Get("Authorization") != "")
	if brow == nil {
		http.Error(w, "browser not running", 503)
		return
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

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
				conn.WriteMessage(websocket.TextMessage, resp)
			}
		}
	}()

	frameCh := brow.Broadcaster.Subscribe()
	defer brow.Broadcaster.Unsubscribe(frameCh)
	for frame := range frameCh {
		if err := conn.WriteMessage(websocket.BinaryMessage, frame); err != nil {
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
