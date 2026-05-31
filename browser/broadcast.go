package browser

import "sync"

type FrameBroadcaster struct {
	mu      sync.RWMutex
	clients map[chan []byte]struct{}
}

func NewBroadcaster() *FrameBroadcaster {
	return &FrameBroadcaster{clients: make(map[chan []byte]struct{})}
}

func (fb *FrameBroadcaster) Subscribe() chan []byte {
	ch := make(chan []byte, 2)
	fb.mu.Lock()
	fb.clients[ch] = struct{}{}
	fb.mu.Unlock()
	return ch
}

func (fb *FrameBroadcaster) Unsubscribe(ch chan []byte) {
	fb.mu.Lock()
	delete(fb.clients, ch)
	fb.mu.Unlock()
	close(ch)
}

func (fb *FrameBroadcaster) Send(frame []byte) {
	fb.mu.RLock()
	defer fb.mu.RUnlock()
	for ch := range fb.clients {
		select {
		case ch <- frame:
		default: // drop if slow
		}
	}
}
