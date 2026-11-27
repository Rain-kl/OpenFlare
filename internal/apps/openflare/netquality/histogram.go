// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package netquality

import (
	"encoding/base64"
	"encoding/binary"
	"errors"
	"math"
	"strings"
)

const (
	// LogHistogramVersion identifies the fixed 64-bin logarithmic RTT histogram schema.
	LogHistogramVersion = 1
	// LogHistogramBuckets is the total bucket count (2 overflow bins + 62 log-uniform bins).
	LogHistogramBuckets = 64
	// LogHistogramMinMs is the lower boundary of the normal logarithmic range.
	LogHistogramMinMs = 0.5
	// LogHistogramMaxMs is the upper boundary of the normal logarithmic range.
	LogHistogramMaxMs = 10000.0
	// LogHistogramNormalBins is the number of geometric bins covering [0.5ms, 10000ms].
	LogHistogramNormalBins = 62

	logHistPrefix          = "v1:"
	logHistUnderflowBin    = 0
	logHistOverflowBin     = LogHistogramBuckets - 1
	logHistUnderflowMidMs  = 0.25
	logHistOverflowMidMs   = 10000.0
	logHistMaxSerializedSz = 2048
	logHistInitialBufCap   = 64
)

var (
	histLogRatio   float64
	histLowerEdges [LogHistogramBuckets]float64
	histUpperEdges [LogHistogramBuckets]float64
	histMidpoints  [LogHistogramBuckets]float64
	errInvalidHist = errors.New("invalid log histogram encoding")
)

func init() {
	histLogRatio = math.Log(LogHistogramMaxMs/LogHistogramMinMs) / float64(LogHistogramNormalBins)
	ratio := math.Exp(histLogRatio)

	histLowerEdges[logHistUnderflowBin] = 0
	histUpperEdges[logHistUnderflowBin] = LogHistogramMinMs
	histMidpoints[logHistUnderflowBin] = logHistUnderflowMidMs

	edge := LogHistogramMinMs
	for i := 1; i <= LogHistogramNormalBins; i++ {
		next := edge * ratio
		if i == LogHistogramNormalBins {
			next = LogHistogramMaxMs
		}
		histLowerEdges[i] = edge
		histUpperEdges[i] = next
		// Harmonic-geometric representative minimizes worst-case relative quantile error: (r-1)/(r+1) ≈ 7.97%.
		histMidpoints[i] = (2.0 * edge * next) / (edge + next)
		edge = next
	}

	histLowerEdges[logHistOverflowBin] = LogHistogramMaxMs
	histUpperEdges[logHistOverflowBin] = math.Inf(1)
	histMidpoints[logHistOverflowBin] = logHistOverflowMidMs
}

// LogHistogram64 is a fixed 64-bin logarithmic RTT histogram (256 B counters).
type LogHistogram64 struct {
	Bins  [LogHistogramBuckets]uint32
	Count uint64
}

// Add records a single RTT observation in milliseconds into its logarithmic bin.
func (h *LogHistogram64) Add(rttMs float64) {
	if h == nil || rttMs <= 0 || math.IsNaN(rttMs) || math.IsInf(rttMs, 0) {
		return
	}
	idx := logHistogramBinIndex(rttMs)
	h.Bins[idx]++
	h.Count++
}

// Merge combines another LogHistogram64 into h.
func (h *LogHistogram64) Merge(other *LogHistogram64) {
	if h == nil || other == nil || other.Count == 0 {
		return
	}
	for i := range LogHistogramBuckets {
		if other.Bins[i] > 0 {
			h.Bins[i] += other.Bins[i]
			h.Count += uint64(other.Bins[i])
		}
	}
}

// Quantile estimates the q-th quantile in [0, 1] using bin representative midpoints.
func (h *LogHistogram64) Quantile(q float64) float64 {
	if h == nil || h.Count == 0 {
		return 0
	}
	var w WeightedLogHistogram64
	w.AddHistogram(h, 1.0)
	return w.Quantile(q)
}

// Encode serializes non-zero histogram bins into a compact versioned base64 string.
func (h *LogHistogram64) Encode() string {
	if h == nil || h.Count == 0 {
		return ""
	}
	buf := make([]byte, 0, logHistInitialBufCap)
	var tmp [binary.MaxVarintLen64]byte
	for i := range LogHistogramBuckets {
		c := h.Bins[i]
		if c == 0 {
			continue
		}
		buf = append(buf, byte(i)) //nolint:gosec // i is bounded 0..63
		n := binary.PutUvarint(tmp[:], uint64(c))
		buf = append(buf, tmp[:n]...)
	}
	if len(buf) == 0 {
		return ""
	}
	return logHistPrefix + base64.RawStdEncoding.EncodeToString(buf)
}

// DecodeLogHistogram64 parses a serialized LogHistogram64 string.
func DecodeLogHistogram64(encoded string) (*LogHistogram64, error) {
	trimmed := strings.TrimSpace(encoded)
	if trimmed == "" {
		return &LogHistogram64{}, nil
	}
	if !strings.HasPrefix(trimmed, logHistPrefix) || len(trimmed) > logHistMaxSerializedSz {
		return nil, errInvalidHist
	}
	raw, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(trimmed, logHistPrefix))
	if err != nil {
		return nil, err
	}
	hist := &LogHistogram64{}
	pos := 0
	for pos < len(raw) {
		idx := int(raw[pos])
		pos++
		if idx < 0 || idx >= LogHistogramBuckets {
			return nil, errInvalidHist
		}
		val, n := binary.Uvarint(raw[pos:])
		if n <= 0 || val == 0 || val > math.MaxUint32 {
			return nil, errInvalidHist
		}
		pos += n
		hist.Bins[idx] += uint32(val)
		hist.Count += val
	}
	return hist, nil
}

// WeightedLogHistogram64 accumulates time-decayed 64-bin distributions during Rollup.
type WeightedLogHistogram64 struct {
	Bins        [LogHistogramBuckets]float64
	TotalWeight float64
}

// AddHistogram adds all counts from h scaled by weight into w.
func (w *WeightedLogHistogram64) AddHistogram(h *LogHistogram64, weight float64) {
	if w == nil || h == nil || h.Count == 0 || weight <= 0 || math.IsNaN(weight) || math.IsInf(weight, 0) {
		return
	}
	for i := range LogHistogramBuckets {
		if h.Bins[i] > 0 {
			delta := float64(h.Bins[i]) * weight
			w.Bins[i] += delta
			w.TotalWeight += delta
		}
	}
}

// AddPoint records a single point observation with weight into w (useful for single-probe samples).
func (w *WeightedLogHistogram64) AddPoint(rttMs, weight float64) {
	if w == nil || rttMs <= 0 || weight <= 0 || math.IsNaN(rttMs) || math.IsNaN(weight) {
		return
	}
	idx := logHistogramBinIndex(rttMs)
	w.Bins[idx] += weight
	w.TotalWeight += weight
}

// Quantile estimates the q-th quantile in [0, 1] from accumulated weighted counts.
func (w *WeightedLogHistogram64) Quantile(q float64) float64 {
	if w == nil || w.TotalWeight <= 0 {
		return 0
	}
	if q <= 0 {
		for i := range LogHistogramBuckets {
			if w.Bins[i] > 0 {
				return histMidpoints[i]
			}
		}
		return 0
	}
	if q >= 1 {
		for i := LogHistogramBuckets - 1; i >= 0; i-- {
			if w.Bins[i] > 0 {
				return histMidpoints[i]
			}
		}
		return 0
	}
	target := q * w.TotalWeight
	var cum float64
	lastNonZero := 0
	for i := range LogHistogramBuckets {
		if w.Bins[i] <= 0 {
			continue
		}
		lastNonZero = i
		cum += w.Bins[i]
		if cum >= target {
			return histMidpoints[i]
		}
	}
	return histMidpoints[lastNonZero]
}

func logHistogramBinIndex(rttMs float64) int {
	if rttMs < LogHistogramMinMs {
		return logHistUnderflowBin
	}
	if rttMs >= LogHistogramMaxMs {
		return logHistOverflowBin
	}
	idx := 1 + int(math.Floor(math.Log(rttMs/LogHistogramMinMs)/histLogRatio))
	if idx < 1 {
		idx = 1
	}
	if idx > LogHistogramNormalBins {
		idx = LogHistogramNormalBins
	}
	for idx < LogHistogramNormalBins && rttMs >= histUpperEdges[idx] {
		idx++
	}
	for idx > 1 && rttMs < histLowerEdges[idx] {
		idx--
	}
	return idx
}
