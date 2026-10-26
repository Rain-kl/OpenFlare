// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package agent

import "time"

const (
	discoveryRetryInitialDelay = 3 * time.Second
	discoveryRetryMaxDelay     = time.Minute
	discoveryRetryMultiplier   = 2
)

type discoveryRetry struct {
	delay time.Duration
	next  time.Time
}

func (r *discoveryRetry) ready(now time.Time) bool {
	return !now.Before(r.next)
}

func (r *discoveryRetry) failed(now time.Time) {
	if r.delay == 0 {
		r.delay = discoveryRetryInitialDelay
	} else {
		r.delay = min(r.delay*discoveryRetryMultiplier, discoveryRetryMaxDelay)
	}
	r.next = now.Add(r.delay)
}
