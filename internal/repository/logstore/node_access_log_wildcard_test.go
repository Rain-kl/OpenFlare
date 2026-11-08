// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/model"
	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
	"github.com/google/go-cmp/cmp"
)

func TestGormNodeAccessLogWildcardHosts(t *testing.T) {
	ResetForTest()
	t.Cleanup(ResetForTest)
	SetConfigReader(func(context.Context, string) (string, error) { return "", nil })
	s := newTestGormStore(t)
	ctx := context.Background()
	base := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
	hosts := []string{
		"api.example.com", " A.B.Example.COM ", "bucket.s3.example.com", "deep.bucket.s3.example.com",
		"example.com", "s3.example.com", "evil-example.com", "api.example.com.evil", ".example.com",
		`x.a%_\'?.example.net`, `x.abc\'?.example.net`, ".s3.other.com", "s3.other.com",
	}
	rows := make([]analyticsmodel.NodeAccessLog, 0, len(hosts)+3)
	for i, host := range hosts {
		rows = append(rows, analyticsmodel.NodeAccessLog{
			ID: uint64(i + 1), NodeID: "n1", Host: host, LoggedAt: base.Add(time.Duration(i%2)*time.Hour + time.Minute),
			RemoteAddr: "1.1.1.1", BytesSent: uint64((i + 1) * 10), StatusCode: 200,
		})
	}
	rows[2].RemoteAddr = "2.2.2.2"
	rows[3].RemoteAddr = "" // 空 IP 仍计入请求与字节数，但不计入 UV。
	rows = append(rows,
		analyticsmodel.NodeAccessLog{ID: 100, NodeID: "other", Host: "api.example.com", LoggedAt: base.Add(time.Minute)},
		analyticsmodel.NodeAccessLog{ID: 101, NodeID: "n1", Host: "api.example.com", LoggedAt: base.Add(2 * time.Hour)},
		analyticsmodel.NodeAccessLog{ID: 102, NodeID: "n1", Host: "bucket.s3.example.com", LoggedAt: base.Add(-time.Second)},
	)
	if err := s.BatchInsertNodeAccessLogs(ctx, rows); err != nil {
		t.Fatalf("BatchInsertNodeAccessLogs(wildcard fixtures) error = %v, want nil", err)
	}

	tests := []struct {
		name    string
		hosts   []string
		host    string
		wantIDs []uint64
	}{
		{name: "exact", hosts: []string{" API.EXAMPLE.COM "}, wantIDs: []uint64{1}},
		{name: "wildcard_multilevel", hosts: []string{" *.Example.com "}, wantIDs: []uint64{1, 2, 3, 4, 6}},
		{name: "nested_wildcard_excludes_suffix_root", hosts: []string{"*.s3.example.com"}, wantIDs: []uint64{3, 4}},
		{name: "overlap_does_not_double_count", hosts: []string{"api.example.com", "*.example.com", "*.s3.example.com", "*.EXAMPLE.COM"}, host: "ignored", wantIDs: []uint64{1, 2, 3, 4, 6}},
		{name: "exact_root_and_nested_wildcard", hosts: []string{"example.com", "*.s3.example.com"}, wantIDs: []uint64{3, 4, 5}},
		{name: "exact_suffix_root", hosts: []string{"example.com"}, wantIDs: []uint64{5}},
		{name: "like_metacharacters_are_literal", hosts: []string{`*.a%_\'?.example.net`}, wantIDs: []uint64{10}},
		{name: "empty_prefix_is_not_a_subdomain", hosts: []string{"*.s3.other.com"}, wantIDs: []uint64{}},
		{name: "single_host_keeps_prefix_search", host: "api.example.com", wantIDs: []uint64{1, 8}},
		{name: "single_host_does_not_enable_wildcard", host: "*.example.com", wantIDs: []uint64{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := model.OpenFlareAccessLogQuery{NodeID: "n1", Hosts: tt.hosts, Host: tt.host, Since: base, Until: base.Add(2 * time.Hour)}
			type totals struct {
				Requests int64
				UniqueIP int64
				Bytes    int64
			}
			want := totals{Requests: int64(len(tt.wantIDs))}
			ips := make(map[string]struct{})
			wantBuckets := make(map[int64]totals)
			bucketIPs := make(map[int64]map[string]struct{})
			for _, id := range tt.wantIDs {
				r := rows[id-1]
				want.Bytes += int64(r.BytesSent)
				if r.RemoteAddr != "" {
					ips[r.RemoteAddr] = struct{}{}
				}
				epoch := r.LoggedAt.Truncate(time.Hour).Unix()
				bucket := wantBuckets[epoch]
				bucket.Requests++
				bucket.Bytes += int64(r.BytesSent)
				if bucketIPs[epoch] == nil {
					bucketIPs[epoch] = make(map[string]struct{})
				}
				if r.RemoteAddr != "" {
					bucketIPs[epoch][r.RemoteAddr] = struct{}{}
				}
				bucket.UniqueIP = int64(len(bucketIPs[epoch]))
				wantBuckets[epoch] = bucket
			}
			want.UniqueIP = int64(len(ips))
			total, unique, bytes, err := s.Count(ctx, q)
			if err != nil {
				t.Fatalf("Count(%+v) error = %v, want nil", q, err)
			}
			if got := (totals{total, unique, bytes}); got != want {
				t.Errorf("Count(%+v) = %+v, want %+v", q, got, want)
			}
			listed, err := s.List(ctx, q)
			if err != nil {
				t.Fatalf("List(%+v) error = %v, want nil", q, err)
			}
			gotIDs := make([]uint64, 0, len(listed))
			for _, r := range listed {
				gotIDs = append(gotIDs, uint64(r.ID))
			}
			slices.Sort(gotIDs)
			if diff := cmp.Diff(tt.wantIDs, gotIDs); diff != "" {
				t.Errorf("List(%+v) IDs mismatch (-want +got):\n%s", q, diff)
			}
			buckets, err := s.BucketAggregates(ctx, q, 3600)
			if err != nil {
				t.Fatalf("BucketAggregates(%+v, 3600) error = %v, want nil", q, err)
			}
			gotBuckets := make(map[int64]totals)
			for _, b := range buckets {
				gotBuckets[b.BucketEpoch] = totals{b.RequestCount, b.UniqueIPCount, b.BytesSent}
			}
			if diff := cmp.Diff(wantBuckets, gotBuckets); diff != "" {
				t.Errorf("BucketAggregates(%+v, 3600) mismatch (-want +got):\n%s", q, diff)
			}
			bucketCount, err := s.CountBuckets(ctx, q, 3600)
			if err != nil {
				t.Fatalf("CountBuckets(%+v, 3600) error = %v, want nil", q, err)
			}
			if bucketCount != int64(len(wantBuckets)) {
				t.Errorf("CountBuckets(%+v, 3600) = %d, want %d", q, bucketCount, len(wantBuckets))
			}
		})
	}
}
