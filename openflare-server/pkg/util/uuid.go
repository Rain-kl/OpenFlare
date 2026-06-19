// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io"

	"github.com/google/uuid"
)

// uniqueIDBytes 生成唯一 ID 所需的随机字节长度
const uniqueIDBytes = 32

// GenerateUniqueIDSimple 生成 64 位唯一标识符
func GenerateUniqueIDSimple() string {
	randomBytes := make([]byte, uniqueIDBytes)
	if _, err := io.ReadFull(rand.Reader, randomBytes); err != nil {
		// 如果随机数生成失败，使用 UUID 作为后备
		uuidBytes := []byte(uuid.NewString())
		hash := sha256.Sum256(uuidBytes)
		copy(randomBytes, hash[:])
	}
	return hex.EncodeToString(randomBytes)
}
