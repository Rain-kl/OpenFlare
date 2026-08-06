package openresty

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const (
	StatusCodeMin = 400
	StatusCodeMax = 599
)

func ParseStatusCodeTag(tag string) (lo, hi int, err error) {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return 0, 0, fmt.Errorf("状态码标签不能为空")
	}
	if i := strings.IndexByte(tag, '-'); i >= 0 {
		lo, err = strconv.Atoi(tag[:i])
		if err != nil {
			return 0, 0, fmt.Errorf("无效状态码区间: %s", tag)
		}
		hi, err = strconv.Atoi(tag[i+1:])
		if err != nil {
			return 0, 0, fmt.Errorf("无效状态码区间: %s", tag)
		}
	} else {
		lo, err = strconv.Atoi(tag)
		if err != nil {
			return 0, 0, fmt.Errorf("无效状态码: %s", tag)
		}
		hi = lo
	}
	if lo > hi {
		return 0, 0, fmt.Errorf("状态码区间左右端点反序: %s", tag)
	}
	if lo < StatusCodeMin || hi > StatusCodeMax {
		return 0, 0, fmt.Errorf("状态码须在 %d–%d: %s", StatusCodeMin, StatusCodeMax, tag)
	}
	return lo, hi, nil
}

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
