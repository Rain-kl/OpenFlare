// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package extpoints

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// convertValue coerces a raw value coming from the configuration file or an
// environment variable into the declared Go type.
func convertValue(raw any, typ reflect.Type) (any, error) {
	if typ == durationType {
		return convertDuration(raw)
	}

	switch typ.Kind() {
	case reflect.Bool:
		return convertBool(raw)
	case reflect.String:
		return convertString(raw)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return convertNumeric(raw, typ, signedNumbers)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return convertNumeric(raw, typ, unsignedNumbers)
	case reflect.Float32, reflect.Float64:
		return convertNumeric(raw, typ, floatingNumbers)
	case reflect.Slice:
		return convertSlice(raw, typ)
	case reflect.Struct:
		return convertStruct(raw, typ)
	case reflect.Pointer:
		return convertPointer(raw, typ)
	default:
		return nil, fmt.Errorf("%w: %s is not a supported configuration type", ErrConfigType, typ)
	}
}

// convertPointer decodes into the element type and returns a non-nil pointer to it.
// Nested pointers are rejected so configuration tags stay one level deep.
func convertPointer(raw any, typ reflect.Type) (any, error) {
	elemType := typ.Elem()
	if elemType.Kind() == reflect.Pointer {
		return nil, fmt.Errorf("%w: %s is not a supported configuration type", ErrConfigType, typ)
	}
	elem, err := convertValue(raw, elemType)
	if err != nil {
		return nil, err
	}
	ptr := reflect.New(elemType)
	ptr.Elem().Set(reflect.ValueOf(elem))
	return ptr.Interface(), nil
}

func convertBool(raw any) (any, error) {
	switch v := raw.(type) {
	case bool:
		return v, nil
	case *bool:
		if v == nil {
			return nil, fmt.Errorf("%w: nil *bool is not a boolean", ErrConfigType)
		}
		return *v, nil
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(v))
		if err != nil {
			return nil, fmt.Errorf("%w: %q is not a boolean", ErrConfigType, v)
		}
		return parsed, nil
	default:
		return nil, fmt.Errorf("%w: %v is not a boolean", ErrConfigType, raw)
	}
}

func convertString(raw any) (any, error) {
	switch v := raw.(type) {
	case string:
		return v, nil
	case bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return fmt.Sprint(v), nil
	default:
		return nil, fmt.Errorf("%w: %v is not a string", ErrConfigType, raw)
	}
}

// numericString extracts the textual form of a value so environment overrides,
// which always arrive as strings, share one parsing path with file values.
func numericString(raw any) (string, bool) {
	switch v := raw.(type) {
	case string:
		return strings.TrimSpace(v), true
	case bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return fmt.Sprint(v), true
	default:
		return "", false
	}
}

// numericKind selects which strconv family converts a raw value.
type numericKind int

const (
	signedNumbers numericKind = iota
	unsignedNumbers
	floatingNumbers
)

// convertNumeric parses a raw value into the numeric type declared by typ. The three
// numeric families share one implementation because they differ only in the strconv
// call and the reflect setter.
func convertNumeric(raw any, typ reflect.Type, family numericKind) (any, error) {
	text, ok := numericString(raw)
	if !ok {
		return nil, fmt.Errorf("%w: %v is not a %s", ErrConfigType, raw, typ)
	}

	out := reflect.New(typ).Elem()
	var err error

	switch family {
	case signedNumbers:
		parsed, parseErr := strconv.ParseInt(text, 10, typ.Bits())
		out.SetInt(parsed)
		err = parseErr
	case unsignedNumbers:
		parsed, parseErr := strconv.ParseUint(text, 10, typ.Bits())
		out.SetUint(parsed)
		err = parseErr
	default:
		parsed, parseErr := strconv.ParseFloat(text, typ.Bits())
		out.SetFloat(parsed)
		err = parseErr
	}

	if err != nil {
		return nil, fmt.Errorf("%w: %q is not a valid %s", ErrConfigType, text, typ)
	}
	return out.Interface(), nil
}

// convertDuration accepts both Go duration strings such as "200ms" and integer
// nanoseconds, mirroring what the previous viper based decoding supported.
func convertDuration(raw any) (any, error) {
	if v, ok := raw.(time.Duration); ok {
		return v, nil
	}

	text, ok := numericString(raw)
	if !ok {
		return nil, fmt.Errorf("%w: %v is not a duration", ErrConfigType, raw)
	}

	if parsed, err := time.ParseDuration(text); err == nil {
		return parsed, nil
	}

	nanos, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("%w: %q is not a valid duration", ErrConfigType, text)
	}
	return time.Duration(nanos), nil
}

// convertSlice promotes a scalar into a single-element slice so that a value such as
// REDIS_ADDR=redis:6379 can populate the redis.addrs list.
func convertSlice(raw any, typ reflect.Type) (any, error) {
	items, ok := sliceItems(raw)
	if !ok {
		items = []any{raw}
	}

	out := reflect.MakeSlice(typ, 0, len(items))
	for _, item := range items {
		converted, err := convertValue(item, typ.Elem())
		if err != nil {
			return nil, err
		}
		out = reflect.Append(out, reflect.ValueOf(converted))
	}
	return out.Interface(), nil
}

// sliceItems normalises the several slice shapes a loader may produce.
func sliceItems(raw any) ([]any, bool) {
	switch v := raw.(type) {
	case []any:
		return v, true
	case []string:
		items := make([]any, len(v))
		for i, s := range v {
			items[i] = s
		}
		return items, true
	}

	rv := reflect.ValueOf(raw)
	if rv.IsValid() && rv.Kind() == reflect.Slice {
		items := make([]any, rv.Len())
		for i := range items {
			items[i] = rv.Index(i).Interface()
		}
		return items, true
	}
	return nil, false
}

func convertStruct(raw any, typ reflect.Type) (any, error) {
	table, ok := asStringMap(raw)
	if !ok {
		return nil, fmt.Errorf("%w: %v is not a mapping, cannot decode into %s", ErrConfigType, raw, typ)
	}

	fields, err := walkConfigFields(typ, "")
	if err != nil {
		return nil, err
	}

	out := reflect.New(typ).Elem()
	for _, f := range fields {
		item, present := table[f.path]
		if !present || item == nil {
			continue
		}

		converted, err := convertValue(item, f.typ)
		if err != nil {
			return nil, fmt.Errorf("%w: %s.%s: %w", ErrConfigType, typ.Name(), f.key, err)
		}
		out.FieldByName(fieldNameForPath(typ, f.path)).Set(reflect.ValueOf(converted))
	}
	return out.Interface(), nil
}

// asStringMap normalises the two map shapes produced by YAML decoders.
func asStringMap(raw any) (map[string]any, bool) {
	switch v := raw.(type) {
	case map[string]any:
		return v, true
	case map[any]any:
		out := make(map[string]any, len(v))
		for key, val := range v {
			name, ok := key.(string)
			if !ok {
				return nil, false
			}
			out[name] = val
		}
		return out, true
	default:
		return nil, false
	}
}

// fieldNameForPath maps a declared config path back to the Go struct field carrying it.
func fieldNameForPath(t reflect.Type, path string) string {
	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).Tag.Get("config") == path {
			return t.Field(i).Name
		}
	}
	return ""
}
