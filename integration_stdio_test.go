package gomcp_test

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// Integration tests that start a real MCP server subprocess and communicate
// over stdio, exactly like Claude Desktop / Cursor would.
// Skip with: go test -short

func TestIntegration_Stdio_FullFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stdio integration test in short mode")
	}

	// Start filesystem server as subprocess
	cmd := exec.Command("go", "run", "./examples/filesystem")
	cmd.Dir = projectRoot(t)
	cmd.Env = append(os.Environ(), "FS_ROOT="+t.TempDir())
	cmd.Stderr = os.Stderr

	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)

	if err := cmd.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer func() {
		stdin.Close()
		cmd.Process.Kill()
		cmd.Wait()
	}()

	time.Sleep(500 * time.Millisecond)

	id := 0
	rpc := func(method string, params any) map[string]any {
		t.Helper()
		id++
		p, _ := json.Marshal(params)
		fmt.Fprintf(stdin, `{"jsonrpc":"2.0","id":%d,"method":"%s","params":%s}`+"\n", id, method, p)
		if !scanner.Scan() {
			t.Fatalf("no response for %s", method)
		}
		var resp map[string]any
		json.Unmarshal(scanner.Bytes(), &resp)
		return resp
	}

	getText := func(resp map[string]any) (string, bool) {
		if e, ok := resp["error"].(map[string]any); ok {
			return e["message"].(string), true
		}
		result, _ := resp["result"].(map[string]any)
		if result == nil {
			return "", true
		}
		isErr, _ := result["isError"].(bool)
		content, _ := result["content"].([]any)
		if len(content) > 0 {
			return content[0].(map[string]any)["text"].(string), isErr
		}
		return "", isErr
	}

	// 1. Initialize
	resp := rpc("initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "integration-test", "version": "1.0"},
	})
	result := resp["result"].(map[string]any)
	if result["protocolVersion"] != "2024-11-05" {
		t.Fatalf("bad protocol version: %v", result)
	}

	// notification (no response)
	id++
	fmt.Fprintf(stdin, `{"jsonrpc":"2.0","method":"notifications/initialized"}`+"\n")
	time.Sleep(50 * time.Millisecond)

	// 2. List tools
	resp = rpc("tools/list", map[string]any{})
	tools := resp["result"].(map[string]any)["tools"].([]any)
	if len(tools) != 5 {
		t.Fatalf("expected 5 tools, got %d", len(tools))
	}

	// 3. Write + read file
	text, isErr := getText(rpc("tools/call", map[string]any{
		"name": "write_file", "arguments": map[string]any{"path": "test.txt", "content": "hello"},
	}))
	if isErr {
		t.Fatalf("write_file error: %s", text)
	}

	text, isErr = getText(rpc("tools/call", map[string]any{
		"name": "read_file", "arguments": map[string]any{"path": "test.txt"},
	}))
	if isErr || text != "hello" {
		t.Fatalf("read_file: got %q, isErr=%v", text, isErr)
	}

	// 4. List dir
	text, _ = getText(rpc("tools/call", map[string]any{"name": "list_dir", "arguments": map[string]any{}}))
	if !strings.Contains(text, "test.txt") {
		t.Errorf("list_dir missing test.txt: %s", text)
	}

	// 5. Search
	text, _ = getText(rpc("tools/call", map[string]any{"name": "search_files", "arguments": map[string]any{"pattern": "*.txt"}}))
	if !strings.Contains(text, "test.txt") {
		t.Errorf("search missing test.txt: %s", text)
	}

	// 6. Nested dir
	getText(rpc("tools/call", map[string]any{"name": "write_file", "arguments": map[string]any{"path": "a/b.txt", "content": "nested"}}))
	text, _ = getText(rpc("tools/call", map[string]any{"name": "read_file", "arguments": map[string]any{"path": "a/b.txt"}}))
	if text != "nested" {
		t.Errorf("nested read: got %q", text)
	}

	// 7. Path traversal
	text, isErr = getText(rpc("tools/call", map[string]any{"name": "read_file", "arguments": map[string]any{"path": "../../etc/passwd"}}))
	if !isErr {
		t.Errorf("path traversal should fail: %s", text)
	}

	// 8. Missing required param
	_, isErr = getText(rpc("tools/call", map[string]any{"name": "read_file", "arguments": map[string]any{}}))
	if !isErr {
		t.Error("missing required should fail")
	}

	// 9. Nonexistent file
	_, isErr = getText(rpc("tools/call", map[string]any{"name": "read_file", "arguments": map[string]any{"path": "nope.txt"}}))
	if !isErr {
		t.Error("nonexistent file should fail")
	}

	// 10. Resource
	resp = rpc("resources/read", map[string]any{"uri": "fs://root"})
	contents := resp["result"].(map[string]any)["contents"].([]any)
	if len(contents) == 0 {
		t.Error("resource should return contents")
	}

	// 11. Prompt
	resp = rpc("prompts/get", map[string]any{"name": "file_review", "arguments": map[string]string{"path": "test.txt"}})
	msgs := resp["result"].(map[string]any)["messages"].([]any)
	if len(msgs) == 0 {
		t.Error("prompt should return messages")
	}

	// 12. Ping
	resp = rpc("ping", map[string]any{})
	if resp["result"] == nil {
		t.Error("ping should return result")
	}

	// 13. Tool not found
	resp = rpc("tools/call", map[string]any{"name": "nope"})
	if resp["error"] == nil {
		t.Error("nonexistent tool should return error")
	}
}

func projectRoot(t *testing.T) string {
	t.Helper()
	// find go.mod
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(dir + "/go.mod"); err == nil {
			return dir
		}
		parent := dir[:strings.LastIndex(dir, "/")]
		if parent == dir {
			t.Fatal("cannot find project root")
		}
		dir = parent
	}
}
