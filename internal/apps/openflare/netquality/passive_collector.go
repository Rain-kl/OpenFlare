// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package netquality aggregates passive edge TCP_INFO latency metrics and active
// traceroute/ping probes into OpenFlare's node-to-network quality matrix.
package netquality

import (
	"context"
	"math"
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
	pkgnetintel "github.com/Rain-kl/Wavelet/pkg/netintel"
)

const (
	passiveFlushInterval       = 60 * time.Second
	maxPassiveBuckets          = 8192
	passiveHighWatermark       = maxPassiveBuckets * 4 / 5
	maxValidTCPRTTUs           = 60_000_000 // 60s upper sanity bound
	passiveFlushTimeout        = 10 * time.Second
	nodeASNCacheRefresh        = 60 * time.Second
	maxCountryCodeLen          = 16
	maxRegionCodeLen           = 32
	percentileMedian           = 0.50
	percentileP95              = 0.95
	round2Scale                = 100
	round4Scale                = 10000
	usPerMillisecond           = 1000.0
	minAccuracyWeightFloorKm   = 10.0
	minGeoSourceWeightFallback = 500.0
	defaultCityGeoAccKm        = 25.0
	collectorWarnCooldownSec   = 60
)

// PassiveSampleInput carries enriched client metadata for one passive TCP_INFO sample.
type PassiveSampleInput struct {
	NodeID        string
	ClientIP      string
	ClientASN     uint32
	ClientCountry string
	ClientRegion  string
	IPFamily      string
	ClientLat     *float64
	ClientLon     *float64
	GeoAccuracyKm float64
	GeoSource     string
	TCPRTTUs      uint32
	TCPMinRTTUs   uint32
	TCPRetrans    uint32
	TCPSegsOut    uint32
	TLSHandshake  uint32
}

// PassiveCollectorStats exposes eviction and drop counters for observability and testing.
type PassiveCollectorStats struct {
	ActiveBuckets      int
	EvictedColdBuckets uint64
	DroppedSamples     uint64
	EarlyFlushes       uint64
}

type passiveBucketKey struct {
	nodeID        string
	clientASN     uint32
	clientCountry string
	clientRegion  string
	ipFamily      string
}

type passiveBucket struct {
	sampleCount   int
	minRTTUs      uint32
	retransSum    uint64
	segsOutSum    uint64
	hist          LogHistogram64
	prefixes      PrefixSketch
	centroid      pkgnetintel.SphericalCentroid
	geoAccSum     float64
	geoAccWeight  float64
	geoSource     string
	capturedAt    time.Time
	lastUpdatedAt time.Time
}

type passiveCollector struct {
	mu                 sync.Mutex
	buckets            map[passiveBucketKey]*passiveBucket
	nodeASNsMu         sync.RWMutex
	nodeASNs           map[string]uint32
	nodeASNAt          time.Time
	startedOnce        sync.Once
	running            atomic.Bool
	flushInFlight      atomic.Bool
	flushWG            sync.WaitGroup
	evictedColdBuckets atomic.Uint64
	droppedSamples     atomic.Uint64
	earlyFlushes       atomic.Uint64
	lastWarnUnix       atomic.Int64
}

var defaultCollector = &passiveCollector{
	buckets:  make(map[passiveBucketKey]*passiveBucket),
	nodeASNs: make(map[string]uint32),
}

// ResetCollectorForTest clears buffered passive samples and collector counters.
func ResetCollectorForTest() {
	defaultCollector.flushWG.Wait()
	defaultCollector.mu.Lock()
	defaultCollector.buckets = make(map[passiveBucketKey]*passiveBucket)
	defaultCollector.mu.Unlock()
	defaultCollector.nodeASNsMu.Lock()
	defaultCollector.nodeASNs = make(map[string]uint32)
	defaultCollector.nodeASNAt = time.Time{}
	defaultCollector.nodeASNsMu.Unlock()
	defaultCollector.evictedColdBuckets.Store(0)
	defaultCollector.droppedSamples.Store(0)
	defaultCollector.earlyFlushes.Store(0)
	defaultCollector.flushInFlight.Store(false)
	defaultCollector.lastWarnUnix.Store(0)
}

// GetPassiveCollectorStats returns current buffered bucket count and drop/eviction counters.
func GetPassiveCollectorStats() PassiveCollectorStats {
	defaultCollector.mu.Lock()
	active := len(defaultCollector.buckets)
	defaultCollector.mu.Unlock()
	return PassiveCollectorStats{
		ActiveBuckets:      active,
		EvictedColdBuckets: defaultCollector.evictedColdBuckets.Load(),
		DroppedSamples:     defaultCollector.droppedSamples.Load(),
		EarlyFlushes:       defaultCollector.earlyFlushes.Load(),
	}
}

// RecordPassiveSample ingests a single downstream TCP RTT measurement using compact parameters.
func RecordPassiveSample(
	ctx context.Context,
	nodeID string,
	clientASN uint32,
	clientCountry string,
	tcpRTTUs uint32,
	tcpMinRTTUs uint32,
	retrans uint32,
	segsOut uint32,
	tlsHandshake uint32,
) {
	RecordPassiveSampleInput(ctx, PassiveSampleInput{
		NodeID:        nodeID,
		ClientASN:     clientASN,
		ClientCountry: clientCountry,
		TCPRTTUs:      tcpRTTUs,
		TCPMinRTTUs:   tcpMinRTTUs,
		TCPRetrans:    retrans,
		TCPSegsOut:    segsOut,
		TLSHandshake:  tlsHandshake,
	})
}

// RecordPassiveSampleInput ingests a single downstream TCP RTT measurement into a bounded 1-minute bucket.
// This runs on the access-log ingestion path and is strictly non-blocking and panic-safe.
func RecordPassiveSampleInput(ctx context.Context, input PassiveSampleInput) {
	defer func() {
		if r := recover(); r != nil {
			logger.ErrorF(ctx, "[NetQuality] RecordPassiveSampleInput panic recovered: %v", r)
		}
	}()

	key, effectiveMinUs, ok := normalizePassiveBucketKey(input)
	if !ok {
		return
	}

	now := time.Now().UTC()
	bucketTime := now.Truncate(time.Minute)
	rttMs := float64(input.TCPRTTUs) / usPerMillisecond

	triggerAsyncFlush := false
	defaultCollector.mu.Lock()
	b, exists := defaultCollector.buckets[key]
	if !exists {
		b, triggerAsyncFlush = allocatePassiveBucketLocked(ctx, key, effectiveMinUs, bucketTime)
		if b == nil {
			defaultCollector.mu.Unlock()
			defaultCollector.droppedSamples.Add(1)
			warnCollectorSaturation(ctx)
			return
		}
	}

	applyInputToBucketLocked(b, input, effectiveMinUs, rttMs, now)
	defaultCollector.mu.Unlock()

	if triggerAsyncFlush {
		scheduleEarlyFlush(ctx)
	}
}

func normalizePassiveBucketKey(input PassiveSampleInput) (passiveBucketKey, uint32, bool) {
	trimmedNode := strings.TrimSpace(input.NodeID)
	if trimmedNode == "" || input.ClientASN == 0 || input.TCPRTTUs == 0 || input.TCPRTTUs > maxValidTCPRTTUs {
		return passiveBucketKey{}, 0, false
	}
	country := strings.ToUpper(strings.TrimSpace(input.ClientCountry))
	if len(country) > maxCountryCodeLen {
		country = country[:maxCountryCodeLen]
	}
	region := pkgnetintel.FormatSubdivisionCode(country, input.ClientRegion)
	if len(region) > maxRegionCodeLen {
		region = region[:maxRegionCodeLen]
	}
	ipFamily := strings.ToLower(strings.TrimSpace(input.IPFamily))
	if ipFamily == "" {
		ipFamily = pkgnetintel.DetectIPFamily(input.ClientIP)
	}
	if ipFamily == "" {
		ipFamily = pkgnetintel.IPFamilyUnknown
	}
	effectiveMinUs := input.TCPRTTUs
	if input.TCPMinRTTUs > 0 && input.TCPMinRTTUs < effectiveMinUs {
		effectiveMinUs = input.TCPMinRTTUs
	}
	return passiveBucketKey{
		nodeID:        trimmedNode,
		clientASN:     input.ClientASN,
		clientCountry: country,
		clientRegion:  region,
		ipFamily:      ipFamily,
	}, effectiveMinUs, true
}

func allocatePassiveBucketLocked(ctx context.Context, key passiveBucketKey, effectiveMinUs uint32, bucketTime time.Time) (*passiveBucket, bool) {
	_ = ctx
	if len(defaultCollector.buckets) >= maxPassiveBuckets {
		if !evictOldestColdBucketLocked(defaultCollector.buckets) {
			return nil, false
		}
		defaultCollector.evictedColdBuckets.Add(1)
	}
	b := &passiveBucket{
		minRTTUs:   effectiveMinUs,
		capturedAt: bucketTime,
	}
	defaultCollector.buckets[key] = b
	return b, len(defaultCollector.buckets) >= passiveHighWatermark
}

func applyInputToBucketLocked(b *passiveBucket, input PassiveSampleInput, effectiveMinUs uint32, rttMs float64, now time.Time) {
	b.sampleCount++
	b.lastUpdatedAt = now
	if effectiveMinUs < b.minRTTUs || b.minRTTUs == 0 {
		b.minRTTUs = effectiveMinUs
	}
	b.retransSum += uint64(input.TCPRetrans)
	b.segsOutSum += uint64(input.TCPSegsOut)
	b.hist.Add(rttMs)
	if strings.TrimSpace(input.ClientIP) != "" {
		b.prefixes.AddIP(input.ClientIP)
	}
	if input.ClientLat != nil && input.ClientLon != nil {
		accKm := normalizeClientGeoAccuracy(input.GeoAccuracyKm, input.GeoSource)
		w := 1.0 / math.Max(minAccuracyWeightFloorKm, accKm)
		b.centroid.Add(*input.ClientLat, *input.ClientLon, w)
		b.geoAccSum += accKm * w
		b.geoAccWeight += w
	}
	if b.geoSource == "" || (b.geoSource != pkgnetintel.GeoSourceCity && input.GeoSource == pkgnetintel.GeoSourceCity) {
		b.geoSource = strings.TrimSpace(input.GeoSource)
	}
}

func normalizeClientGeoAccuracy(accKm float64, geoSource string) float64 {
	if accKm > 0 {
		return accKm
	}
	if strings.EqualFold(geoSource, pkgnetintel.GeoSourceCity) {
		return defaultCityGeoAccKm
	}
	return minGeoSourceWeightFallback
}

func evictOldestColdBucketLocked(buckets map[passiveBucketKey]*passiveBucket) bool {
	var oldestKey passiveBucketKey
	var oldestTime time.Time
	found := false
	for k, b := range buckets {
		if b == nil || b.sampleCount > 1 {
			continue
		}
		ts := b.lastUpdatedAt
		if ts.IsZero() {
			ts = b.capturedAt
		}
		if !found || ts.Before(oldestTime) {
			oldestKey = k
			oldestTime = ts
			found = true
		}
	}
	if !found {
		return false
	}
	delete(buckets, oldestKey)
	return true
}

func scheduleEarlyFlush(ctx context.Context) {
	if !defaultCollector.flushInFlight.CompareAndSwap(false, true) {
		return
	}
	defaultCollector.earlyFlushes.Add(1)
	defaultCollector.flushWG.Add(1)
	go func() {
		defer defaultCollector.flushWG.Done()
		defer defaultCollector.flushInFlight.Store(false)
		flushCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), passiveFlushTimeout)
		defer cancel()
		if err := FlushNow(flushCtx); err != nil {
			logger.WarnF(flushCtx, "[NetQuality] early high-watermark passive flush failed: %v", err)
		}
	}()
}

func warnCollectorSaturation(ctx context.Context) {
	nowUnix := time.Now().Unix()
	prev := defaultCollector.lastWarnUnix.Load()
	if nowUnix-prev < collectorWarnCooldownSec {
		return
	}
	if defaultCollector.lastWarnUnix.CompareAndSwap(prev, nowUnix) {
		logger.WarnF(ctx, "[NetQuality] passive bucket capacity (%d) saturated; evicted=%d dropped=%d",
			maxPassiveBuckets, defaultCollector.evictedColdBuckets.Load(), defaultCollector.droppedSamples.Load())
	}
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
		if bucket == nil || bucket.sampleCount <= 0 || bucket.hist.Count == 0 {
			continue
		}
		p50 := bucket.hist.Quantile(percentileMedian)
		p95 := bucket.hist.Quantile(percentileP95)
		rttMin := float64(bucket.minRTTUs) / usPerMillisecond
		if rttMin <= 0 {
			rttMin = p50
		}

		denom := max(bucket.segsOutSum, uint64(uint32(bucket.sampleCount))) //nolint:gosec // sampleCount > 0 checked above
		retransRate := math.Min(1.0, float64(bucket.retransSum)/float64(denom))

		var clientLat, clientLon *float64
		if lat, lon, ok := bucket.centroid.Result(); ok {
			la := round4(lat)
			lo := round4(lon)
			clientLat = &la
			clientLon = &lo
		}
		var geoAccuracyKm float64
		if bucket.geoAccWeight > 0 {
			geoAccuracyKm = round2(bucket.geoAccSum / bucket.geoAccWeight)
		}

		nodeASN := nodeASNs[key.nodeID]
		capturedAt := bucket.capturedAt
		if capturedAt.IsZero() {
			capturedAt = now
		}

		record := &model.OpenFlareNetworkQualitySample{
			NodeID:              key.nodeID,
			NodeASN:             nodeASN,
			ClientASN:           key.clientASN,
			ClientCountry:       key.clientCountry,
			ClientRegion:        key.clientRegion,
			IPFamily:            key.ipFamily,
			StatsVersion:        model.NetworkQualityStatsVersionHist64,
			Source:              model.NetworkQualitySourceTCPPassive,
			SampleCount:         bucket.sampleCount,
			RTTMinMs:            round2(rttMin),
			RTTP50Ms:            round2(p50),
			RTTP95Ms:            round2(p95),
			RTTHist:             bucket.hist.Encode(),
			PrefixSketch:        bucket.prefixes.Encode(),
			DistinctPrefixCount: bucket.prefixes.DistinctCount(),
			ClientLat:           clientLat,
			ClientLon:           clientLon,
			GeoAccuracyKm:       geoAccuracyKm,
			GeoSource:           bucket.geoSource,
			RetransRate:         round4(retransRate),
			LossRate:            0,
			HopCount:            0,
			CapturedAt:          capturedAt,
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

func round2(v float64) float64 {
	return math.Round(v*round2Scale) / round2Scale
}

func round4(v float64) float64 {
	return math.Round(v*round4Scale) / round4Scale
}
