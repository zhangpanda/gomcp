package schema

import (
	"reflect"
	"testing"
)

type Simple struct {
	Name   string  `json:"name" mcp:"required,desc=User name"`
	Age    int     `json:"age" mcp:"min=0,max=150"`
	Score  float64 `json:"score" mcp:"default=0.0"`
	Active bool    `json:"active"`
}

type WithEnum struct {
	Order string `json:"order" mcp:"required,enum=asc|desc"`
}

type WithPattern struct {
	Code string `json:"code" mcp:"pattern=^[A-Z]{3}$"`
}

type Nested struct {
	Label   string `json:"label" mcp:"required"`
	Address struct {
		City string `json:"city" mcp:"required,desc=City"`
		Zip  string `json:"zip"`
	} `json:"address"`
}

type WithSlice struct {
	Tags []string `json:"tags" mcp:"desc=Tag list"`
	Nums []int    `json:"nums"`
}

type WithPointer struct {
	Name *string `json:"name" mcp:"required"`
}

type unexportedField struct {
	Public  string `json:"public"`
	private string //nolint
}

type SkipField struct {
	Keep string `json:"keep"`
	Skip string `json:"-"`
}

func TestGenerate_Simple(t *testing.T) {
	res := Generate(reflect.TypeOf(Simple{}))

	if len(res.Properties) != 4 {
		t.Fatalf("expected 4 properties, got %d", len(res.Properties))
	}
	if len(res.Required) != 1 || res.Required[0] != "name" {
		t.Errorf("expected required=[name], got %v", res.Required)
	}

	name := res.Properties["name"]
	if name.Type != "string" {
		t.Errorf("name type: want string, got %s", name.Type)
	}
	if name.Description != "User name" {
		t.Errorf("name desc: want 'User name', got %q", name.Description)
	}

	age := res.Properties["age"]
	if age.Type != "integer" {
		t.Errorf("age type: want integer, got %s", age.Type)
	}
	if age.Minimum == nil || *age.Minimum != 0 {
		t.Errorf("age min: want 0, got %v", age.Minimum)
	}
	if age.Maximum == nil || *age.Maximum != 150 {
		t.Errorf("age max: want 150, got %v", age.Maximum)
	}

	score := res.Properties["score"]
	if score.Type != "number" {
		t.Errorf("score type: want number, got %s", score.Type)
	}
	if score.Default != 0.0 {
		t.Errorf("score default: want 0.0, got %v", score.Default)
	}

	active := res.Properties["active"]
	if active.Type != "boolean" {
		t.Errorf("active type: want boolean, got %s", active.Type)
	}
}

func TestGenerate_Enum(t *testing.T) {
	res := Generate(reflect.TypeOf(WithEnum{}))
	order := res.Properties["order"]
	if len(order.Enum) != 2 {
		t.Fatalf("expected 2 enum values, got %d", len(order.Enum))
	}
	if order.Enum[0] != "asc" || order.Enum[1] != "desc" {
		t.Errorf("unexpected enum: %v", order.Enum)
	}
}

func TestGenerate_Pattern(t *testing.T) {
	res := Generate(reflect.TypeOf(WithPattern{}))
	code := res.Properties["code"]
	if code.Pattern != "^[A-Z]{3}$" {
		t.Errorf("unexpected pattern: %s", code.Pattern)
	}
	if code.PatternRe == nil || !code.PatternRe.MatchString("ABC") || code.PatternRe.MatchString("abc") {
		t.Errorf("expected precompiled PatternRe for valid/invalid strings, got PatternRe=%v", code.PatternRe)
	}
}

func TestGenerate_Nested(t *testing.T) {
	res := Generate(reflect.TypeOf(Nested{}))
	addr := res.Properties["address"]
	if addr.Type != "object" {
		t.Fatalf("address type: want object, got %s", addr.Type)
	}
	if len(addr.Properties) != 2 {
		t.Fatalf("address props: want 2, got %d", len(addr.Properties))
	}
	if len(addr.Required) != 1 || addr.Required[0] != "city" {
		t.Errorf("address required: want [city], got %v", addr.Required)
	}
	if addr.Properties["city"].Description != "City" {
		t.Errorf("city desc: want 'City', got %q", addr.Properties["city"].Description)
	}
}

func TestGenerate_Slice(t *testing.T) {
	res := Generate(reflect.TypeOf(WithSlice{}))
	tags := res.Properties["tags"]
	if tags.Type != "array" {
		t.Fatalf("tags type: want array, got %s", tags.Type)
	}
	if tags.Items == nil || tags.Items.Type != "string" {
		t.Errorf("tags items: want string, got %v", tags.Items)
	}
	if tags.Description != "Tag list" {
		t.Errorf("tags desc: want 'Tag list', got %q", tags.Description)
	}

	nums := res.Properties["nums"]
	if nums.Items == nil || nums.Items.Type != "integer" {
		t.Errorf("nums items: want integer, got %v", nums.Items)
	}
}

func TestGenerate_Pointer(t *testing.T) {
	res := Generate(reflect.TypeOf(WithPointer{}))
	if res.Properties["name"].Type != "string" {
		t.Errorf("pointer field type: want string, got %s", res.Properties["name"].Type)
	}
	if len(res.Required) != 1 {
		t.Errorf("expected 1 required, got %d", len(res.Required))
	}
}

func TestGenerate_UnexportedFieldsSkipped(t *testing.T) {
	res := Generate(reflect.TypeOf(unexportedField{}))
	if len(res.Properties) != 1 {
		t.Errorf("expected 1 property (only public), got %d", len(res.Properties))
	}
}

func TestGenerate_SkipDashTag(t *testing.T) {
	res := Generate(reflect.TypeOf(SkipField{}))
	if _, ok := res.Properties["-"]; ok {
		t.Error("field with json:\"-\" should be skipped")
	}
	if len(res.Properties) != 1 {
		t.Errorf("expected 1 property, got %d", len(res.Properties))
	}
}

func TestGenerate_NonStruct(t *testing.T) {
	res := Generate(reflect.TypeOf("hello"))
	if len(res.Properties) != 0 {
		t.Errorf("non-struct should produce empty properties, got %d", len(res.Properties))
	}
}

func TestGenerate_PointerToStruct(t *testing.T) {
	res := Generate(reflect.TypeOf((*Simple)(nil)))
	if len(res.Properties) != 4 {
		t.Errorf("pointer to struct should work, got %d properties", len(res.Properties))
	}
}
