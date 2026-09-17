// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"Wavelet/plugins/domain/msg_gateway/model/do"
	"encoding/json"
	"strings"
)

// GetFlatBody flattens nested body map into dot-separated key-value map.
func GetFlatBody(body map[string]any) map[string]any {
	jsonBytes, err := json.Marshal(body)
	if err != nil {
		return body
	}
	var jsonMap map[string]any
	if err := json.Unmarshal(jsonBytes, &jsonMap); err != nil {
		return body
	}

	flatResult := make(map[string]any)
	FlattenMap("", jsonMap, flatResult)
	return flatResult
}

// FlattenMap recursively flattens map key-values with dot notation.
func FlattenMap(prefix string, m, result map[string]any) {
	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		if nestedMap, ok := v.(map[string]any); ok {
			FlattenMap(key, nestedMap, result)
		} else {
			result[key] = v
		}
	}
}

// RenderCustomPayload substitutes the supported template variables of a custom
// webhook body, JSON-escaping every injected value.
func RenderCustomPayload(template string, req do.CustomPushRequest) string {
	result := template
	result = strings.ReplaceAll(result, "$title", EscapeJSONString(req.Title))
	result = strings.ReplaceAll(result, "$description", EscapeJSONString(req.Description))
	result = strings.ReplaceAll(result, "$content", EscapeJSONString(req.Content))
	result = strings.ReplaceAll(result, "$url", EscapeJSONString(req.URL))
	result = strings.ReplaceAll(result, "$to", EscapeJSONString(req.To))
	return result
}

// EscapeJSONString renders s as a JSON string body without the surrounding quotes.
func EscapeJSONString(s string) string {
	b, _ := json.Marshal(s)
	const minJSONLen = 2
	if len(b) >= minJSONLen {
		return string(b[1 : len(b)-1])
	}
	return s
}
