// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package automationcontract

import (
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
)

func validateStructuredArgument(raw string, schema map[string]any) error {
	const maxSize = 128 << 10
	if len(raw) > maxSize {
		return errors.New("JSON argument exceeds 128 KiB")
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	value, err := readJSONValue(decoder, 0)
	if err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return errors.New("unexpected data after JSON value")
	}
	encoded, err := json.Marshal(schema)
	if err != nil {
		return err
	}
	var parsed jsonschema.Schema
	if err := json.Unmarshal(encoded, &parsed); err != nil {
		return err
	}
	resolved, err := parsed.Resolve(nil)
	if err != nil {
		return err
	}
	if err := resolved.Validate(value); err != nil {
		return errors.New("JSON argument does not match the operation schema")
	}
	return nil
}

func readJSONValue(decoder *json.Decoder, depth int) (any, error) {
	const maxDepth = 32
	if depth > maxDepth {
		return nil, errors.New("JSON argument is nested too deeply")
	}
	token, err := decoder.Token()
	if err != nil {
		return nil, errors.New("invalid JSON argument")
	}
	switch token {
	case json.Delim('{'):
		fields := make(map[string]any)
		for decoder.More() {
			key, err := decoder.Token()
			name, ok := key.(string)
			if err != nil || !ok {
				return nil, errors.New("invalid JSON object key")
			}
			if _, exists := fields[name]; exists {
				return nil, errors.New("duplicate JSON object key")
			}
			fields[name], err = readJSONValue(decoder, depth+1)
			if err != nil {
				return nil, err
			}
		}
		_, err := decoder.Token()
		return fields, err
	case json.Delim('['):
		items := make([]any, 0)
		for decoder.More() {
			item, err := readJSONValue(decoder, depth+1)
			if err != nil {
				return nil, err
			}
			items = append(items, item)
		}
		_, err := decoder.Token()
		return items, err
	default:
		return token, nil
	}
}
