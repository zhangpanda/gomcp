package adapter_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zhangpanda/gomcp"
	"github.com/zhangpanda/gomcp/adapter"
	"github.com/zhangpanda/gomcp/mcptest"
)

// BUG O2: OpenAPI body construction used ctx.String to read every
// request-body property, so []any came out as Go's "[a b]" string
// formatting and nested maps became "map[a:1]". Complex types were
// effectively unsendable. Fixed by reading from ctx.Args() so raw
// decoded values pass through.
func TestRegression_OpenAPIBodyComplexTypes(t *testing.T) {
	var received struct {
		Tags     []string       `json:"tags"`
		Metadata map[string]any `json:"metadata"`
		Count    int            `json:"count"`
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer upstream.Close()

	spec := `openapi: 3.0.0
paths:
  /widgets:
    post:
      operationId: createWidget
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                tags:
                  type: array
                  items:
                    type: string
                metadata:
                  type: object
                count:
                  type: integer
`
	specPath := writeTempYAML(t, spec)

	s := gomcp.New("t", "1.0")
	if err := adapter.ImportOpenAPI(s, specPath, adapter.OpenAPIOptions{ServerURL: upstream.URL}); err != nil {
		t.Fatal(err)
	}
	client := mcptest.NewClient(t, s)

	res := client.CallTool("createWidget", map[string]any{
		"tags":     []any{"alpha", "beta"},
		"metadata": map[string]any{"env": "prod", "region": "us-west"},
		"count":    42,
	})
	if res.IsError() {
		t.Fatalf("call failed: %s", res.Text())
	}

	if len(received.Tags) != 2 || received.Tags[0] != "alpha" || received.Tags[1] != "beta" {
		t.Fatalf("expected tags=[alpha beta], got %v", received.Tags)
	}
	if received.Metadata["env"] != "prod" || received.Metadata["region"] != "us-west" {
		t.Fatalf("expected metadata to round-trip, got %v", received.Metadata)
	}
	if received.Count != 42 {
		t.Fatalf("expected count=42, got %d", received.Count)
	}
}

// BUG O5: array schemas were emitted without 'items', breaking strict
// clients. openAPIToJSONSchema now propagates items recursively.
func TestRegression_OpenAPIArraySchemaHasItems(t *testing.T) {
	spec := `openapi: 3.0.0
paths:
  /search:
    post:
      operationId: search
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                keywords:
                  type: array
                  items:
                    type: string
`
	specPath := writeTempYAML(t, spec)

	s := gomcp.New("t", "1.0")
	if err := adapter.ImportOpenAPI(s, specPath, adapter.OpenAPIOptions{ServerURL: "http://example.invalid"}); err != nil {
		t.Fatal(err)
	}

	// Dig into the generated schema via tools/list raw JSON-RPC.
	req, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/list"})
	raw := s.HandleRaw(context.Background(), req)
	var env struct {
		Result struct {
			Tools []struct {
				Name        string         `json:"name"`
				InputSchema map[string]any `json:"inputSchema"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}
	var kw map[string]any
	for _, tool := range env.Result.Tools {
		if tool.Name != "search" {
			continue
		}
		props, _ := tool.InputSchema["properties"].(map[string]any)
		kw, _ = props["keywords"].(map[string]any)
	}
	if kw == nil {
		t.Fatal("could not locate 'keywords' property in tool schema")
	}
	if kw["type"] != "array" {
		t.Fatalf("keywords should be array, got %v", kw["type"])
	}
	items, ok := kw["items"].(map[string]any)
	if !ok {
		t.Fatalf("keywords.items missing or not an object: %v", kw["items"])
	}
	if items["type"] != "string" {
		t.Fatalf("keywords.items.type should be 'string', got %v", items["type"])
	}
}
