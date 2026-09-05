package config

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// Get retrieves a configuration value using case-insensitive dot notation (e.g., "llm.provider").
func Get(cfg *Config, key string) (any, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, fmt.Errorf("unknown configuration key %q", key)
	}

	parts := strings.Split(key, ".")
	val := reflect.ValueOf(cfg)

	for _, part := range parts {
		if val.Kind() == reflect.Pointer {
			if val.IsNil() {
				return nil, fmt.Errorf("unknown configuration key %q", key)
			}
			val = val.Elem()
		}

		if val.Kind() != reflect.Struct {
			return nil, fmt.Errorf("unknown configuration key %q", key)
		}

		fieldVal, found := findField(val, part)
		if !found {
			return nil, fmt.Errorf("unknown configuration key %q", key)
		}
		val = fieldVal
	}

	return val.Interface(), nil
}

// Set validates and applies a string value to a configuration field via dot notation.
func Set(cfg *Config, key string, valStr string) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("unknown configuration key %q", key)
	}

	parts := strings.Split(key, ".")
	val := reflect.ValueOf(cfg)

	for i, part := range parts {
		if val.Kind() == reflect.Pointer {
			if val.IsNil() {
				if !val.CanSet() {
					return fmt.Errorf("unknown configuration key %q", key)
				}
				newElem := reflect.New(val.Type().Elem())
				val.Set(newElem)
			}
			val = val.Elem()
		}

		if val.Kind() != reflect.Struct {
			return fmt.Errorf("unknown configuration key %q", key)
		}

		fieldVal, found := findField(val, part)
		if !found {
			return fmt.Errorf("unknown configuration key %q", key)
		}

		if i == len(parts)-1 {
			// Leaf field to update
			return applyValue(fieldVal, valStr, key)
		}

		val = fieldVal
	}

	return nil
}

func findField(structVal reflect.Value, name string) (reflect.Value, bool) {
	typ := structVal.Type()
	normName := normalizeKey(name)

	for i := 0; i < structVal.NumField(); i++ {
		fieldMeta := typ.Field(i)
		fieldVal := structVal.Field(i)

		// 1. Check YAML tag
		yamlTag := fieldMeta.Tag.Get("yaml")
		tagKey := strings.Split(yamlTag, ",")[0]
		if tagKey != "" && normalizeKey(tagKey) == normName {
			return fieldVal, true
		}

		// 2. Check field name
		if normalizeKey(fieldMeta.Name) == normName {
			return fieldVal, true
		}
	}

	return reflect.Value{}, false
}

func normalizeKey(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "_", "")
	s = strings.ReplaceAll(s, "-", "")
	return s
}

func applyValue(field reflect.Value, valStr string, fullKey string) error {
	if !field.CanSet() {
		return fmt.Errorf("cannot set configuration key %q", fullKey)
	}

	// Handle time.Duration specially (which is an int64 under the hood)
	if field.Type() == reflect.TypeOf(time.Duration(0)) {
		d, err := time.ParseDuration(valStr)
		if err != nil {
			return fmt.Errorf("invalid duration value %q for key %q: %w", valStr, fullKey, err)
		}
		field.Set(reflect.ValueOf(d))
		return nil
	}

	switch field.Kind() {
	case reflect.String:
		field.SetString(valStr)
		return nil

	case reflect.Bool:
		b, err := strconv.ParseBool(valStr)
		if err != nil {
			return fmt.Errorf("invalid boolean value %q for key %q", valStr, fullKey)
		}
		field.SetBool(b)
		return nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(valStr, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid integer value %q for key %q", valStr, fullKey)
		}
		field.SetInt(n)
		return nil

	case reflect.Slice:
		if field.Type().Elem().Kind() == reflect.String {
			trimmed := strings.TrimSpace(valStr)
			if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
				var parsed []string
				if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
					return fmt.Errorf("invalid json string array %q for key %q: %w", valStr, fullKey, err)
				}
				field.Set(reflect.ValueOf(parsed))
				return nil
			}

			rawParts := strings.Split(valStr, ",")
			slice := make([]string, 0, len(rawParts))
			for _, p := range rawParts {
				p = strings.TrimSpace(p)
				if p != "" {
					slice = append(slice, p)
				}
			}
			field.Set(reflect.ValueOf(slice))
			return nil
		}
		return fmt.Errorf("unsupported slice element type %s for key %q", field.Type().Elem(), fullKey)

	default:
		return fmt.Errorf("unsupported type %s for key %q", field.Type(), fullKey)
	}
}
