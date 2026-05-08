package gomcp

import "time"

// This file exposes internal helpers to the gomcp_test package. Its
// *_test.go suffix means it is compiled only for tests, so these
// helpers never ship as part of the public API.

// EvictIdleForTest triggers the session-idle sweep immediately using
// the given "now" timestamp. Tests use it to avoid depending on the
// one-minute eviction ticker.
func (sm *SessionManager) EvictIdleForTest(now time.Time) {
	sm.evictIdle(now)
}

// SetNotifyFnForTest appends a notify callback — normally this slot is
// populated by [Server.HTTP] / [Server.Handler] when the HTTP transport
// attaches, but tests need an independent observer.
func (s *Server) SetNotifyFnForTest(fn func(method string, params any)) {
	s.mu.Lock()
	s.notifyFn = append(s.notifyFn, fn)
	s.mu.Unlock()
}
