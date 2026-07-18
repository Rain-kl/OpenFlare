// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package analytics

import "strings"

const (
	uaLabelUnknown = "Unknown"
	uaLabelBot     = "Bot"
	uaLabelOther   = "Other"
	uaTokenBot     = "bot"
	uaTokenAndroid = "android"
	uaTokenSpider  = "spider"
	uaTokenCrawler = "crawler"
)

type uaMatchRule struct {
	label    string
	contains []string
	allOf    []string
	noneOf   []string
}

func matchUARules(uaLower string, rules []uaMatchRule, fallback string) string {
	if uaLower == "" {
		return uaLabelUnknown
	}
	for _, rule := range rules {
		matched := false
		for _, token := range rule.contains {
			if strings.Contains(uaLower, token) {
				matched = true
				break
			}
		}
		if !matched && len(rule.allOf) > 0 {
			matched = true
			for _, token := range rule.allOf {
				if !strings.Contains(uaLower, token) {
					matched = false
					break
				}
			}
		}
		if !matched {
			continue
		}
		excluded := false
		for _, token := range rule.noneOf {
			if strings.Contains(uaLower, token) {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}
		return rule.label
	}
	return fallback
}

var browserRules = []uaMatchRule{
	{label: "WeChat", contains: []string{"micromessenger"}},
	{label: "Postman", contains: []string{"postman"}},
	{label: "CLI", contains: []string{"curl/", "wget/"}},
	{label: "Edge", contains: []string{"edg/", "edgios/", "edga/"}},
	{label: "Opera", contains: []string{"opr/", "opera"}},
	{label: "Firefox", contains: []string{"firefox", "fxios"}},
	{label: "Chrome", contains: []string{"crios", "chrome"}, noneOf: []string{"chromium"}},
	{label: "Chromium", contains: []string{"chromium"}},
	{label: "Safari", contains: []string{"safari"}},
	{label: uaLabelBot, contains: []string{uaTokenBot, uaTokenSpider, uaTokenCrawler, "slurp"}},
}

var osRules = []uaMatchRule{
	{label: "Android", contains: []string{uaTokenAndroid}},
	{label: "iOS", contains: []string{"iphone", "ipad", "ipod", "ios"}},
	{label: "Windows", contains: []string{"windows"}},
	{label: "macOS", contains: []string{"mac os x", "macintosh", "macos"}},
	{label: "Chrome OS", contains: []string{"cros"}},
	{label: "Linux", contains: []string{"linux"}},
	{label: uaLabelBot, contains: []string{uaTokenBot, uaTokenSpider, uaTokenCrawler}},
}

var deviceRules = []uaMatchRule{
	{
		label:    uaLabelBot,
		contains: []string{uaTokenBot, uaTokenSpider, uaTokenCrawler, "slurp", "curl/", "wget/", "python-requests", "go-http-client", "postman"},
	},
	{
		label:    "Tablet",
		contains: []string{"ipad", "tablet"},
	},
	{
		label:  "Tablet",
		allOf:  []string{uaTokenAndroid},
		noneOf: []string{"mobile"},
	},
	{
		label:    "Mobile",
		contains: []string{"mobi", "iphone", "ipod", uaTokenAndroid},
	},
}

// ParseBrowserName performs lightweight User-Agent browser identification.
func ParseBrowserName(ua string) string {
	return matchUARules(strings.ToLower(ua), browserRules, uaLabelOther)
}

// ParseOSName performs lightweight User-Agent OS identification.
func ParseOSName(ua string) string {
	return matchUARules(strings.ToLower(ua), osRules, uaLabelOther)
}

// ParseDeviceType performs lightweight User-Agent device type identification.
func ParseDeviceType(ua string) string {
	return matchUARules(strings.ToLower(ua), deviceRules, "Desktop")
}
