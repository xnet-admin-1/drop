package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

type Browser struct {
	mu     sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
	Broadcaster *FrameBroadcaster
}

const UserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.7778.96 Safari/537.36"

func LaunchBrowser() (*Browser, error) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("disable-infobars", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("window-size", "1280,720"),
		chromedp.UserAgent(UserAgent),
	)
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, cancel := chromedp.NewContext(allocCtx)
	if err := chromedp.Run(ctx,
		chromedp.Navigate("https://search.xnet.ngo"),
	); err != nil {
		cancel()
		allocCancel()
		return nil, err
	}
	b := &Browser{ctx: ctx, cancel: func() { cancel(); allocCancel() }, Broadcaster: NewBroadcaster()}
	log.Println("screencast goroutine starting")
	go b.screencast()
	return b, nil
}

func (b *Browser) screencast() {
	ticker := time.NewTicker(66 * time.Millisecond) // 15fps
	defer ticker.Stop()
	for {
		select {
		case <-b.ctx.Done():
			return
		case <-ticker.C:
			b.mu.Lock()
			var buf []byte
			err := chromedp.Run(b.ctx, chromedp.CaptureScreenshot(&buf))
			b.mu.Unlock()
			if err != nil {
				continue
			}
			b.Broadcaster.Send(buf)
		}
	}
}

func (b *Browser) Navigate(url string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return chromedp.Run(b.ctx, chromedp.Navigate(url))
}

func (b *Browser) Back() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return chromedp.Run(b.ctx, chromedp.NavigateBack())
}

func (b *Browser) Forward() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return chromedp.Run(b.ctx, chromedp.NavigateForward())
}

func (b *Browser) Reload() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return chromedp.Run(b.ctx, chromedp.Reload())
}

func (b *Browser) Click(x, y float64, button string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	opt := chromedp.ButtonLeft
	if button == "right" { opt = chromedp.ButtonRight }
	if button == "middle" { opt = chromedp.ButtonMiddle }
	return chromedp.Run(b.ctx,
		chromedp.MouseClickXY(x, y, opt),
	)
}

func (b *Browser) MouseMove(x, y float64) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return chromedp.Run(b.ctx, input.DispatchMouseEvent(input.MouseMoved, x, y))
}

func (b *Browser) Scroll(x, y, deltaX, deltaY float64) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return chromedp.Run(b.ctx,
		input.DispatchMouseEvent(input.MouseWheel, x, y).WithDeltaX(deltaX).WithDeltaY(deltaY),
	)
}

func (b *Browser) KeyEvent(key string, keyCode int, eventType string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	t := input.KeyDown
	if eventType == "up" { t = input.KeyUp }
	return chromedp.Run(b.ctx,
		input.DispatchKeyEvent(t).WithKey(key).WithWindowsVirtualKeyCode(int64(keyCode)).WithNativeVirtualKeyCode(int64(keyCode)),
	)
}

func (b *Browser) TypeText(text string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return chromedp.Run(b.ctx, input.InsertText(text))
}

func (b *Browser) Screenshot() ([]byte, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	var buf []byte
	if err := chromedp.Run(b.ctx, chromedp.FullScreenshot(&buf, 90)); err != nil {
		return nil, err
	}
	return buf, nil
}

func (b *Browser) Evaluate(expr string) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	var res interface{}
	if err := chromedp.Run(b.ctx, chromedp.Evaluate(expr, &res)); err != nil {
		return "", err
	}
	return fmt.Sprintf("%v", res), nil
}

func (b *Browser) GetURL() (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	var url string
	if err := chromedp.Run(b.ctx, chromedp.Location(&url)); err != nil {
		return "", err
	}
	return url, nil
}

func (b *Browser) GetTitle() (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	var title string
	if err := chromedp.Run(b.ctx, chromedp.Title(&title)); err != nil {
		return "", err
	}
	return title, nil
}

func (b *Browser) GetPageInfo() (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	var res string
	js := `(function() {
		const els = document.querySelectorAll("a, button, input, select, textarea, [role=button], [onclick]");
		const items = [];
		els.forEach((el) => {
			const rect = el.getBoundingClientRect();
			if (rect.width === 0 && rect.height === 0) return;
			items.push({tag: el.tagName.toLowerCase(), text: (el.textContent||"").trim().slice(0,50), type: el.type||"", x: Math.round(rect.x+rect.width/2), y: Math.round(rect.y+rect.height/2)});
		});
		return JSON.stringify(items);
	})()`
	if err := chromedp.Run(b.ctx, chromedp.Evaluate(js, &res)); err != nil {
		return "", err
	}
	return res, nil
}

func (b *Browser) SetViewport(width, height int) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return chromedp.Run(b.ctx,
		chromedp.EmulateViewport(int64(width), int64(height)),
	)
}

func (b *Browser) EnableCookies() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return chromedp.Run(b.ctx, page.Enable())
}

// HandleInput processes a JSON input message from the browser UI
func (b *Browser) HandleInput(msg []byte) ([]byte, error) {
	var m map[string]interface{}
	if err := json.Unmarshal(msg, &m); err != nil {
		return nil, err
	}
	action, _ := m["action"].(string)
	switch action {
	case "navigate":
		url, _ := m["url"].(string)
		return nil, b.Navigate(url)
	case "back":
		return nil, b.Back()
	case "forward":
		return nil, b.Forward()
	case "reload":
		return nil, b.Reload()
	case "click":
		x, _ := m["x"].(float64)
		y, _ := m["y"].(float64)
		btn, _ := m["button"].(string)
		if btn == "" { btn = "left" }
		return nil, b.Click(x, y, btn)
	case "mousemove":
		x, _ := m["x"].(float64)
		y, _ := m["y"].(float64)
		return nil, b.MouseMove(x, y)
	case "scroll":
		x, _ := m["x"].(float64)
		y, _ := m["y"].(float64)
		dx, _ := m["deltaX"].(float64)
		dy, _ := m["deltaY"].(float64)
		return nil, b.Scroll(x, y, dx, dy)
	case "keydown", "keyup":
		key, _ := m["key"].(string)
		code, _ := m["keyCode"].(float64)
		evType := "down"
		if action == "keyup" { evType = "up" }
		return nil, b.KeyEvent(key, int(code), evType)
	case "type":
		text, _ := m["text"].(string)
		return nil, b.TypeText(text)
	case "geturl":
		url, err := b.GetURL()
		if err != nil { return nil, err }
		title, _ := b.GetTitle()
		resp, _ := json.Marshal(map[string]string{"url": url, "title": title})
		return resp, nil
	case "viewport":
		w, _ := m["width"].(float64)
		h, _ := m["height"].(float64)
		return nil, b.SetViewport(int(w), int(h))
	}
	return nil, fmt.Errorf("unknown action: %s", action)
}

func (b *Browser) Close() {
	b.cancel()
}
