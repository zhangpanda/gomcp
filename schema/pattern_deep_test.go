package schema

import (
	"reflect"
	"testing"
)

type nestZip struct {
	Addr struct {
		Zip string `json:"zip" mcp:"pattern=^[0-9]{5}$"`
	} `json:"addr"`
}

func TestInitPropertyPattern_NestedField(t *testing.T) {
	res := Generate(reflect.TypeOf(nestZip{}))
	zip := res.Properties["addr"].Properties["zip"]
	if zip.Pattern == "" || zip.PatternRe == nil {
		t.Fatalf("expected pattern + PatternRe, got pat=%q re=%v", zip.Pattern, zip.PatternRe)
	}
	if !zip.PatternRe.MatchString("90210") || zip.PatternRe.MatchString("x") {
		t.Fatalf("PatternRe match bug")
	}
}

type badRE struct {
	Field string `mcp:"pattern=("`
}

func TestInitPropertyPattern_InvalidPattern_NoPanic(t *testing.T) {
	res := Generate(reflect.TypeOf(badRE{}))
	p := res.Properties["field"]
	if p.PatternRe != nil {
		t.Fatalf("expected nil PatternRe for invalid pattern, got %v", p.PatternRe)
	}
	// manual Result: Validate still path uses MatchString with Pattern string
	err := Validate(map[string]any{"field": "abc"}, res)
	// invalid regex: MatchString compiles; err != nil is ignored, typically no match
	if err == nil {
		t.Fatalf("expected validation error for non-matching or invalid re behavior")
	}
}
