package schema_test

import (
	"reflect"
	"sync"
	"testing"

	"github.com/zhangpanda/gomcp/schema"
)

// BUG S1: concurrent Generate on the same struct type previously stored
// a sentinel (empty Result) in the cache before populating the fields,
// so a racing second goroutine could Load the sentinel and return an
// empty schema.
func TestRegression_GenerateConcurrentSameType(t *testing.T) {
	type Payload struct {
		Name  string `json:"name" mcp:"required"`
		Count int    `json:"count" mcp:"min=0,max=100"`
		Tag   string `json:"tag" mcp:"enum=a|b|c"`
	}

	var wg sync.WaitGroup
	const N = 50
	results := make([]schema.Result, N)
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx] = schema.Generate(reflect.TypeOf(Payload{}))
		}(i)
	}
	wg.Wait()

	for i, res := range results {
		if len(res.Properties) == 0 {
			t.Fatalf("goroutine %d got empty Properties — generator race regressed", i)
		}
		if _, ok := res.Properties["name"]; !ok {
			t.Fatalf("goroutine %d missing 'name' property", i)
		}
	}
}

// BUG S2: validator did not recurse into "type: object" fields, so
// nested required keys and nested constraints were silently ignored.
func TestRegression_ValidateNestedObject(t *testing.T) {
	type Inner struct {
		Age int `json:"age" mcp:"required,min=0,max=150"`
	}
	type Outer struct {
		User Inner `json:"user" mcp:"required"`
	}
	res := schema.Generate(reflect.TypeOf(Outer{}))

	// Missing nested required key must now fail.
	err := schema.Validate(map[string]any{"user": map[string]any{}}, res)
	if err == nil {
		t.Fatal("expected validation error for missing user.age, got nil")
	}
	if got := err.Error(); !contains(got, "user.age") {
		t.Fatalf("expected error on 'user.age', got %q", got)
	}

	// Out-of-range nested value must now fail.
	err = schema.Validate(map[string]any{"user": map[string]any{"age": 999.0}}, res)
	if err == nil {
		t.Fatal("expected validation error for user.age > 150")
	}
	if got := err.Error(); !contains(got, "user.age") {
		t.Fatalf("expected error on 'user.age', got %q", got)
	}
}

// BUG S3: validator did not descend into array items, so per-item
// constraints (pattern, min/max, enum) were silently bypassed.
func TestRegression_ValidateArrayItems(t *testing.T) {
	type Req struct {
		Ports []int `json:"ports" mcp:"required"`
	}
	res := schema.Generate(reflect.TypeOf(Req{}))
	// Manually attach a min constraint on the array item.
	min := 1024.0
	item := *res.Properties["ports"].Items
	item.Minimum = &min
	p := res.Properties["ports"]
	p.Items = &item
	res.Properties["ports"] = p

	// A value below the min should now fail, pointing at the index.
	err := schema.Validate(map[string]any{"ports": []any{80.0}}, res)
	if err == nil {
		t.Fatal("expected validation error for ports[0] < 1024")
	}
	if got := err.Error(); !contains(got, "ports[0]") {
		t.Fatalf("expected error on 'ports[0]', got %q", got)
	}

	// All-valid items should pass.
	if err := schema.Validate(map[string]any{"ports": []any{8080.0, 9000.0}}, res); err != nil {
		t.Fatalf("valid array should pass, got %v", err)
	}
}

// BUG S5: map fields used to be mis-declared as "string" in the
// generated schema, which meant tools accepting free-form maps of
// properties were unusable from strict MCP clients.
func TestRegression_MapFieldSchemaIsObject(t *testing.T) {
	type Req struct {
		Meta map[string]string `json:"meta"`
	}
	res := schema.Generate(reflect.TypeOf(Req{}))
	if got := res.Properties["meta"].Type; got != "object" {
		t.Fatalf("map field should be schema type 'object', got %q", got)
	}
}

// Self-referential struct should not infinite-loop and should produce a
// finite schema — this was the v1.3.0 fix, but the Batch-A generator
// rewrite re-implemented cycle detection via a per-call visited set.
// Keep a check here to make sure we did not regress it.
func TestRegression_GenerateSelfReferential(t *testing.T) {
	type Node struct {
		Value int   `json:"value"`
		Next  *Node `json:"next"`
	}
	res := schema.Generate(reflect.TypeOf(Node{}))
	if _, ok := res.Properties["value"]; !ok {
		t.Fatal("expected 'value' property on self-ref struct")
	}
	if _, ok := res.Properties["next"]; !ok {
		t.Fatal("expected 'next' property on self-ref struct")
	}
	// nested 'next' should be an object; its inner schema is allowed to
	// be empty — we just must not recurse indefinitely.
	if got := res.Properties["next"].Type; got != "object" {
		t.Fatalf("'next' should be object, got %q", got)
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
