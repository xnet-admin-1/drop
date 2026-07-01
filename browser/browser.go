package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// findChromePath searches for Chrome/Chromium in common locations
func findChromePath() (string, error) {
	// 1. Check system PATH for known browser executables
	commands := []string{"google-chrome", "chromium-browser", "chromium", "chrome", "/snap/bin/chromium", "/usr/bin/google-chrome"}
	
	for _, cmd := range commands {
		path, err := exec.LookPath(cmd)
		if err == nil {
			log.Printf("Found browser in PATH: %s", path)
			return path, nil
		}
	}
	
	// 2. Check for local .chromium directory (project-relative)
	execPath, err := os.Executable()
	if err == nil {
		localChromium := filepath.Join(filepath.Dir(execPath), ".chromium", "chrome-linux", "chrome")
		if _, err := os.Stat(localChromium); err == nil {
			log.Printf("Found local browser: %s", localChromium)
			return localChromium, nil
		}
		
		// Also check project root
		localChromium = filepath.Join(filepath.Dir(execPath), ".chromium", "chrome-linux", "chrome")
		projectRoot := filepath.Dir(filepath.Dir(execPath))
		localChromium = filepath.Join(projectRoot, ".chromium", "chrome-linux", "chrome")
		if _, err := os.Stat(localChromium); err == nil {
			log.Printf("Found local browser: %s", localChromium)
			return localChromium, nil
		}
	}
	
	// 3. Check HOME/.local/bin
	homeDir := os.Getenv("HOME")
	if homeDir != "" {
		localPaths := []string{
			filepath.Join(homeDir, ".local", "bin", "chromium"),
			filepath.Join(homeDir, ".local", "lib", "chromium", "chrome-linux", "chrome"),
			filepath.Join(homeDir, ".chromium", "chrome-linux", "chrome"),
		}
		for _, p := range localPaths {
			if _, err := os.Stat(p); err == nil {
				log.Printf("Found local browser: %s", p)
				return p, nil
			}
		}
	}
	
	// 4. Check current working directory
	cwdChromium := filepath.Join(".", ".chromium", "chrome-linux", "chrome")
	if _, err := os.Stat(cwdChromium); err == nil {
		log.Printf("Found local browser: %s", cwdChromium)
		return cwdChromium, nil
	}
	
	return "", fmt.Errorf("no browser found. Please install Chromium/Chrome and ensure it's in your PATH, or run 'make setup-browser' to download automatically. See README.md for setup instructions")
}

// checkChrome checks if Chrome/Chromium is available
func checkChrome() error {
	_, err := findChromePath()
	return err
}

func LaunchBrowser() (*Browser, error) {
	// Find Chrome executable
	chromePath, err := findChromePath()
	if err != nil {
		return nil, fmt.Errorf("%w. See README.md for setup instructions", err)
	}
	
	log.Printf("Launching browser: %s", chromePath)
	
	// Detect if using local portable Chromium
	isLocal := !filepath.IsAbs(chromePath) || 
	           strings.Contains(chromePath, ".chromium") || 
	           strings.Contains(chromePath, "chrome-linux")
	
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("disable-infobars", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("window-size", "1280,720"),
		chromedp.UserAgent(UserAgent),
		chromedp.ExecPath(chromePath),
	)
	
	// For local Chromium, additional flags
	if isLocal {
		opts = append(opts,
			chromedp.Flag("disable-gpu", true),
		)
	}
	
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, cancel := chromedp.NewContext(allocCtx)
	if err := chromedp.Run(ctx,
		chromedp.Navigate("https://search.xnet.ngo"),
	); err != nil {
		cancel()
		allocCancel()
		return nil, fmt.Errorf("failed to launch browser: %w. Make sure Chrome/Chromium is properly installed", err)
	}
	b := &Browser{ctx: ctx, cancel: func() { cancel(); allocCancel() }, Broadcaster: NewBroadcaster()}
	log.Println("screencast goroutine starting")
	go b.screencast()
	return b, nil
}

func (b *Browser) screencast() {
	ticker := time.NewTicker(100 * time.Millisecond) // 10fps
	defer ticker.Stop()
	for {
		select {
		case <-b.ctx.Done():
			return
		case <-ticker.C:
			b.mu.Lock()
			var buf []byte
			err := chromedp.Run(b.ctx, chromedp.ActionFunc(func(ctx context.Context) error {
				var err error
				buf, err = page.CaptureScreenshot().WithFormat(page.CaptureScreenshotFormatJpeg).WithQuality(50).Do(ctx)
				return err
			}))
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
	log.Printf("CLICK x=%.1f y=%.1f button=%s", x, y, button)
	b.mu.Lock()
	defer b.mu.Unlock()
	// Use DOM-level click via JS - works on all elements and doesn't block on navigation
	js := fmt.Sprintf(`(function(x,y){
		var el=document.elementFromPoint(x,y);
		if(!el)return;
		var ev=new MouseEvent('click',{bubbles:true,cancelable:true,clientX:x,clientY:y});
		el.dispatchEvent(ev);
		if(el.tagName==='A'&&el.href)window.location=el.href;
	})(%f,%f)`, x, y)
	return chromedp.Run(b.ctx, chromedp.Evaluate(js, nil))
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
	// Map common keys to proper codes
	codes := map[string]int{"Enter":13,"Backspace":8,"Tab":9,"Escape":27,"ArrowLeft":37,"ArrowRight":39,"ArrowUp":38,"ArrowDown":40,"Delete":46}
	if c, ok := codes[key]; ok && keyCode == 0 { keyCode = c }
	cmd := input.DispatchKeyEvent(t).WithKey(key).WithWindowsVirtualKeyCode(int64(keyCode)).WithNativeVirtualKeyCode(int64(keyCode))
	if key == "Enter" && t == input.KeyDown {
		cmd = cmd.WithText("\r")
	}
	return chromedp.Run(b.ctx, cmd)
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
	log.Printf("HandleInput: %s", action)
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