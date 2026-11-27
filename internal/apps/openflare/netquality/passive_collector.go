// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package netquality aggregates passive edge TCP_INFO latency metrics and active
// traceroute/ping probes into OpenFlare's node-to-ASN network quality matrix.
package netquality

import (
	"context"
	"math"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/netintel"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/platform/lifecycle"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/pkg/logger"
)

const (
	passiveFlushInterval   = 60 * time.Second
	maxPassiveBuckets      = 4096
	maxBucketReservoir     = 256
	maxValidTCPRTTUs       = 60_000_000 // 60s upper sanity bound
	passiveFlushTimeout    = 10 * time.Second
	nodeASNCacheRefresh    = 60 * time.Second
	maxCountryCodeLen      = 16
	initialBucketSampleCap = 16
	percentileMedian       = 0.50
	percentileP95          = 0.95
	round2Scale            = 100
	round4Scale            = 10000
	usPerMillisecond       = 1000.0
)

type passiveBucketKey struct {
	nodeID        string
	clientASN     uint32
	clientCountry string
}

type passiveBucket struct {
	sampleCount int
	minRTTUs    uint32
	retransSum  uint64
	segsOutSum  uint64
	rttMsSample []float64
	rngState    uint64
	capturedAt  time.Time
}

type passiveCollector struct {
	mu          sync.Mutex
	buckets     map[passiveBucketKey]*passiveBucket
	nodeASNsMu  sync.RWMutex
	nodeASNs    map[string]uint32
	nodeASNAt   time.Time
	startedOnce sync.Once
	running     atomic.Bool
}

var defaultCollector = &passiveCollector{
	buckets:  make(map[passiveBucketKey]*passiveBucket),
	nodeASNs: make(map[string]uint32),
}

// ResetCollectorForTest clears buffered passive samples (intended for unit tests).
func ResetCollectorForTest() {
	defaultCollector.mu.Lock()
	defaultCollector.buckets = make(map[passiveBucketKey]*passiveBucket)
	defaultCollector.mu.Unlock()
	defaultCollector.nodeASNsMu.Lock()
	defaultCollector.nodeASNs = make(map[string]uint32)
	defaultCollector.nodeASNAt = time.Time{}
	defaultCollector.nodeASNsMu.Unlock()
}

// RecordPassiveSample ingests a single downstream TCP RTT measurement into a bounded 1-minute bucket.
// This runs on the access-log ingestion path and is strictly non-blocking and panic-safe.
func RecordPassiveSample(
	ctx context.Context,
	nodeID string,
	clientASN uint32,
	clientCountry string,
	tcpRTTUs uint32,
	tcpMinRTTUs uint32,
	retrans uint32,
	segsOut uint32,
	_ uint32,
) {
	defer func() {
		if r := recover(); r != nil {
			logger.ErrorF(ctx, "[NetQuality] RecordPassiveSample panic recovered: %v", r)
		}
	}()

	trimmedNode := strings.TrimSpace(nodeID)
	if trimmedNode == "" || clientASN == 0 || tcpRTTUs == 0 || tcpRTTUs > maxValidTCPRTTUs {
		return
	}
	country := strings.ToUpper(strings.TrimSpace(clientCountry))
	if len(country) > maxCountryCodeLen {
		country = country[:maxCountryCodeLen]
	}

	effectiveMinUs := tcpRTTUs
	if tcpMinRTTUs > 0 && tcpMinRTTUs < effectiveMinUs {
		effectiveMinUs = tcpMinRTTUs
	}

	key := passiveBucketKey{
		nodeID:        trimmedNode,
		clientASN:     clientASN,
		clientCountry: country,
	}

	now := time.Now().UTC().Truncate(time.Minute)
	rttMs := float64(tcpRTTUs) / usPerMillisecond

	defaultCollector.mu.Lock()
	b, ok := defaultCollector.buckets[key]
	if !ok {
		if len(defaultCollector.buckets) >= maxPassiveBuckets {
			defaultCollector.mu.Unlock()
			return
		}
		b = &passiveBucket{
			minRTTUs:    effectiveMinUs,
			rttMsSample: make([]float64, 0, initialBucketSampleCap),
			rngState:    uint64(clientASN) ^ uint64(tcpRTTUs) ^ 0x9e3779b97f4a7c15,
			capturedAt:  now,
		}
		defaultCollector.buckets[key] = b
	}

	b.sampleCount++
	if effectiveMinUs < b.minRTTUs || b.minRTTUs == 0 {
		b.minRTTUs = effectiveMinUs
	}
	b.retransSum += uint64(retrans)
	b.segsOutSum += uint64(segsOut)

	if len(b.rttMsSample) < maxBucketReservoir {
		b.rttMsSample = append(b.rttMsSample, rttMs)
	} else {
		// SplitMix64 reservoir replacement keeps memory bounded per (node, asn, country) bucket.
		b.rngState += 0x9e3779b97f4a7c15
		z := b.rngState
		z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
		z = (z ^ (z >> 27)) * 0x94d049bb133111eb
		z ^= z >> 31
		slot := int(z % uint64(uint32(b.sampleCount))) //nolint:gosec // sampleCount >= 1 here
		if slot < maxBucketReservoir {
			b.rttMsSample[slot] = rttMs
		}
	}
	defaultCollector.mu.Unlock()
}

// StartPassiveCollector starts the background 60-second bucket flusher and registers clean shutdown draining.
func StartPassiveCollector(ctx context.Context) {
	defaultCollector.startedOnce.Do(func() {
		collectorCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
		defaultCollector.running.Store(true)

		stopped := make(chan struct{})
		lifecycle.OnShutdown("network quality passive collector", func(shutdownCtx context.Context) error {
			cancel()
			select {
			case <-stopped:
			case <-shutdownCtx.Done():
			}
			flushCtx, flushCancel := context.WithTimeout(context.WithoutCancel(shutdownCtx), passiveFlushTimeout)
			defer flushCancel()
			return FlushNow(flushCtx)
		})

		go func() {
			defer close(stopped)
			defer defaultCollector.running.Store(false)
			defer func() {
				if r := recover(); r != nil {
					logger.ErrorF(collectorCtx, "[NetQuality] passive collector panic recovered: %v", r)
				}
			}()

			ticker := time.NewTicker(passiveFlushInterval)
			defer ticker.Stop()

			for {
				select {
				case <-collectorCtx.Done():
					return
				case <-ticker.C:
					flushCtx, flushCancel := context.WithTimeout(collectorCtx, passiveFlushTimeout)
					if err := FlushNow(flushCtx); err != nil {
						logger.WarnF(collectorCtx, "[NetQuality] flush passive quality buckets failed: %v", err)
					}
					flushCancel()
				}
			}
		}()
	})
}

// FlushNow drains all active 1-minute passive buckets and persists aggregated samples to logstore.
func FlushNow(ctx context.Context) error {
	defaultCollector.mu.Lock()
	if len(defaultCollector.buckets) == 0 {
		defaultCollector.mu.Unlock()
		return nil
	}
	drained := defaultCollector.buckets
	defaultCollector.buckets = make(map[passiveBucketKey]*passiveBucket)
	defaultCollector.mu.Unlock()

	if db.DB(ctx) == nil {
		return nil
	}
	nodeASNs := lookupCachedNodeASNs(ctx)

	now := time.Now().UTC()
	var firstErr error
	for key, bucket := range drained {
		if bucket == nil || bucket.sampleCount <= 0 || len(bucket.rttMsSample) == 0 {
			continue
		}
		sort.Float64s(bucket.rttMsSample)
		p50 := percentileSorted(bucket.rttMsSample, percentileMedian)
		p95 := percentileSorted(bucket.rttMsSample, percentileP95)
		rttMin := float64(bucket.minRTTUs) / usPerMillisecond
		if rttMin <= 0 || rttMin > bucket.rttMsSample[0] {
			rttMin = bucket.rttMsSample[0]
		}

		denom := max(bucket.segsOutSum, uint64(uint32(bucket.sampleCount))) //nolint:gosec // sampleCount > 0 checked above
		retransRate := math.Min(1.0, float64(bucket.retransSum)/float64(denom))

		nodeASN := nodeASNs[key.nodeID]
		capturedAt := bucket.capturedAt
		if capturedAt.IsZero() {
			capturedAt = now
		}

		record := &model.OpenFlareNetworkQualitySample{
			NodeID:        key.nodeID,
			NodeASN:       nodeASN,
			ClientASN:     key.clientASN,
			ClientCountry: key.clientCountry,
			Source:        model.NetworkQualitySourceTCPPassive,
			SampleCount:   bucket.sampleCount,
			RTTMinMs:      round2(rttMin),
			RTTP50Ms:      round2(p50),
			RTTP95Ms:      round2(p95),
			RetransRate:   round4(retransRate),
			LossRate:      0,
			HopCount:      0,
			CapturedAt:    capturedAt,
		}
		if err := repository.InsertOpenFlareNetworkQualitySample(ctx, record); err != nil && firstErr == nil {
			firstErr = err
			logger.WarnF(ctx, "[NetQuality] insert passive sample failed (node=%s asn=%d): %v", key.nodeID, key.clientASN, err)
		}
	}
	return firstErr
}

func lookupCachedNodeASNs(ctx context.Context) map[string]uint32 {
	defaultCollector.nodeASNsMu.RLock()
	fresh := time.Since(defaultCollector.nodeASNAt) < nodeASNCacheRefresh && len(defaultCollector.nodeASNs) > 0
	cached := defaultCollector.nodeASNs
	defaultCollector.nodeASNsMu.RUnlock()
	if fresh {
		return cached
	}

	nodes, err := repository.ListOpenFlareNodes(ctx)
	if err != nil {
		return cached
	}
	refreshed := make(map[string]uint32, len(nodes))
	for i := range nodes {
		n := nodes[i]
		if n.ASN == 0 && strings.TrimSpace(n.IP) != "" {
			if res := netintel.LookupIP(n.IP); res != nil && res.ASN > 0 {
				n.ASN = res.ASN
			}
		}
		if n.ASN > 0 {
			refreshed[n.NodeID] = n.ASN
		}
	}
	defaultCollector.nodeASNsMu.Lock()
	defaultCollector.nodeASNs = refreshed
	defaultCollector.nodeASNAt = time.Now()
	defaultCollector.nodeASNsMu.Unlock()
	return refreshed
}

func percentileSorted(sorted []float64, q float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	idx := q * float64(len(sorted)-1)
	lo := int(math.Floor(idx))
	hi := int(math.Ceil(idx))
	if lo == hi {
		return sorted[lo]
	}
	frac := idx - float64(lo)
	return sorted[lo]*(1-frac) + sorted[hi]*frac
}

func round2(v float64) float64 {
	return math.Round(v*round2Scale) / round2Scale
}

func round4(v float64) float64 {
	return math.Round(v*round4Scale) / round4Scale
}
