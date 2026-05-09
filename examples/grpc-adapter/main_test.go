package main

import (
	"strings"
	"testing"

	"github.com/zhangpanda/gomcp/mcptest"
)

// TestBuildServer_ImportsHealthCheck asserts that the gRPC adapter
// example really imports grpc.health.v1.Health/Check as an MCP tool
// and that a round-trip call reaches the dynamic protobuf request
// builder end-to-end. Uses mcptest so we can exercise the MCP server
// in-process without the stdio transport.
func TestBuildServer_ImportsHealthCheck(t *testing.T) {
	s, cleanup, err := buildServer()
	if err != nil {
		t.Fatalf("buildServer: %v", err)
	}
	defer cleanup()

	c := mcptest.NewClient(t, s)
	c.Initialize()

	// grpcToolName lowercases the trailing service name and snake-
	// cases the method. For grpc.health.v1.Health/Check that is
	// "health.check".
	tools := c.ListTools()
	if !containsTool(tools, "health.check") {
		t.Fatalf("expected health.check tool, got %v", tools)
	}

	// Probe the overall status.
	res := c.CallTool("health.check", map[string]any{"service": ""})
	res.AssertNoError(t)
	if !strings.Contains(res.Text(), "SERVING") {
		t.Errorf("empty-service probe: expected SERVING, got %q", res.Text())
	}

	// Named component must also serve, proving arguments round-trip
	// through dynamicpb.
	res = c.CallTool("health.check", map[string]any{"service": "example.svc"})
	res.AssertNoError(t)
	if !strings.Contains(res.Text(), "SERVING") {
		t.Errorf("example.svc probe: expected SERVING, got %q", res.Text())
	}
}

func containsTool(tools []string, want string) bool {
	for _, t := range tools {
		if t == want {
			return true
		}
	}
	return false
}
