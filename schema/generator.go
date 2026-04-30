package schema

import (
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// Property represents a JSON Schema property.
type Property struct {
	Type        string              `json:"type"`
	Description string              `json:"description,omitempty"`
	Default     any                 `json:"default,omitempty"`
	Enum        []any               `json:"enum,omitempty"`
	Minimum     *float64            `json:"minimum,omitempty"`
	Maximum     *float64            `json:"maximum,omitempty"`
	Pattern     string              `json:"pattern,omitempty"`
	PatternRe   *regexp.Regexp      `json:"-"` // from Pattern, used by Validate; not serialized
	Properties  map[string]Property `json:"properties,omitempty"`
	Required    []string            `json:"required,omitempty"`
	Items       *Property           `json:"items,omitempty"`
}

// Result holds the generated schema for a struct.
type Result struct {
	Properties map[string]Property
	Required   []string
}

var schemaCache sync.Map // reflect.Type → Result

// Generate produces schema properties and required list from a struct type.
func Generate(t reflect.Type) Result {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return Result{}
	}

	if cached, ok := schemaCache.Load(t); ok {
		return cached.(Result)
	}

	// Store a sentinel before recursing to break self-referential cycles.
	res := Result{Properties: make(map[string]Property)}
	schemaCache.Store(t, res)

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		name := fieldName(f)
		if name == "-" {
			continue
		}

		prop := fieldProp(f)
		res.Properties[name] = prop

		if hasTag(f, "required") {
			res.Required = append(res.Required, name)
		}
	}
	for k, prop := range res.Properties {
		initPropertyPattern(&prop)
		res.Properties[k] = prop
	}
	schemaCache.Store(t, res)
	return res
}

// initPropertyPattern compiles Pattern into PatternRe and recurses into nested properties.
func initPropertyPattern(p *Property) {
	if p == nil {
		return
	}
	p.PatternRe = nil
	if p.Pattern != "" {
		if re, err := regexp.Compile(p.Pattern); err == nil {
			p.PatternRe = re
		}
	}
	for k, sub := range p.Properties {
		initPropertyPattern(&sub)
		p.Properties[k] = sub
	}
	if p.Items != nil {
		initPropertyPattern(p.Items)
	}
}

func fieldName(f reflect.StructField) string {
	if tag := f.Tag.Get("json"); tag != "" {
		parts := strings.Split(tag, ",")
		if parts[0] != "" {
			return parts[0]
		}
	}
	return strings.ToLower(f.Name[:1]) + f.Name[1:]
}

func fieldProp(f reflect.StructField) Property {
	ft := f.Type
	if ft.Kind() == reflect.Ptr {
		ft = ft.Elem()
	}

	p := Property{}

	switch ft.Kind() {
	case reflect.Struct:
		nested := Generate(ft)
		p.Type = "object"
		p.Properties = nested.Properties
		p.Required = nested.Required
	case reflect.Slice, reflect.Array:
		p.Type = "array"
		elemType := ft.Elem()
		if elemType.Kind() == reflect.Ptr {
			elemType = elemType.Elem()
		}
		if elemType.Kind() == reflect.Struct {
			nested := Generate(elemType)
			elem := Property{Type: "object", Properties: nested.Properties, Required: nested.Required}
			p.Items = &elem
		} else {
			elem := Property{Type: goTypeToJSON(elemType.Kind())}
			p.Items = &elem
		}
	default:
		p.Type = goTypeToJSON(ft.Kind())
	}

	parseMCPTag(f, &p)
	return p
}

func parseMCPTag(f reflect.StructField, p *Property) {
	tag := f.Tag.Get("mcp")
	if tag == "" || tag == "-" {
		return
	}
	for _, part := range strings.Split(tag, ",") {
		part = strings.TrimSpace(part)
		k, v, _ := strings.Cut(part, "=")
		switch k {
		case "desc":
			p.Description = v
		case "default":
			p.Default = parseValue(v, f.Type)
		case "min":
			if n, err := strconv.ParseFloat(v, 64); err == nil {
				p.Minimum = &n
			}
		case "max":
			if n, err := strconv.ParseFloat(v, 64); err == nil {
				p.Maximum = &n
			}
		case "enum":
			for _, e := range strings.Split(v, "|") {
				p.Enum = append(p.Enum, e)
			}
		case "pattern":
			p.Pattern = v
		}
	}
}

func hasTag(f reflect.StructField, key string) bool {
	tag := f.Tag.Get("mcp")
	for _, part := range strings.Split(tag, ",") {
		if strings.TrimSpace(part) == key {
			return true
		}
	}
	return false
}

func parseValue(s string, t reflect.Type) any {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Int, reflect.Int64:
		if n, err := strconv.Atoi(s); err == nil {
			return n
		}
	case reflect.Float64:
		if n, err := strconv.ParseFloat(s, 64); err == nil {
			return n
		}
	case reflect.Bool:
		if b, err := strconv.ParseBool(s); err == nil {
			return b
		}
	}
	return s
}

func goTypeToJSON(k reflect.Kind) string {
	switch k {
	case reflect.String:
		return "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "integer"
	case reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Bool:
		return "boolean"
	default:
		return "string"
	}
}
