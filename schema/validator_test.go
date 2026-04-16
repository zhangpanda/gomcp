package schema

import (
	"reflect"
	"strings"
	"testing"
)

func TestValidate_Required(t *testing.T) {
	s := Result{
		Required:   []string{"name"},
		Properties: map[string]Property{"name": {Type: "string"}},
	}
	err := Validate(map[string]any{}, s)
	if err == nil {
		t.Fatal("expected error for missing required field")
	}
	if !strings.Contains(err.Error(), "name: required") {
		t.Errorf("unexpected error: %s", err.Error())
	}

	err = Validate(map[string]any{"name": "alice"}, s)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_Minimum(t *testing.T) {
	min := 1.0
	s := Result{
		Properties: map[string]Property{"age": {Type: "integer", Minimum: &min}},
	}
	err := Validate(map[string]any{"age": 0.0}, s)
	if err == nil {
		t.Fatal("expected error for value below minimum")
	}
	if !strings.Contains(err.Error(), ">= 1") {
		t.Errorf("unexpected error: %s", err.Error())
	}

	err = Validate(map[string]any{"age": 5.0}, s)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_Maximum(t *testing.T) {
	max := 100.0
	s := Result{
		Properties: map[string]Property{"score": {Type: "integer", Maximum: &max}},
	}
	err := Validate(map[string]any{"score": 200.0}, s)
	if err == nil {
		t.Fatal("expected error for value above maximum")
	}
}

func TestValidate_Enum(t *testing.T) {
	s := Result{
		Properties: map[string]Property{"order": {Type: "string", Enum: []any{"asc", "desc"}}},
	}
	err := Validate(map[string]any{"order": "random"}, s)
	if err == nil {
		t.Fatal("expected error for invalid enum value")
	}
	if !strings.Contains(err.Error(), "must be one of") {
		t.Errorf("unexpected error: %s", err.Error())
	}

	err = Validate(map[string]any{"order": "asc"}, s)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_Pattern(t *testing.T) {
	s := Result{
		Properties: map[string]Property{"code": {Type: "string", Pattern: "^[A-Z]{3}$"}},
	}
	err := Validate(map[string]any{"code": "abc"}, s)
	if err == nil {
		t.Fatal("expected error for pattern mismatch")
	}

	err = Validate(map[string]any{"code": "ABC"}, s)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_MissingOptionalField(t *testing.T) {
	min := 0.0
	s := Result{
		Properties: map[string]Property{"age": {Type: "integer", Minimum: &min}},
	}
	// missing optional field should not error
	err := Validate(map[string]any{}, s)
	if err != nil {
		t.Errorf("unexpected error for missing optional field: %v", err)
	}
}

func TestValidate_MultipleErrors(t *testing.T) {
	min := 1.0
	max := 10.0
	s := Result{
		Required: []string{"name", "age"},
		Properties: map[string]Property{
			"name": {Type: "string"},
			"age":  {Type: "integer", Minimum: &min, Maximum: &max},
		},
	}
	err := Validate(map[string]any{"age": 20.0}, s)
	if err == nil {
		t.Fatal("expected error")
	}
	ve, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected *ValidationError, got %T", err)
	}
	if len(ve.Fields) != 2 {
		t.Errorf("expected 2 field errors, got %d: %s", len(ve.Fields), err.Error())
	}
}

func TestValidate_IntegrationWithGenerate(t *testing.T) {
	type Input struct {
		Query string `json:"query" mcp:"required"`
		Limit int    `json:"limit" mcp:"min=1,max=100"`
	}
	s := Generate(reflect.TypeOf(Input{}))

	// valid
	err := Validate(map[string]any{"query": "test", "limit": 10.0}, s)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// missing required + out of range
	err = Validate(map[string]any{"limit": 200.0}, s)
	if err == nil {
		t.Fatal("expected error")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "query: required") {
		t.Errorf("expected 'query: required' in error: %s", errStr)
	}
	if !strings.Contains(errStr, "must be <= 100") {
		t.Errorf("expected 'must be <= 100' in error: %s", errStr)
	}
}
