package util

import "strconv"

func compareVersionNumbers(left, right VersionInfo) int {
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
	return 0
}

func compareGitDescribeDistance(left, right VersionInfo) int {
	if left.GitDescribeDistance == right.GitDescribeDistance {
		return 0
	}
	if left.GitDescribeDistance < right.GitDescribeDistance {
		return -1
	}
	return 1
}

func compareGitDescribeTails(left, right VersionInfo) int {
	maxLen := len(left.GitDescribeTail)
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

func comparePrereleaseIdentifiers(left, right VersionInfo) int {
	if len(left.Prerelease) == 0 && len(right.Prerelease) == 0 {
		return 0
	}
	if len(left.Prerelease) == 0 {
		return 1
	}
	if len(right.Prerelease) == 0 {
		return -1
	}

	maxLen := len(left.Prerelease)
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
		if result := comparePrereleasePart(left.Prerelease[index], right.Prerelease[index]); result != 0 {
			return result
		}
	}
	return 0
}

func comparePrereleasePart(leftPart, rightPart string) int {
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
	return 0
}
