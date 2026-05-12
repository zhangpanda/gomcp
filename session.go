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
	// evicted is set by the SessionManager under s.mu (write) before
	// removing the entry from the map; touch() checks it so a session
	// that has just been evicted cannot silently come back to life.
	evicted bool
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

// touch records session activity for TTL eviction. Returns false if the
// session has been evicted — callers should treat that as "session
// missing" and create a fresh one.
func (s *Session) touch() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.evicted {
		return false
	}
	s.lastAccess = time.Now()
	return true
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
			sm.evictIdle(time.Now())
		}
	}
}

// evictIdle walks the session map and removes entries that have been
// idle for longer than the TTL. The check and removal are held under
// the session's own write lock to eliminate the classic race:
//
//  1. evictLoop reads lastAccess under RLock, finds it stale
//  2. evictLoop releases RLock
//  3. user touch() under Lock updates lastAccess to now
//  4. evictLoop deletes the entry anyway, silently losing the session
//
// By holding Lock through CompareAndDelete, touch() is serialised: if
// touch wins, we see the new lastAccess and skip delete; if we win,
// touch returns false and the caller creates a fresh session.
//
// Exported as a method (lowercase) so regression tests can exercise it
// deterministically instead of waiting for the ticker.
func (sm *SessionManager) evictIdle(now time.Time) {
	sm.sessions.Range(func(key, value any) bool {
		s, ok := value.(*Session)
		if !ok {
			return true
		}
		s.mu.Lock()
		idle := now.Sub(s.lastAccess) > sm.ttl
		if idle {
			s.evicted = true
		}
		s.mu.Unlock()
		if idle {
			// CompareAndDelete avoids wiping out a replacement session
			// that a racing Get may have just stored under the same id.
			sm.sessions.CompareAndDelete(key, s)
		}
		return true
	})
}

func (sm *SessionManager) close() {
	sm.closeOnce.Do(func() { close(sm.done) })
}

// Get returns an existing session or creates a new one for the given ID.
func (sm *SessionManager) Get(id string) *Session {
	if v, ok := sm.sessions.Load(id); ok {
		if sess, ok := v.(*Session); ok && sess.touch() {
			return sess
		}
		// evicted between Load and touch — fall through and re-create.
	}
	s := newSession()
	s.ID = id
	actual, loaded := sm.sessions.LoadOrStore(id, s)
	if loaded {
		if sess, ok := actual.(*Session); ok && sess.touch() {
			return sess
		}
		// The entry we found was evicted too. Force-replace it.
		sm.sessions.Store(id, s)
	}
	return s
}

// GetOrCreate returns an existing session or creates one with a new ID.
// When id is empty (e.g. stdio transport with no Mcp-Session-Id header),
// a single shared "default" session is returned so that repeated calls
// without a session header don't leak unbounded session objects.
func (sm *SessionManager) GetOrCreate(id string) *Session {
	if id != "" {
		return sm.Get(id)
	}
	return sm.Get("_default")
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
