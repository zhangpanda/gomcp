// Deep concurrency and stress tests for public API behavior.
package gomcp_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/zhangpanda/gomcp"
)

// callToolText is like [callRaw] but safe to use from multiple goroutines (no *testing.T).
func callToolText(s *gomcp.Server, tool string, args map[string]any) (string, bool) {
	params, _ := json.Marshal(map[string]any{"name": tool, "arguments": args})
	req, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": json.RawMessage(params)})
	resp := s.HandleRaw(context.Background(), req)
	var msg struct {
		Result *struct {
			Content []struct{ Text string } `json:"content"`
			IsError bool                    `json:"isError"`
		} `json:"result"`
		Error *struct{ Message string } `json:"error"`
	}
	json.Unmarshal(resp, &msg)
	if msg.Error != nil {
		return msg.Error.Message, true
	}
	if msg.Result != nil && len(msg.Result.Content) > 0 {
		return msg.Result.Content[0].Text, msg.Result.IsError
	}
	return "", true
}

func TestDeep_ConcurrentCallsLatestVersion(t *testing.T) {
	s := gomcp.New("stress", "1.0")
	s.Tool("api", "a", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("v1"), nil
	}, gomcp.Version("1.0"))
	s.Tool("api", "a", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("v2"), nil
	}, gomcp.Version("2.0"))

	const workers = 256
	var wg sync.WaitGroup
	errCh := make(chan string, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			text, isErr := callToolText(s, "api", nil)
			if isErr {
				errCh <- "isErr: " + text
				return
			}
			if text != "v2" {
				errCh <- "want v2, got " + text
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for e := range errCh {
		t.Fatal(e)
	}
}

func TestDeep_ConcurrentExactVersion(t *testing.T) {
	s := gomcp.New("stress", "1.0")
	s.Tool("x", "a", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("a"), nil
	}, gomcp.Version("1.0"))
	s.Tool("x", "a", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("b"), nil
	}, gomcp.Version("2.0"))

	const workers = 128
	var wg sync.WaitGroup
	errCh := make(chan string, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			text, _ := callToolText(s, "x@1.0", nil)
			if text != "a" {
				errCh <- "x@1.0 want a, got " + text
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for e := range errCh {
		t.Fatal(e)
	}
}

func TestDeep_ConcurrentToolsList(t *testing.T) {
	s := gomcp.New("stress", "1.0")
	s.Tool("p", "d", func(ctx *gomcp.Context) (*gomcp.CallToolResult, error) {
		return ctx.Text("ok"), nil
	}, gomcp.Version("1.0"))
	const workers = 64
	var wg sync.WaitGroup
	errCh := make(chan string, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp := s.HandleRaw(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`))
			if resp == nil || len(resp) < 20 {
				errCh <- "bad tools/list: " + string(resp)
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for e := range errCh {
		t.Fatal(e)
	}
}
