// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package openresty

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const (
	// StatusCodeMin is the lowest HTTP status code accepted for origin error pages.
	StatusCodeMin = 400
	// StatusCodeMax is the highest HTTP status code accepted for origin error pages.
	StatusCodeMax = 599
)

// ParseStatusCodeTag parses a single tag such as "502" or "500-599".
// Bounds must fall within StatusCodeMin–StatusCodeMax inclusive.
func ParseStatusCodeTag(tag string) (lo, hi int, err error) {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return 0, 0, errors.New("status code tag must not be empty")
	}
	if before, after, ok := strings.Cut(tag, "-"); ok {
		lo, err = strconv.Atoi(before)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid status code range: %s", tag)
		}
		hi, err = strconv.Atoi(after)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid status code range: %s", tag)
		}
	} else {
		lo, err = strconv.Atoi(tag)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid status code: %s", tag)
		}
		hi = lo
	}
	if lo > hi {
		return 0, 0, fmt.Errorf("status code range endpoints are reversed: %s", tag)
	}
	if lo < StatusCodeMin || hi > StatusCodeMax {
		return 0, 0, fmt.Errorf("status code must be between %d and %d: %s", StatusCodeMin, StatusCodeMax, tag)
	}
	return lo, hi, nil
}

// ExpandStatusCodeTags expands status code tags into a sorted unique list of integers.
func ExpandStatusCodeTags(tags []string) ([]int, error) {
	set := map[int]struct{}{}
	for _, tag := range tags {
		lo, hi, err := ParseStatusCodeTag(tag)
		if err != nil {
			return nil, err
		}
		for c := lo; c <= hi; c++ {
			set[c] = struct{}{}
		}
	}
	out := make([]int, 0, len(set))
	for c := range set {
		out = append(out, c)
	}
	sort.Ints(out)
	return out, nil
}
