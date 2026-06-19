// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package idgen 提供分布式 ID 生成器
package idgen

import (
	"fmt"
	"log"

	"github.com/Rain-kl/Wavelet/internal/config"
	"github.com/bwmarrin/snowflake"
)

// 2025-12-01 00:00:00 UTC 的毫秒时间戳
const epoch int64 = 1764547200000

const maxNegativeIDRetries = 3

var node *snowflake.Node

func init() {
	snowflake.Epoch = epoch

	nodeID := config.Config.App.NodeID
	var err error
	node, err = snowflake.NewNode(nodeID)
	if err != nil {
		log.Fatalf("[Snowflake] init failed: %v\n", err)
	}
	log.Printf("[Snowflake] initialized with node ID: %d, epoch: 2025-12-01\n", nodeID)
}

// NextUint64ID 生成下一个分布式唯一 ID。
// 理论上不应出现负值；若出现则最多重试 maxNegativeIDRetries 次，仍失败则 panic。
func NextUint64ID() uint64 {
	for attempt := 1; attempt <= maxNegativeIDRetries; attempt++ {
		id := node.Generate().Int64()
		if id >= 0 {
			return uint64(id)
		}
		log.Printf("[Snowflake] generated negative ID: %d (attempt %d/%d)", id, attempt, maxNegativeIDRetries)
	}
	panic(fmt.Sprintf("[Snowflake] generated negative ID after %d attempts", maxNegativeIDRetries))
}
