package gomcp_test

import (
	"sync"
	"testing"
	"time"

	"github.com/zhangpanda/gomcp"
)

// TestRegression_CloseDuringEvictionNoDeadlock hammers Server.Close()
// in parallel with tight eviction sweeps. Previous Close() revisions
// serialized shutdown behind s.mu and could block an in-flight
// evictIdle that held session locks; a slow sweep could then wedge
// Close forever.
//
// The current shape — closeOnce + sessions.close() closing a
// dedicated done channel — should be race-safe. This test locks
// that behaviour in: after Close returns, further sweeps must be
// no-ops and must not deadlock, panic, or resurrect the server.
func TestRegression_CloseDuringEvictionNoDeadlock(t *testing.T) {
	const sessions = 200
	const sweepers = 4
	const sweepLoops = 500

	s := gomcp.New("t", "1.0")
	sm := s.Sessions()

	// Seed a pile of sessions so the eviction sweep actually has work
	// to iterate through rather than returning on an empty map.
	for i := 0; i < sessions; i++ {
		sm.Get("sess-" + itoa(i))
	}

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Sweepers: call evictIdle with a past-TTL "now" on one third of
	// the iterations and a present "now" on the other two thirds,
	// mimicking the real ticker that runs every minute against a mix
	// of idle and active sessions.
	for w := 0; w < sweepers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; ; i++ {
				select {
				case <-stop:
					return
				default:
				}
				if i%3 == 0 {
					sm.EvictIdleForTest(time.Now().Add(time.Hour))
				} else {
					sm.EvictIdleForTest(time.Now())
				}
				if i >= sweepLoops {
					return
				}
			}
		}()
	}

	// A toucher goroutine keeps new sessions flowing so Close meets
	// an actually-loaded manager, not a freshly-drained one.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			sm.Get("live-" + itoa(i%17))
			if i >= sweepLoops {
				return
			}
		}
	}()

	// Give the background goroutines ~5ms to make some noise, then
	// call Close from the main goroutine with a hard deadline.
	time.Sleep(5 * time.Millisecond)

	done := make(chan struct{})
	go func() {
		s.Close()
		s.Close() // idempotency check — must not hang on the second call
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatalf("Close() did not return within 3s — deadlock against eviction sweep")
	}

	close(stop)
	wg.Wait()

	// A post-Close sweep must still be harmless. Previously the
	// sessions field would have been nil'd out, which we don't want
	// to regress into: existing holders of *SessionManager would
	// crash.
	sm.EvictIdleForTest(time.Now())
}

// itoa is a tiny stdlib-free integer formatter so this test file can
// avoid pulling in strconv just for the session-seeding loop.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
