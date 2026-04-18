package gomcp

import (
	"sync"
	"time"

	"github.com/zhangpanda/gomcp/internal/uid"
)

// Session represents a client connection session.
type Session struct {
	ID        string
	CreatedAt time.Time
	mu        sync.RWMutex
	store     map[string]any
}

func newSession() *Session {
	return &Session{
		ID:        uid.New(),
		CreatedAt: time.Now(),
		store:     make(map[string]any),
	}
}

// Set stores a value in the session.
func (s *Session) Set(key string, val any) {
	s.mu.Lock()
	s.store[key] = val
	s.mu.Unlock()
}

// Get retrieves a value from the session.
func (s *Session) Get(key string) (any, bool) {
	s.mu.RLock()
	v, ok := s.store[key]
	s.mu.RUnlock()
	return v, ok
}

// SessionManager tracks active sessions.
type SessionManager struct {
	sessions sync.Map
}

func newSessionManager() *SessionManager {
	return &SessionManager{}
}

// Get returns an existing session or creates a new one for the given ID.
func (sm *SessionManager) Get(id string) *Session {
	if v, ok := sm.sessions.Load(id); ok {
		return v.(*Session)
	}
	s := newSession()
	s.ID = id
	actual, _ := sm.sessions.LoadOrStore(id, s)
	return actual.(*Session)
}

// GetOrCreate returns an existing session or creates one with a new ID.
func (sm *SessionManager) GetOrCreate(id string) *Session {
	if id != "" {
		return sm.Get(id)
	}
	s := newSession()
	sm.sessions.Store(s.ID, s)
	return s
}

// Remove deletes a session.
func (sm *SessionManager) Remove(id string) {
	sm.sessions.Delete(id)
}

// Count returns the number of active sessions.
func (sm *SessionManager) Count() int {
	n := 0
	sm.sessions.Range(func(_, _ any) bool { n++; return true })
	return n
}
