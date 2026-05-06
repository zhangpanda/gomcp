package gomcp

import (
	"sync"
	"time"

	"github.com/zhangpanda/gomcp/internal/uid"
)

// Session represents a client connection session.
type Session struct {
	ID         string
	CreatedAt  time.Time
	mu         sync.RWMutex
	store      map[string]any
	lastAccess time.Time
}

func newSession() *Session {
	now := time.Now()
	return &Session{
		ID:         uid.New(),
		CreatedAt:  now,
		lastAccess: now,
		store:      make(map[string]any),
	}
}

// touch records session activity for TTL eviction.
func (s *Session) touch() {
	s.mu.Lock()
	s.lastAccess = time.Now()
	s.mu.Unlock()
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
	sessions  sync.Map
	ttl       time.Duration
	done      chan struct{}
	closeOnce sync.Once
}

func newSessionManager() *SessionManager {
	sm := &SessionManager{
		ttl:  30 * time.Minute,
		done: make(chan struct{}),
	}
	go sm.evictLoop()
	return sm
}

func (sm *SessionManager) evictLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-sm.done:
			return
		case <-ticker.C:
			now := time.Now()
			sm.sessions.Range(func(key, value any) bool {
				s, ok := value.(*Session)
				if !ok {
					return true
				}
				s.mu.RLock()
				idle := now.Sub(s.lastAccess) > sm.ttl
				s.mu.RUnlock()
				if idle {
					sm.sessions.Delete(key)
				}
				return true
			})
		}
	}
}

func (sm *SessionManager) close() {
	sm.closeOnce.Do(func() { close(sm.done) })
}

// Get returns an existing session or creates a new one for the given ID.
func (sm *SessionManager) Get(id string) *Session {
	if v, ok := sm.sessions.Load(id); ok {
		if sess, ok := v.(*Session); ok {
			sess.touch()
			return sess
		}
	}
	s := newSession()
	s.ID = id
	actual, _ := sm.sessions.LoadOrStore(id, s)
	if sess, ok := actual.(*Session); ok {
		sess.touch()
		return sess
	}
	return s
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
