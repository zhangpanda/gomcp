package gomcp

import (
	"reflect"

	"github.com/zhangpanda/gomcp/schema"
)

func generateSchema(t reflect.Type) JSONSchema {
	res := schema.Generate(t)
	return JSONSchema{
		Type:       "object",
		Properties: convertProps(res.Properties),
		Required:   res.Required,
	}
}

func convertProps(props map[string]schema.Property) map[string]JSONSchema {
	out := make(map[string]JSONSchema, len(props))
	for k, p := range props {
		out[k] = convertProp(p)
	}
	return out
}

func convertProp(p schema.Property) JSONSchema {
	s := JSONSchema{
		Type:        p.Type,
		Description: p.Description,
		Default:     p.Default,
		Enum:        p.Enum,
		Minimum:     p.Minimum,
		Maximum:     p.Maximum,
		Pattern:     p.Pattern,
		Required:    p.Required,
	}
	if p.Properties != nil {
		s.Properties = convertProps(p.Properties)
	}
	if p.Items != nil {
		converted := convertProp(*p.Items)
		s.Items = &converted
	}
	return s
}
