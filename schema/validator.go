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
	var errs []FieldError

	// check required
	for _, name := range s.Required {
		if _, ok := args[name]; !ok {
			errs = append(errs, FieldError{Field: name, Message: "required"})
		}
	}

	// check constraints per field
	for name, prop := range s.Properties {
		val, ok := args[name]
		if !ok {
			continue
		}
		if fieldErrs := validateField(name, val, prop); len(fieldErrs) > 0 {
			errs = append(errs, fieldErrs...)
		}
	}

	if len(errs) > 0 {
		return &ValidationError{Fields: errs}
	}
	return nil
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
