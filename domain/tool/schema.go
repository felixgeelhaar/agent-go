package tool

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Schema wraps JSON Schema for input/output validation.
type Schema struct {
	raw json.RawMessage
}

// NewSchema creates a schema from raw JSON.
func NewSchema(raw json.RawMessage) Schema {
	return Schema{raw: raw}
}

// EmptySchema returns a schema that accepts any input.
func EmptySchema() Schema {
	return Schema{raw: json.RawMessage(`{}`)}
}

// ObjectSchema returns a schema for an object with the given properties.
func ObjectSchema(properties map[string]json.RawMessage, required []string) Schema {
	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	raw, _ := json.Marshal(schema)
	return Schema{raw: raw}
}

// Raw returns the underlying JSON schema.
func (s Schema) Raw() json.RawMessage {
	return s.raw
}

// IsEmpty returns true if the schema is empty or nil.
func (s Schema) IsEmpty() bool {
	return len(s.raw) == 0 || string(s.raw) == "{}" || string(s.raw) == "null"
}

// Validate validates data against a practical JSON Schema subset:
// top-level type, required object properties, and per-property type checks.
// Nested schemas beyond one property level and keywords like pattern/minimum
// are not enforced here; use a full Draft 2020-12 validator in infrastructure
// when needed.
func (s Schema) Validate(data json.RawMessage) error {
	if s.IsEmpty() {
		return nil
	}
	if !json.Valid(data) {
		return errors.New("invalid JSON")
	}

	var schema map[string]any
	if err := json.Unmarshal(s.raw, &schema); err != nil {
		return fmt.Errorf("invalid schema: %w", err)
	}

	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return errors.New("invalid JSON")
	}

	return validateAgainst(schema, value, "")
}

func validateAgainst(schema map[string]any, value any, path string) error {
	if typ, ok := schema["type"].(string); ok {
		if err := checkJSONType(typ, value, path); err != nil {
			return err
		}
	}

	if typ, _ := schema["type"].(string); typ == "object" || typ == "" {
		obj, ok := value.(map[string]any)
		if !ok {
			if typ == "object" {
				return typeError(path, "object", value)
			}
			return nil
		}

		if required, ok := schema["required"].([]any); ok {
			for _, r := range required {
				key, ok := r.(string)
				if !ok {
					continue
				}
				if _, exists := obj[key]; !exists {
					return fmt.Errorf("%w: missing required property %q", ErrInvalidInput, fieldPath(path, key))
				}
			}
		}

		props, _ := schema["properties"].(map[string]any)
		for key, propSchemaRaw := range props {
			propSchema, ok := propSchemaRaw.(map[string]any)
			if !ok {
				continue
			}
			v, exists := obj[key]
			if !exists {
				continue
			}
			if err := validateAgainst(propSchema, v, fieldPath(path, key)); err != nil {
				return err
			}
		}
	}

	return nil
}

func checkJSONType(typ string, value any, path string) error {
	switch typ {
	case "object":
		if _, ok := value.(map[string]any); !ok {
			return typeError(path, "object", value)
		}
	case "array":
		if _, ok := value.([]any); !ok {
			return typeError(path, "array", value)
		}
	case "string":
		if _, ok := value.(string); !ok {
			return typeError(path, "string", value)
		}
	case "number":
		if _, ok := value.(float64); !ok {
			return typeError(path, "number", value)
		}
	case "integer":
		n, ok := value.(float64)
		if !ok || n != float64(int64(n)) {
			return typeError(path, "integer", value)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return typeError(path, "boolean", value)
		}
	case "null":
		if value != nil {
			return typeError(path, "null", value)
		}
	}
	return nil
}

func typeError(path, want string, value any) error {
	got := jsonTypeName(value)
	if path == "" {
		return fmt.Errorf("%w: expected type %s, got %s", ErrInvalidInput, want, got)
	}
	return fmt.Errorf("%w: %s: expected type %s, got %s", ErrInvalidInput, path, want, got)
}

func jsonTypeName(value any) string {
	switch value.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case float64:
		return "number"
	case string:
		return "string"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		return "unknown"
	}
}

func fieldPath(parent, key string) string {
	if parent == "" {
		return key
	}
	return parent + "." + key
}

// MarshalJSON implements json.Marshaler.
func (s Schema) MarshalJSON() ([]byte, error) {
	if s.raw == nil {
		return []byte("{}"), nil
	}
	return s.raw, nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (s *Schema) UnmarshalJSON(data []byte) error {
	s.raw = data
	return nil
}
