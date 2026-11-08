// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package zone

import (
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/testhelper"
	"github.com/google/go-cmp/cmp"
)

func TestGetStatsWildcardHosts(t *testing.T) {
	type totals struct {
		Requests int64
		Visitors int64
		Bytes    int64
	}
	tests := []struct {
		name    string
		domains []string
		want    totals
		buckets map[int]totals
	}{
		{
			name:    "wildcard_matches_multiple_levels_without_root",
			domains: []string{"*.example.com"}, want: totals{Requests: 5, Visitors: 3, Bytes: 150},
			buckets: map[int]totals{0: {Requests: 2, Visitors: 1, Bytes: 30}, 1: {Requests: 3, Visitors: 2, Bytes: 120}},
		},
		{
			name:    "nested_wildcard_excludes_its_suffix_root",
			domains: []string{"*.s3.example.com"}, want: totals{Requests: 2, Visitors: 1, Bytes: 70},
			buckets: map[int]totals{1: {Requests: 2, Visitors: 1, Bytes: 70}},
		},
		{
			name:    "overlapping_exact_and_wildcards_are_not_summed_twice",
			domains: []string{"api.example.com", "*.example.com", "*.s3.example.com"}, want: totals{Requests: 5, Visitors: 3, Bytes: 150},
			buckets: map[int]totals{0: {Requests: 2, Visitors: 1, Bytes: 30}, 1: {Requests: 3, Visitors: 2, Bytes: 120}},
		},
		{
			name:    "exact_root_can_be_included_explicitly",
			domains: []string{"example.com", "*.s3.example.com"}, want: totals{Requests: 3, Visitors: 2, Bytes: 130},
			buckets: map[int]totals{0: {Requests: 1, Visitors: 1, Bytes: 60}, 1: {Requests: 2, Visitors: 1, Bytes: 70}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := setupZoneDB(t)
			testhelper.SetupLogStoresForTest(t)
			site, err := Create(ctx, Input{Domain: "example.com"})
			if err != nil {
				t.Fatalf("Create(example.com) error = %v, want nil", err)
			}
			// 直接登记域名，隔离流量统计与业务层域名校验的回归范围。
			for _, domain := range tt.domains {
				if err := repository.CreateZoneDomain(ctx, &model.ZoneDomain{ZoneID: site.ID, Domain: domain}); err != nil {
					t.Fatalf("CreateZoneDomain(%q) error = %v, want nil", domain, err)
				}
			}
			base := time.Now().UTC().Truncate(time.Hour).Add(-3 * time.Hour)
			rows := []*model.OpenFlareAccessLog{
				{Host: "api.example.com", LoggedAt: base.Add(time.Minute), RemoteAddr: "1.1.1.1", BytesSent: 10},
				{Host: " Deep.API.Example.COM ", LoggedAt: base.Add(2 * time.Minute), RemoteAddr: "1.1.1.1", BytesSent: 20},
				{Host: "bucket.s3.example.com", LoggedAt: base.Add(time.Hour + time.Minute), RemoteAddr: "2.2.2.2", BytesSent: 30},
				{Host: "deep.bucket.s3.example.com", LoggedAt: base.Add(time.Hour + 2*time.Minute), BytesSent: 40},
				{Host: "s3.example.com", LoggedAt: base.Add(time.Hour + 3*time.Minute), RemoteAddr: "3.3.3.3", BytesSent: 50},
				{Host: "example.com", LoggedAt: base.Add(3 * time.Minute), RemoteAddr: "1.1.1.1", BytesSent: 60},
				{Host: "evil-example.com", LoggedAt: base.Add(time.Minute), RemoteAddr: "9.9.9.9", BytesSent: 70},
				{Host: "bucket.s3.example.com.evil", LoggedAt: base.Add(time.Minute), RemoteAddr: "9.9.9.9", BytesSent: 80},
				{Host: "bucket.s3.example.com", LoggedAt: base.Add(-48 * time.Hour), RemoteAddr: "9.9.9.9", BytesSent: 90},
			}
			if err := repository.InsertOpenFlareAccessLogsBatch(ctx, rows); err != nil {
				t.Fatalf("InsertOpenFlareAccessLogsBatch(wildcard fixtures) error = %v, want nil", err)
			}
			stats, err := GetStats(ctx, site.ID, "24h")
			if err != nil {
				t.Fatalf("GetStats(%d, 24h) error = %v, want nil", site.ID, err)
			}
			if got := (totals{Requests: stats.RequestCount, Visitors: stats.UniqueVisitors, Bytes: stats.BytesSent}); got != tt.want {
				t.Errorf("GetStats(domains=%v) totals = %+v, want %+v", tt.domains, got, tt.want)
			}
			if !stats.Available || stats.DomainCount != len(tt.domains) {
				t.Errorf("GetStats(domains=%v) available/domain_count = %t/%d, want true/%d", tt.domains, stats.Available, stats.DomainCount, len(tt.domains))
			}
			wantBuckets := make(map[int64]totals)
			for offset, total := range tt.buckets {
				wantBuckets[base.Add(time.Duration(offset)*time.Hour).Unix()] = total
			}
			gotBuckets := make(map[int64]totals)
			for _, point := range stats.Series {
				if point.RequestCount != 0 || point.UniqueVisitors != 0 || point.BytesSent != 0 {
					gotBuckets[point.BucketStartedAt.Unix()] = totals{Requests: point.RequestCount, Visitors: point.UniqueVisitors, Bytes: point.BytesSent}
				}
			}
			if diff := cmp.Diff(wantBuckets, gotBuckets); diff != "" {
				t.Errorf("GetStats(domains=%v) nonempty series mismatch (-want +got):\n%s", tt.domains, diff)
			}
		})
	}
}
