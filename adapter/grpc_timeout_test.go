package adapter

import (
	"net"
	"testing"
	"time"

	"github.com/zhangpanda/gomcp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TestImportGRPC_TimeoutExpires verifies that ImportGRPC respects the Timeout
// option by connecting to a server that never responds to reflection.
func TestImportGRPC_TimeoutExpires(t *testing.T) {
	// Start a TCP listener that accepts connections but never responds.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	// Accept connections in background but do nothing (simulates a hung server).
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			// Hold the connection open but never respond.
			_ = conn
		}
	}()

	conn, err := grpc.NewClient(
		ln.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	s := gomcp.New("test-grpc-timeout", "1.0.0")

	start := time.Now()
	err = ImportGRPC(s, conn, GRPCOptions{
		Timeout: 500 * time.Millisecond,
	})
	elapsed := time.Since(start)

	// Should fail with a deadline/context error within reasonable time.
	if err == nil {
		t.Fatal("expected error from timeout, got nil")
	}
	t.Logf("ImportGRPC returned error after %v: %v", elapsed, err)

	// Verify it didn't take much longer than the timeout.
	if elapsed > 3*time.Second {
		t.Fatalf("took %v, expected to fail within ~500ms timeout", elapsed)
	}
}

// TestImportGRPC_TimeoutDefault verifies that zero Timeout uses default (10s)
// rather than blocking forever. We just verify the function returns an error
// within a reasonable time when connecting to a non-responding server.
func TestImportGRPC_TimeoutDefault(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping default timeout test in short mode (takes ~10s)")
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_ = conn
		}
	}()

	conn, err := grpc.NewClient(
		ln.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	s := gomcp.New("test-grpc-default-timeout", "1.0.0")

	start := time.Now()
	err = ImportGRPC(s, conn, GRPCOptions{
		// Timeout: 0 means default 10s
	})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error from default timeout, got nil")
	}
	t.Logf("ImportGRPC (default timeout) returned after %v: %v", elapsed, err)

	// Should return around 10s, definitely before 15s.
	if elapsed > 15*time.Second {
		t.Fatalf("default timeout took %v, expected ~10s", elapsed)
	}
}

// TestImportGRPC_TimeoutNegativeDisabled verifies negative timeout disables
// the deadline (we test with a very short connection timeout on the client
// side to avoid the test running forever).
func TestImportGRPC_TimeoutNegativeNoDeadline(t *testing.T) {
	// This just verifies the code path doesn't panic with negative timeout.
	// We use a closed listener so gRPC fails immediately with connection error.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close() // Close immediately so connection fails fast.

	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	s := gomcp.New("test-grpc-no-deadline", "1.0.0")

	err = ImportGRPC(s, conn, GRPCOptions{
		Timeout: -1, // disabled
	})

	// Should fail with connection error, not a deadline error.
	if err == nil {
		t.Fatal("expected connection error, got nil")
	}
	t.Logf("ImportGRPC (no deadline) returned: %v", err)
}
