// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package push

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"text/template"
	"time"
)

var (
	// placeholderRegex matches {{ ... }} tags
	placeholderRegex = regexp.MustCompile(`\{\{\s*([^}]+?)\s*\}\}`)
	// identifierRegex matches simple identifiers like name or user.username
	identifierRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_.]*$`)
)

// jsonMap is a map that serializes to JSON when printed as a string in templates.
type jsonMap map[string]any

func (m jsonMap) String() string {
	b, err := json.Marshal(map[string]any(m))
	if err != nil {
		return fmt.Sprintf("%v", map[string]any(m))
	}
	return string(b)
}

func (m jsonMap) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any(m))
}

// jsonSlice is a slice that serializes to JSON when printed as a string in templates.
type jsonSlice []any

func (s jsonSlice) String() string {
	b, err := json.Marshal([]any(s))
	if err != nil {
		return fmt.Sprintf("%v", []any(s))
	}
	return string(b)
}

func (s jsonSlice) MarshalJSON() ([]byte, error) {
	return json.Marshal([]any(s))
}

var defaultFuncMap = template.FuncMap{
	"default": func(fallback any, val any) any {
		if val == nil {
			return fallback
		}
		switch v := val.(type) {
		case string:
			if v == "" {
				return fallback
			}
		case bool:
			if !v {
				return fallback
			}
		case int:
			if v == 0 {
				return fallback
			}
		case int32:
			if v == 0 {
				return fallback
			}
		case int64:
			if v == 0 {
				return fallback
			}
		case float64:
			if v == 0 {
				return fallback
			}
		}
		return val
	},
	"toJson": func(v any) string {
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprint(v)
		}
		return string(b)
	},
	"upper": strings.ToUpper,
	"lower": strings.ToLower,
	"trim":  strings.TrimSpace,
	"dateFormat": func(format string, t any) string {
		switch v := t.(type) {
		case time.Time:
			return v.Format(format)
		case *time.Time:
			if v != nil {
				return v.Format(format)
			}
		}
		return fmt.Sprint(t)
	},
}

// hasKey checks if a dot-delimited or plain key exists in body
func hasKey(body map[string]any, key string) bool {
	if _, ok := body[key]; ok {
		return true
	}
	parts := strings.Split(key, ".")
	var cur any = body
	for _, part := range parts {
		m, ok := cur.(map[string]any)
		if !ok {
			return false
		}
		val, exists := m[part]
		if !exists {
			return false
		}
		cur = val
	}
	return true
}

// normalizeTemplate converts legacy {{key}} / {{user.name}} into Go template {{.user.name}}
// while preserving Go template keywords, dot expressions, pipelines, and missing placeholders.
func normalizeTemplate(tmpl string, body map[string]any) string {
	return placeholderRegex.ReplaceAllStringFunc(tmpl, func(match string) string {
		sub := strings.TrimSpace(match[2 : len(match)-2])
		if sub == "" {
			return match
		}
		// If it's already a dot expression or special variable ($...)
		if strings.HasPrefix(sub, ".") || strings.HasPrefix(sub, "$") {
			return match
		}
		// If it's a known Go template keyword or block
		firstWord := strings.Fields(sub)[0]
		switch firstWord {
		case "if", "else", "end", "range", "with", "template", "define", "block", "nil", "true", "false":
			return match
		}
		// Check if it's a pipeline like `key | default "val"`
		if strings.Contains(sub, "|") {
			const pipelineSplitCount = 2
			parts := strings.SplitN(sub, "|", pipelineSplitCount)
			left := strings.TrimSpace(parts[0])
			right := strings.TrimSpace(parts[1])
			if identifierRegex.MatchString(left) && !strings.HasPrefix(left, ".") && !strings.HasPrefix(left, "$") {
				return fmt.Sprintf("{{ .%s | %s }}", left, right)
			}
			return match
		}
		// Simple identifier: if present in body, convert to dot expression; otherwise preserve as is for fallback
		if identifierRegex.MatchString(sub) {
			if hasKey(body, sub) {
				return fmt.Sprintf("{{ .%s }}", sub)
			}
			// Missing key: keep original text so fallback or literal is preserved
			return match
		}
		return match
	})
}

// prepareContext pre-processes the body map so that:
// 1. Dotted keys like "user.name" are expanded to nested map structure.
// 2. Complex structs, slices, and maps have JSON-friendly string representations when directly interpolated.
func prepareContext(body map[string]any) jsonMap {
	if body == nil {
		return make(jsonMap)
	}
	ctx := make(jsonMap, len(body))
	for k, v := range body {
		formatted := formatContextValue(v)
		ctx[k] = formatted
		// If key contains '.', expand into nested hierarchy
		if strings.Contains(k, ".") {
			parts := strings.Split(k, ".")
			cur := ctx
			for i := 0; i < len(parts)-1; i++ {
				sub, ok := cur[parts[i]].(jsonMap)
				if !ok {
					sub = make(jsonMap)
					cur[parts[i]] = sub
				}
				cur = sub
			}
			cur[parts[len(parts)-1]] = formatted
		}
	}
	return ctx
}

// formatContextValue formats slices and maps to JSON representation for direct string printing,
// while preserving basic scalar types for template functions.
func formatContextValue(v any) any {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, bool, time.Time:
		return val
	case []byte:
		return string(val)
	case map[string]any:
		jm := make(jsonMap, len(val))
		for k, subVal := range val {
			jm[k] = formatContextValue(subVal)
		}
		return jm
	case []any:
		js := make(jsonSlice, len(val))
		for i, subVal := range val {
			js[i] = formatContextValue(subVal)
		}
		return js
	case []string:
		js := make(jsonSlice, len(val))
		for i, subVal := range val {
			js[i] = subVal
		}
		return js
	default:
		return val
	}
}

// ParseTemplate parses template strings by replacing {{placeholder}} structures with values from body.
// It supports Go text/template expressions (e.g. if/else, pipelines, default, toJson) as well as legacy {{key}} placeholders.
func ParseTemplate(templateStr string, body map[string]any) string {
	if templateStr == "" {
		return ""
	}

	normalized := normalizeTemplate(templateStr, body)
	ctx := prepareContext(body)

	tmpl, err := template.New("push_tmpl").
		Funcs(defaultFuncMap).
		Option("missingkey=zero").
		Parse(normalized)
	if err != nil {
		return fallbackReplace(templateStr, body)
	}

	var buf bytes.Buffer
	if err = tmpl.Execute(&buf, ctx); err != nil {
		return fallbackReplace(templateStr, body)
	}

	return buf.String()
}

func fallbackReplace(template string, body map[string]any) string {
	var buf strings.Builder
	buf.Grow(len(template))

	i := 0
	for {
		pos := strings.Index(template[i:], "{{")
		if pos == -1 {
			buf.WriteString(template[i:])
			break
		}
		buf.WriteString(template[i : i+pos])
		i += pos + 2

		endPos := strings.Index(template[i:], "}}")
		if endPos == -1 {
			buf.WriteString("{{")
			buf.WriteString(template[i:])
			break
		}
		key := strings.TrimSpace(template[i : i+endPos])
		key = strings.TrimPrefix(key, ".")
		if val, ok := body[key]; ok {
			buf.WriteString(formatValue(val))
		} else {
			buf.WriteString("{{")
			buf.WriteString(template[i : i+endPos])
			buf.WriteString("}}")
		}
		i += endPos + 2
	}
	return buf.String()
}

func formatValue(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	case int:
		return strconv.Itoa(val)
	case int32:
		return strconv.FormatInt(int64(val), 10)
	case int64:
		return strconv.FormatInt(val, 10)
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(val)
	default:
		b, err := json.Marshal(v)
		if err == nil {
			return string(b)
		}
		return fmt.Sprintf("%v", v)
	}
}

// bodyTitle returns the notification title, falling back to the default.
func bodyTitle(body map[string]any) string {
	if t, ok := body["title"].(string); ok && t != "" {
		return t
	}
	return defaultTitle
}

// bodyContent returns the notification body, rendering every entry with format
// (a "%s … %v" pair) and joining them with sep when no content field is given.
// Entries render in sorted key order so identical bodies always produce
// identical text.
func bodyContent(body map[string]any, format, sep string) string {
	if c, ok := body["content"].(string); ok && c != "" {
		return c
	}
	parts := make([]string, 0, len(body))
	for _, k := range slices.Sorted(maps.Keys(body)) {
		parts = append(parts, fmt.Sprintf(format, k, body[k]))
	}
	return strings.Join(parts, sep)
}
