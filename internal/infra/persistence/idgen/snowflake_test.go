// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package idgen

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNextUint64ID(t *testing.T) {
	id := NextUint64ID()
	assert.NotZero(t, id)
}
