package gomcp

import (
	"testing"
	"time"
)

// BUG M1: the RateLimit token bucket used to subtract tokens whenever
// now.Sub(lastTime) returned a negative duration — which happens after
// an NTP step back or a VM suspend/resume. Before this fix the bucket
// would then deny all requests until the (potentially very small)
// negative delta fully refilled. This test verifies the guard inside
// (*tokenBucket).take().
func TestRegression_RateLimitClockRegression(t *testing.T) {
	b := &tokenBucket{
		tokens:   10,
		max:      60,
		rate:     1.0, // 1 token / second
		lastTime: time.Now(),
	}

	// First take burns one token normally.
	if !b.take() {
		t.Fatal("initial take should succeed with 10 tokens in bucket")
	}

	// Simulate a clock step back by an hour. With the old code the next
	// take would compute a hugely negative delta, zero the bucket, and
	// deny. With the fix the negative delta is ignored and we still
	// have tokens left.
	b.lastTime = time.Now().Add(1 * time.Hour)
	if !b.take() {
		t.Fatal("token bucket refused a caller after clock went backward — regression")
	}
	if b.tokens < 0 {
		t.Fatalf("bucket should not go negative after clock regression, got %f", b.tokens)
	}
}
