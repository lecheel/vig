package wig

import (
	"slices"
	"sync"
)

type Position struct {
	Line int
	Char int
}

type Range struct {
	Start Position
	End   Position
}

type EventTextChange struct {
	Buf     *Buffer
	Start   Position
	End     Position
	NewEnd  Position
	Text    string
	OldText string
}

type EventBufferModeChange struct {
	Buf     *Buffer
	OldMode Mode
	NewMode Mode
}

type EventBufferReloaded struct {
	Buf *Buffer
}

type EventsManager struct {
	mu        sync.RWMutex
	listeners []chan Event
}

type EventKeyPressed struct {
	Key string
}

func NewEventsManager() *EventsManager {
	return &EventsManager{
		listeners: make([]chan Event, 0),
	}
}

func (e *EventsManager) Subscribe() <-chan Event {
	e.mu.Lock()
	defer e.mu.Unlock()
	c := make(chan Event)
	e.listeners = append(e.listeners, c)
	return c
}

func (e *EventsManager) Unsubscribe(ch <-chan Event) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.listeners = slices.DeleteFunc(e.listeners, func(delCh chan Event) bool {
		if delCh == ch {
			close(delCh)
			return true
		}
		return false
	})
}

type Event struct {
	Msg any
	Wg  *sync.WaitGroup
}

// this is very quick and dirty implementation.
// we need sync processing to not mess up lsp and treesitter.
// TODO: rewrite.
func (e *EventsManager) Broadcast(msg any) {
	e.mu.RLock()
	if len(e.listeners) == 0 {
		e.mu.RUnlock()
		return
	}
	listeners := make([]chan Event, len(e.listeners))
	copy(listeners, e.listeners)
	e.mu.RUnlock()

	wg := sync.WaitGroup{}
	for _, l := range listeners {
		if l == nil {
			continue
		}
		wg.Add(1)
		l <- Event{msg, &wg}
		wg.Wait()
	}
}
