package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/signal"
	"syscall"
)

// MessageHandler processes a raw JSON-RPC message and returns a response (or nil for notifications).
type MessageHandler func(ctx context.Context, msg json.RawMessage) json.RawMessage

// ServeStdio runs the MCP server over stdin/stdout with graceful shutdown.
func ServeStdio(handler MessageHandler) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	return serveIO(ctx, os.Stdin, os.Stdout, handler)
}

func serveIO(ctx context.Context, r io.Reader, w io.Writer, handler MessageHandler) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024) // 10MB max

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		resp := handler(ctx, line)
		if resp == nil {
			continue
		}

		resp = append(resp, '\n')
		if _, err := w.Write(resp); err != nil {
			return err
		}
	}
	return scanner.Err()
}
