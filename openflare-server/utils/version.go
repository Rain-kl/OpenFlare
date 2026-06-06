package utils

import (
	"strconv"
	"strings"
)

type VersionInfo struct {
	Valid               bool
	IsDev               bool
	Numbers             []int
	Prerelease          []string
	GitDescribeDistance int
	GitDescribeTail     []string
}

func ParseVersionInfo(version string) VersionInfo {
	normalized := strings.TrimSpace(strings.TrimPrefix(version, "v"))
	if normalized == "" || normalized == "dev" {
		return VersionInfo{IsDev: strings.EqualFold(normalized, "dev")}
	}
	base := normalized
	prerelease := ""
	if separator := strings.IndexRune(normalized, '-'); separator >= 0 {
		base = normalized[:separator]
		prerelease = normalized[separator+1:]
	}

	segments := strings.Split(base, ".")
	parts := make([]int, 0, len(segments))
	for _, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			parts = append(parts, 0)
			continue
		}

		numeric := strings.Builder{}
		for _, r := range segment {
			if r < '0' || r > '9' {
				break
			}
			numeric.WriteRune(r)
		}
		if numeric.Len() == 0 {
			parts = append(parts, 0)
			continue
		}
		value, err := strconv.Atoi(numeric.String())
		if err != nil {
			return VersionInfo{}
		}
		parts = append(parts, value)
	}
	info := VersionInfo{Valid: len(parts) > 0, Numbers: parts}
	if prerelease != "" {
		identifiers := splitPrereleaseIdentifiers(prerelease)
		if distance, tail, ok := parseGitDescribeIdentifiers(identifiers); ok {
			info.GitDescribeDistance = distance
			info.GitDescribeTail = tail
		} else {
			info.Prerelease = identifiers
		}
	}
	return info
}

func parseGitDescribeIdentifiers(identifiers []string) (int, []string, bool) {
	if len(identifiers) < 2 {
		return 0, nil, false
	}
	distance, err := strconv.Atoi(strings.TrimSpace(identifiers[0]))
	if err != nil || distance <= 0 {
		return 0, nil, false
	}
	commitToken := strings.TrimSpace(identifiers[1])
	if commitToken == "" || !strings.HasPrefix(strings.ToLower(commitToken), "g") {
		return 0, nil, false
	}
	return distance, identifiers[1:], true
}

func splitPrereleaseIdentifiers(value string) []string {
	parts := strings.FieldsFunc(strings.TrimSpace(value), func(r rune) bool {
		return r == '.' || r == '-'
	})
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			filtered = append(filtered, part)
		}
	}
	return filtered
}

// CompareVersions compares two version strings.
// Returns -1 if left < right, 1 if left > right, and 0 if they are equal.
func CompareVersions(local, remote string) int {
	left := ParseVersionInfo(local)
	right := ParseVersionInfo(remote)
	if left.IsDev {
		if right.Valid {
			return -1
		}
		return 0
	}
	if !left.Valid || !right.Valid {
		return 0
	}

	maxLen := len(left.Numbers)
	if len(right.Numbers) > maxLen {
		maxLen = len(right.Numbers)
	}
	for index := 0; index < maxLen; index++ {
		leftValue := 0
		rightValue := 0
		if index < len(left.Numbers) {
			leftValue = left.Numbers[index]
		}
		if index < len(right.Numbers) {
			rightValue = right.Numbers[index]
		}
		if leftValue < rightValue {
			return -1
		}
		if leftValue > rightValue {
			return 1
		}
	}

	if left.GitDescribeDistance != right.GitDescribeDistance {
		if left.GitDescribeDistance < right.GitDescribeDistance {
			return -1
		}
		return 1
	}
	if left.GitDescribeDistance > 0 || right.GitDescribeDistance > 0 {
		maxLen = len(left.GitDescribeTail)
		if len(right.GitDescribeTail) > maxLen {
			maxLen = len(right.GitDescribeTail)
		}
		for index := 0; index < maxLen; index++ {
			if index >= len(left.GitDescribeTail) {
				return -1
			}
			if index >= len(right.GitDescribeTail) {
				return 1
			}
			if left.GitDescribeTail[index] < right.GitDescribeTail[index] {
				return -1
			}
			if left.GitDescribeTail[index] > right.GitDescribeTail[index] {
				return 1
			}
		}
		return 0
	}

	if len(left.Prerelease) == 0 && len(right.Prerelease) == 0 {
		return 0
	}
	if len(left.Prerelease) == 0 {
		return 1
	}
	if len(right.Prerelease) == 0 {
		return -1
	}

	maxLen = len(left.Prerelease)
	if len(right.Prerelease) > maxLen {
		maxLen = len(right.Prerelease)
	}
	for index := 0; index < maxLen; index++ {
		if index >= len(left.Prerelease) {
			return -1
		}
		if index >= len(right.Prerelease) {
			return 1
		}
		leftPart := left.Prerelease[index]
		rightPart := right.Prerelease[index]
		leftNumber, leftErr := strconv.Atoi(leftPart)
		rightNumber, rightErr := strconv.Atoi(rightPart)
		switch {
		case leftErr == nil && rightErr == nil:
			if leftNumber < rightNumber {
				return -1
			}
			if leftNumber > rightNumber {
				return 1
			}
		case leftErr == nil:
			return -1
		case rightErr == nil:
			return 1
		default:
			if leftPart < rightPart {
				return -1
			}
			if leftPart > rightPart {
				return 1
			}
		}
	}
	return 0
}
