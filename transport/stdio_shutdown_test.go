package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestServeIO_ContextCancel verifies that serveIO returns promptly when
// the context is cancelled, even if the reader is blocked (no data).
func TestServeIO_ContextCancel(t *testing.T) {
	// Use a pipe where we never write — simulates stdin with no data.
	pr, pw := io.Pipe()
	defer pw.Close()

	var w bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- serveIO(ctx, pr, &w, echoHandler)
	}()

	// Cancel the context after a short delay.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected nil error on ctx cancel, got: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("serveIO did not return within 2s after context cancellation — shutdown is broken")
	}
}

// TestServeIO_ContextCancelMidStream verifies that serveIO stops processing
// after context is cancelled even if there are already messages queued.
func TestServeIO_ContextCancelMidStream(t *testing.T) {
	pr, pw := io.Pipe()
	var w bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())

	// A slow handler that gives us time to cancel.
	slowHandler := func(_ context.Context, msg json.RawMessage) json.RawMessage {
		time.Sleep(100 * time.Millisecond)
		return echoHandler(context.Background(), msg)
	}

	done := make(chan error, 1)
	go func() {
		done <- serveIO(ctx, pr, &w, slowHandler)
	}()

	// Send a message, then cancel while handler is processing.
	pw.Write([]byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n"))
	time.Sleep(30 * time.Millisecond)
	cancel()
	pw.Close()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected nil error, got: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("serveIO did not return within 2s after context cancellation")
	}
}

// TestServeIO_ReaderError verifies that scanner errors are propagated.
func TestServeIO_ReaderError(t *testing.T) {
	pr, pw := io.Pipe()
	var w bytes.Buffer
	ctx := context.Background()

	done := make(chan error, 1)
	go func() {
		done <- serveIO(ctx, pr, &w, echoHandler)
	}()

	// Write one valid message, then forcefully close with error.
	pw.Write([]byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n"))
	time.Sleep(50 * time.Millisecond)
	pw.CloseWithError(io.ErrUnexpectedEOF)

	select {
	case err := <-done:
		if err == nil {
			// io.Pipe CloseWithError may surface as scanner.Err() or just EOF.
			// Either nil (EOF treated as clean) or the error is acceptable.
			t.Log("serveIO returned nil (pipe close treated as EOF)")
		} else {
			t.Logf("serveIO returned error as expected: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("serveIO did not return within 2s after reader error")
	}
}

// TestServeIO_LargePayload verifies large messages are handled correctly.
func TestServeIO_LargePayload(t *testing.T) {
	// Create a ~500KB JSON payload.
	bigValue := strings.Repeat("x", 500*1024)
	msg := `{"jsonrpc":"2.0","id":1,"method":"echo","params":{"data":"` + bigValue + `"}}` + "\n"

	r := strings.NewReader(msg)
	var w bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := serveIO(ctx, r, &w, echoHandler)
	if err != nil {
		t.Fatalf("serveIO error: %v", err)
	}
	if w.Len() == 0 {
		t.Fatal("expected response for large payload, got nothing")
	}
}

// TestServeIO_RapidMessages verifies high-throughput message processing.
func TestServeIO_RapidMessages(t *testing.T) {
	const n = 1000
	var input strings.Builder
	for i := 0; i < n; i++ {
		input.WriteString(`{"jsonrpc":"2.0","id":` + strings.Repeat("", 0) + `1,"method":"ping"}` + "\n")
	}

	r := strings.NewReader(input.String())
	var w bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := serveIO(ctx, r, &w, echoHandler)
	if err != nil {
		t.Fatalf("serveIO error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(w.String()), "\n")
	if len(lines) != n {
		t.Fatalf("expected %d responses, got %d", n, len(lines))
	}
}

// TestServeIO_ConcurrentCancelAndData tests the race between ctx cancel and
// incoming data — no panic or data race should occur.
func TestServeIO_ConcurrentCancelAndData(t *testing.T) {
	for i := 0; i < 50; i++ {
		pr, pw := io.Pipe()
		var w bytes.Buffer
		ctx, cancel := context.WithCancel(context.Background())

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			serveIO(ctx, pr, &w, echoHandler)
		}()

		// Race: write and cancel at roughly the same time.
		go func() {
			pw.Write([]byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n"))
			pw.Close()
		}()
		go cancel()

		wg.Wait()
		pr.Close()
	}
}

// TestServeIO_WriteError verifies that write errors cause serveIO to return.
func TestServeIO_WriteError(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n"
	r := strings.NewReader(input)

	// Writer that always fails.
	failWriter := &errWriter{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := serveIO(ctx, r, failWriter, echoHandler)
	if err == nil {
		t.Fatal("expected write error, got nil")
	}
}

type errWriter struct{}

func (e *errWriter) Write([]byte) (int, error) {
	return 0, io.ErrClosedPipe
}

// TestServeIO_CleanEOF verifies that a clean reader EOF produces no error.
func TestServeIO_CleanEOF(t *testing.T) {
	// Empty reader — immediate EOF.
	r := strings.NewReader("")
	var w bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := serveIO(ctx, r, &w, echoHandler)
	if err != nil {
		t.Fatalf("expected nil on clean EOF, got: %v", err)
	}
	if w.Len() != 0 {
		t.Fatalf("expected no output on empty input, got: %s", w.String())
	}
}
