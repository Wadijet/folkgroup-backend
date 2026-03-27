package decisionlive

import "sync"

type subscription struct {
	ch chan DecisionLiveEvent
}

type hub struct {
	mu   sync.Mutex
	subs map[string][]*subscription
}

func newHub() *hub {
	return &hub{subs: make(map[string][]*subscription)}
}

func (h *hub) subscribe(key string) (<-chan DecisionLiveEvent, func()) {
	// Buffer lớn: tránh bỏ frame khi client chậm hoặc replay WS chưa đọc kịp liveCh.
	ch := make(chan DecisionLiveEvent, 256)
	sub := &subscription{ch: ch}
	h.mu.Lock()
	h.subs[key] = append(h.subs[key], sub)
	h.mu.Unlock()
	cancel := func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		list := h.subs[key]
		for i, s := range list {
			if s == sub {
				h.subs[key] = append(list[:i], list[i+1:]...)
				break
			}
		}
		if len(h.subs[key]) == 0 {
			delete(h.subs, key)
		}
		close(ch)
	}
	return ch, cancel
}

func (h *hub) broadcast(key string, ev DecisionLiveEvent) {
	h.mu.Lock()
	list := h.subs[key]
	h.mu.Unlock()
	for _, sub := range list {
		select {
		case sub.ch <- ev:
		default:
			// Client chậm — bỏ frame, không block pipeline
		}
	}
}
