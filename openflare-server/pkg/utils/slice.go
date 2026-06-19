package utils

import (
	"sort"
	"strings"
	"time"
)

// Unique returns a new slice containing only the unique elements of the input slice,
// preserving their original order.
func Unique[T comparable](slice []T) []T {
	if slice == nil {
		return nil
	}
	seen := make(map[T]struct{})
	result := make([]T, 0)
	for _, item := range slice {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}

// UniqueAndCleanStringSlice trims spaces, removes empty elements, and returns only the unique elements
// of the input string slice. It preserves order and returns nil if the resulting slice is empty.
func UniqueAndCleanStringSlice(slice []string) []string {
	if slice == nil {
		return nil
	}
	seen := make(map[string]struct{})
	result := make([]string, 0)
	for _, item := range slice {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// IdentifiableTimeRecord represents a database record that has a unique ID and a primary timestamp field.
type IdentifiableTimeRecord interface {
	GetID() uint
	GetTime() time.Time
}

// SortAndLimitRecords sorts a slice of IdentifiableTimeRecord descendingly by their timestamp (and ID as a tie-breaker),
// and limits the slice to the specified size if limit > 0.
func SortAndLimitRecords[T IdentifiableTimeRecord](rows []T, limit int) []T {
	if len(rows) == 0 {
		return rows
	}
	sort.Slice(rows, func(i, j int) bool {
		ti := rows[i].GetTime()
		tj := rows[j].GetTime()
		if ti.Equal(tj) {
			return rows[i].GetID() > rows[j].GetID()
		}
		return ti.After(tj)
	})
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	return rows
}
