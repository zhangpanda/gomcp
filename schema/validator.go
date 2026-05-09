package schema

import (
	"fmt"
	"regexp"
	"strings"
)

// ValidationError holds field-level validation failures.
type ValidationError struct {
	Fields []FieldError
}

// FieldError describes a validation failure for a single field.
type FieldError struct {
	Field   string
	Message string
}

// Error returns a semicolon-separated list of field errors.
func (e *ValidationError) Error() string {
	msgs := make([]string, len(e.Fields))
	for i, f := range e.Fields {
		msgs[i] = f.Field + ": " + f.Message
	}
	return strings.Join(msgs, "; ")
}

// Validate checks args against a schema Result. Returns nil if valid.
func Validate(args map[string]any, s Result) error {
	errs := validateObject("", args, s.Properties, s.Required)
	if len(errs) > 0 {
		return &ValidationError{Fields: errs}
	}
	return nil
}

// validateObject validates a map against a set of property schemas and a
// required list. Used both at the top level and recursively for nested
// "type: object" fields.
func validateObject(pathPrefix string, args map[string]any, props map[string]Property, required []string) []FieldError {
	var errs []FieldError

	// required check
	for _, name := range required {
		if _, ok := args[name]; !ok {
			errs = append(errs, FieldError{Field: joinPath(pathPrefix, name), Message: "required"})
		}
	}

	// per-field constraints
	for name, prop := range props {
		val, ok := args[name]
		if !ok {
			continue
		}
		if fieldErrs := validateField(joinPath(pathPrefix, name), val, prop); len(fieldErrs) > 0 {
			errs = append(errs, fieldErrs...)
		}
	}

	return errs
}

// joinPath composes a dotted error path for nested validation.
func joinPath(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}

func validateField(name string, val any, prop Property) []FieldError {
	var errs []FieldError

	// enum check
	if len(prop.Enum) > 0 {
		found := false
		for _, e := range prop.Enum {
			if fmt.Sprintf("%v", e) == fmt.Sprintf("%v", val) {
				found = true
				break
			}
		}
		if !found {
			errs = append(errs, FieldError{Field: name, Message: fmt.Sprintf("must be one of %v", prop.Enum)})
		}
	}

	// numeric range
	if num, ok := toFloat(val); ok {
		if prop.Minimum != nil && num < *prop.Minimum {
			errs = append(errs, FieldError{Field: name, Message: fmt.Sprintf("must be >= %v", *prop.Minimum)})
		}
		if prop.Maximum != nil && num > *prop.Maximum {
			errs = append(errs, FieldError{Field: name, Message: fmt.Sprintf("must be <= %v", *prop.Maximum)})
		}
	}

	// pattern (use PatternRe from [Generate] / [initPropertyPattern] when present)
	if s, ok := val.(string); ok {
		if prop.PatternRe != nil {
			if !prop.PatternRe.MatchString(s) {
				errs = append(errs, FieldError{Field: name, Message: fmt.Sprintf("must match pattern %s", prop.Pattern)})
			}
		} else if prop.Pattern != "" {
			if matched, _ := regexp.MatchString(prop.Pattern, s); !matched {
				errs = append(errs, FieldError{Field: name, Message: fmt.Sprintf("must match pattern %s", prop.Pattern)})
			}
		}
	}

	// Recurse into nested objects / arrays so that deep violations are
	// reported. Before this was added, required keys and constraints on
	// nested structs were silently ignored.
	switch prop.Type {
	case "object":
		if m, ok := val.(map[string]any); ok {
			if len(prop.Properties) > 0 || len(prop.Required) > 0 {
				errs = append(errs, validateObject(name, m, prop.Properties, prop.Required)...)
			}
		}
	case "array":
		if prop.Items != nil {
			// JSON decoding commonly yields []any; accept the concrete
			// []string / []float64 / []map[string]any cases too.
			switch arr := val.(type) {
			case []any:
				for i, item := range arr {
					errs = append(errs, validateField(fmt.Sprintf("%s[%d]", name, i), item, *prop.Items)...)
				}
			case []string:
				for i, item := range arr {
					errs = append(errs, validateField(fmt.Sprintf("%s[%d]", name, i), item, *prop.Items)...)
				}
			case []map[string]any:
				for i, item := range arr {
					errs = append(errs, validateField(fmt.Sprintf("%s[%d]", name, i), item, *prop.Items)...)
				}
			}
		}
	}

	return errs
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}
