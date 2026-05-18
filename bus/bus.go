package bus

import "sync"

const EventProjectRemoved = "project.removed"

type Handler func(payload any)

type Bus struct {
	mu       sync.RWMutex
	handlers map[string][]Handler
}

func New() *Bus {
	return &Bus{handlers: make(map[string][]Handler)}
}

// Default is the process-wide event bus. Modules subscribe in LoadMe and emit
// during their operations — no direct inter-module client wiring required.
var Default = New()

func (b *Bus) On(event string, h Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[event] = append(b.handlers[event], h)
}

func (b *Bus) Emit(event string, payload any) {
	b.mu.RLock()
	hs := make([]Handler, len(b.handlers[event]))
	copy(hs, b.handlers[event])
	b.mu.RUnlock()
	for _, h := range hs {
		h(payload)
	}
}
