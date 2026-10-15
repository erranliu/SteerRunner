package contracts

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"unicode/utf8"
)

var errJSON = errors.New("invalid or ambiguous JSON")
var rawMessageType = reflect.TypeFor[json.RawMessage]()

// DecodeStrict rejects aliases, duplicate keys and multiple JSON values.
// Diagnostics deliberately exclude keys/values supplied by the caller.
func DecodeStrict(data []byte, target any) error {
	if !utf8.Valid(data) || len(data) == 0 {
		return errJSON
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := uniqueValue(decoder, 0); err != nil {
		return errJSON
	}
	if _, err := decoder.Token(); err != io.EOF {
		return errJSON
	}
	var value any
	decoder = json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return errJSON
	}
	if _, ok := value.(map[string]any); !ok {
		return errJSON
	}
	if err := exactFields(value, reflect.TypeOf(target)); err != nil {
		return errJSON
	}
	decoder = json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return errJSON
	}
	return nil
}

func uniqueValue(decoder *json.Decoder, depth int) error {
	if depth > 64 {
		return errJSON
	}
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, compound := token.(json.Delim)
	if !compound {
		return nil
	}
	switch delimiter {
	case '{':
		seen := map[string]bool{}
		for decoder.More() {
			key, err := decoder.Token()
			name, ok := key.(string)
			if err != nil || !ok || seen[name] {
				return errJSON
			}
			seen[name] = true
			if err := uniqueValue(decoder, depth+1); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			if err := uniqueValue(decoder, depth+1); err != nil {
				return err
			}
		}
	default:
		return errJSON
	}
	_, err = decoder.Token()
	return err
}

func exactFields(value any, kind reflect.Type) error {
	if value == nil {
		if kind.Kind() == reflect.Pointer || kind.Kind() == reflect.Interface || kind == rawMessageType {
			return nil
		}
		return errJSON
	}
	for kind.Kind() == reflect.Pointer {
		kind = kind.Elem()
	}
	if kind == rawMessageType || kind.Kind() == reflect.Interface {
		return nil
	}
	switch kind.Kind() {
	case reflect.Struct:
		object, ok := value.(map[string]any)
		if !ok {
			return errJSON
		}
		fields := map[string]reflect.Type{}
		for i := range kind.NumField() {
			field := kind.Field(i)
			if field.PkgPath != "" {
				continue
			}
			name := strings.Split(field.Tag.Get("json"), ",")[0]
			if name == "-" {
				continue
			}
			if name == "" {
				name = field.Name
			}
			fields[name] = field.Type
		}
		for name, child := range object {
			fieldType, ok := fields[name]
			if !ok || exactFields(child, fieldType) != nil {
				return errJSON
			}
		}
	case reflect.Slice, reflect.Array:
		if array, ok := value.([]any); ok {
			for _, child := range array {
				if err := exactFields(child, kind.Elem()); err != nil {
					return err
				}
			}
		}
	case reflect.Map:
		if object, ok := value.(map[string]any); ok {
			for _, child := range object {
				if err := exactFields(child, kind.Elem()); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
