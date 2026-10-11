package save

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"time"
)

func checkJSON(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := checkValue(dec, "$", 0); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return fmt.Errorf("save must contain exactly one JSON document")
	}
	return nil
}

func checkValue(dec *json.Decoder, path string, depth int) error {
	if depth > 64 {
		return fmt.Errorf("%s: nesting exceeds 64 levels", path)
	}
	token, err := dec.Token()
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if value, ok := token.(string); ok && len(value) > MaxStringBytes {
		return fmt.Errorf("string exceeds limit")
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	seen := map[string]bool{}
	count := 0
	for dec.More() {
		key := fmt.Sprintf("%s[%d]", path, count)
		if delimiter == '{' {
			raw, err := dec.Token()
			if err != nil {
				return err
			}
			name, ok := raw.(string)
			if !ok {
				return fmt.Errorf("%s: expected object key", path)
			}
			if len(name) > MaxStringBytes {
				return fmt.Errorf("object key exceeds limit")
			}
			if seen[name] {
				return fmt.Errorf("%s.%s: duplicate field", path, name)
			}
			seen[name] = true
			key = path + "." + name
		}
		count++
		if count > MaxEntries {
			return fmt.Errorf("%s: too many entries", path)
		}
		if err := checkValue(dec, key, depth+1); err != nil {
			return err
		}
	}
	_, err = dec.Token()
	return err
}

func checkFields(data []byte, target any) error {
	var raw any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&raw); err != nil {
		return err
	}
	return checkTypedFields(raw, reflect.ValueOf(target), "$")
}

type encodedField struct {
	value    reflect.Value
	required bool
}

func jsonFields(value reflect.Value) map[string]encodedField {
	fields := map[string]encodedField{}
	typ := value.Type()
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		if !f.IsExported() {
			continue
		}
		name := strings.Split(f.Tag.Get("json"), ",")[0]
		if name == "-" {
			continue
		}
		if f.Anonymous && name == "" {
			for k, v := range jsonFields(value.Field(i)) {
				fields[k] = v
			}
			continue
		}
		if name == "" {
			name = f.Name
		}
		fields[name] = encodedField{value: value.Field(i), required: !strings.Contains(f.Tag.Get("json"), ",omitempty") && !strings.Contains(f.Tag.Get("json"), ",omitzero")}
	}
	return fields
}

func checkTypedFields(raw any, value reflect.Value, path string) error {
	if !value.IsValid() {
		return fmt.Errorf("%s: invalid value", path)
	}
	if raw == nil {
		switch value.Kind() {
		case reflect.Pointer, reflect.Interface, reflect.Map, reflect.Slice:
			return nil
		default:
			return fmt.Errorf("%s: null is not allowed", path)
		}
	}
	for value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface {
		if value.IsNil() {
			if raw != nil {
				return fmt.Errorf("%s: invalid null value", path)
			}
			return nil
		}
		value = value.Elem()
	}
	if value.Type() == reflect.TypeFor[time.Time]() {
		return nil
	}
	switch node := raw.(type) {
	case map[string]any:
		switch value.Kind() {
		case reflect.Struct:
			fields := jsonFields(value)
			for name, field := range fields {
				if _, ok := node[name]; field.required && !ok {
					return fmt.Errorf("%s.%s: required field", path, name)
				}
			}
			for name, child := range node {
				field, ok := fields[name]
				if !ok {
					return fmt.Errorf("%s.%s: unknown field", path, name)
				}
				if err := checkTypedFields(child, field.value, path+"."+name); err != nil {
					return err
				}
			}
		case reflect.Map:
			for name, child := range node {
				key := reflect.New(value.Type().Key()).Elem()
				switch key.Kind() {
				case reflect.String:
					key.SetString(name)
				case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
					number, err := strconv.ParseInt(name, 10, key.Type().Bits())
					if err != nil {
						return fmt.Errorf("%s.%s: invalid integer key: %w", path, name, err)
					}
					key.SetInt(number)
				default:
					return fmt.Errorf("%s: unsupported map key type %s", path, key.Type())
				}
				if err := checkTypedFields(child, value.MapIndex(key), path+"."+name); err != nil {
					return err
				}
			}
		}
	case []any:
		if value.Kind() != reflect.Slice && value.Kind() != reflect.Array {
			return fmt.Errorf("%s: expected array", path)
		}
		if value.Len() != len(node) {
			return fmt.Errorf("%s: invalid array", path)
		}
		for i, child := range node {
			if err := checkTypedFields(child, value.Index(i), fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	}
	return nil
}
