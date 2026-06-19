// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package util provides framework-agnostic helper types and HTTP utilities.
package util

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// StringArray custom type for handling JSON arrays
type StringArray []string

// Scan 实现 sql.Scanner 接口，从数据库读取 JSON 数组
func (sa *StringArray) Scan(value interface{}) error {
	bytesValue, ok := value.([]byte)
	if !ok {
		return fmt.Errorf(errInvalidCustomValue, value)
	}
	return json.Unmarshal(bytesValue, sa)
}

// Value 实现 driver.Valuer 接口，将 JSON 数组序列化为数据库存储值
func (sa StringArray) Value() (driver.Value, error) {
	return json.Marshal(sa)
}
