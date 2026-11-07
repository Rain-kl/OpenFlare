// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package i18n translates built-in server text at presentation boundaries.
// Runtime diagnostics and stored task text remain English.
package i18n

import (
	"embed"
	"encoding/json"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

//go:embed locales/*.json
var catalogFiles embed.FS

// DefaultLocale is the source language used for API clients without a preference.
const DefaultLocale = "en"

type textPattern struct {
	id     string
	match  *regexp.Regexp
	prefix string
	suffix string
	score  int
}

type catalog struct {
	locales  map[string]map[string]string
	exact    map[string]string
	patterns []textPattern
}

var (
	printfVerb = regexp.MustCompile(`%(?:\[[0-9]+\])?[-+# 0]*(?:[0-9]+|\*)?(?:\.(?:[0-9]+|\*))?[a-zA-Z%]`)
	builtIn    = loadCatalog()
)

func loadCatalog() catalog {
	result := catalog{locales: make(map[string]map[string]string), exact: make(map[string]string)}
	files, err := fs.Glob(catalogFiles, "locales/*.json")
	if err != nil {
		panic(err)
	}
	for _, file := range files {
		data, readErr := catalogFiles.ReadFile(file)
		if readErr != nil {
			panic(readErr)
		}
		var messages map[string]string
		if decodeErr := json.Unmarshal(data, &messages); decodeErr != nil {
			panic(decodeErr)
		}
		locale := strings.TrimSuffix(path.Base(file), ".json")
		result.locales[locale] = messages
	}
	locales := make([]string, 0, len(result.locales))
	for locale := range result.locales {
		locales = append(locales, locale)
	}
	sort.Strings(locales)
	for _, locale := range locales {
		messages := result.locales[locale]
		ids := make([]string, 0, len(messages))
		for id := range messages {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			text := messages[id]
			if _, exists := result.exact[text]; !exists {
				result.exact[text] = id
			}
			if pattern, ok := compilePattern(id, text); ok {
				result.patterns = append(result.patterns, pattern)
			}
		}
	}
	// Go errors use a lowercase initial. Recognize the earlier capitalized
	// English spelling too when displaying existing task errors and logs.
	english := result.locales[DefaultLocale]
	ids := make([]string, 0, len(english))
	for id := range english {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		text := english[id]
		if text == "" || text[0] < 'a' || text[0] > 'z' {
			continue
		}
		legacy := strings.ToUpper(text[:1]) + text[1:]
		if _, exists := result.exact[legacy]; exists {
			continue
		}
		result.exact[legacy] = id
		if pattern, ok := compilePattern(id, legacy); ok {
			result.patterns = append(result.patterns, pattern)
		}
	}
	sort.SliceStable(result.patterns, func(left, right int) bool {
		return result.patterns[left].score > result.patterns[right].score
	})
	return result
}

func splitFormat(format string) ([]string, bool) {
	indices := printfVerb.FindAllStringIndex(format, -1)
	if len(indices) == 0 {
		// Some existing boundaries append an external error to a literal prefix.
		if strings.HasSuffix(format, ": ") {
			return []string{format, ""}, true
		}
		return nil, false
	}
	parts := []string{}
	start := 0
	for _, index := range indices {
		if format[index[0]:index[1]] == "%%" {
			continue
		}
		parts = append(parts, strings.ReplaceAll(format[start:index[0]], "%%", "%"))
		start = index[1]
	}
	if len(parts) == 0 {
		return nil, false
	}
	parts = append(parts, strings.ReplaceAll(format[start:], "%%", "%"))
	return parts, true
}

func compilePattern(id, format string) (textPattern, bool) {
	parts, ok := splitFormat(format)
	if !ok {
		return textPattern{}, false
	}
	score := 0
	var pattern strings.Builder
	pattern.WriteString("(?s)^")
	for index, part := range parts {
		score += len(part)
		if index > 0 {
			pattern.WriteString("(.*?)")
		}
		pattern.WriteString(regexp.QuoteMeta(part))
	}
	pattern.WriteString("$")
	if score == 0 {
		return textPattern{}, false
	}
	return textPattern{id: id, match: regexp.MustCompile(pattern.String()), prefix: parts[0], suffix: parts[len(parts)-1], score: score}, true
}

// ResolveLocale honors a supported locale cookie, then Accept-Language, then English.
func ResolveLocale(cookie, acceptLanguage string) string {
	if locale, ok := supportedLocale(cookie); ok {
		return locale
	}
	type preference struct {
		locale string
		weight float64
	}
	preferences := make([]preference, 0)
	for _, item := range strings.Split(acceptLanguage, ",") {
		fields := strings.Split(item, ";")
		locale, ok := supportedLocale(fields[0])
		if !ok {
			continue
		}
		weight := 1.0
		for _, field := range fields[1:] {
			if value, found := strings.CutPrefix(strings.TrimSpace(field), "q="); found {
				parsed, err := strconv.ParseFloat(value, 64)
				if err != nil || parsed < 0 || parsed > 1 {
					weight = 0
				} else {
					weight = parsed
				}
			}
		}
		if weight > 0 {
			preferences = append(preferences, preference{locale: locale, weight: weight})
		}
	}
	sort.SliceStable(preferences, func(left, right int) bool {
		return preferences[left].weight > preferences[right].weight
	})
	if len(preferences) > 0 {
		return preferences[0].locale
	}
	return DefaultLocale
}

func supportedLocale(value string) (string, bool) {
	value = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), "_", "-"))
	for locale := range builtIn.locales {
		if strings.EqualFold(locale, value) {
			return locale, true
		}
	}
	base, _, _ := strings.Cut(value, "-")
	if base == "zh" {
		_, ok := builtIn.locales["zh-CN"]
		return "zh-CN", ok
	}
	_, ok := builtIn.locales[base]
	return base, ok
}

// Translate renders known system text without modifying interpolated values.
// Both English and historical translated templates are recognized. Unknown text
// is returned unchanged; this function must only be used for system-owned fields.
func Translate(locale, text string) string {
	// Authentication responses include stable control prefixes consumed by the
	// login UI. Translate their human-readable suffix, preserving the protocol.
	for _, prefix := range []string{"smtp_invalid:", "need_email_code:"} {
		if suffix, ok := strings.CutPrefix(text, prefix); ok {
			return prefix + translate(locale, suffix)
		}
	}
	return translate(locale, text)
}

func translate(locale, text string) string {
	messages, ok := builtIn.locales[locale]
	if !ok {
		messages = builtIn.locales[DefaultLocale]
	}
	if id, found := builtIn.exact[text]; found {
		if translated, exists := messages[id]; exists {
			return translated
		}
		return text
	}
	// English diagnostics are already in the source language. Avoid scanning the
	// legacy catalog for ordinary English log lines.
	if locale == DefaultLocale && !strings.ContainsFunc(text, func(r rune) bool { return unicode.Is(unicode.Han, r) }) {
		return text
	}
	for _, pattern := range builtIn.patterns {
		if !strings.HasPrefix(text, pattern.prefix) || !strings.HasSuffix(text, pattern.suffix) {
			continue
		}
		matches := pattern.match.FindStringSubmatch(text)
		if len(matches) == 0 {
			continue
		}
		translated, found := messages[pattern.id]
		if !found {
			return text
		}
		parts, valid := splitFormat(translated)
		if !valid || len(parts) != len(matches) {
			return text
		}
		var output strings.Builder
		for index, part := range parts {
			if index > 0 {
				output.WriteString(matches[index])
			}
			output.WriteString(part)
		}
		return output.String()
	}
	return text
}

// TranslateLog preserves timestamps and unknown legacy lines while translating
// the known system template on each line. Stored log contents are never rewritten.
func TranslateLog(locale, text string) string {
	lines := strings.Split(text, "\n")
	for index, line := range lines {
		prefix := ""
		if len(line) >= len("[00:00:00] ") && line[0] == '[' && line[9] == ']' && line[10] == ' ' {
			prefix, line = line[:11], line[11:]
		}
		lines[index] = prefix + Translate(locale, line)
	}
	return strings.Join(lines, "\n")
}
