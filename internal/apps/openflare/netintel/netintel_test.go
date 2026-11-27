// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package netintel

import (
	"bytes"
	"compress/bzip2"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/testhelper"
	pkgnetintel "github.com/Rain-kl/Wavelet/pkg/netintel"
)

func TestEnrichNodeASNAndReloadFromDir(t *testing.T) {
	dir := t.TempDir()
	asnsCSV := "asn,name,class,cc\nAS13335,\"Cloudflare, Inc.\",Content,US\n"
	tagsTxt := "anycast,1\n"
	asnTagsTxt := "anycast AS13335\n"
	tableJSONL := "{\"CIDR\":\"1.1.1.0/24\",\"ASN\":13335,\"Hits\":500}\n"

	if err := os.WriteFile(filepath.Join(dir, "bgptools-asns.csv"), []byte(asnsCSV), 0o600); err != nil {
		t.Fatalf("write asns: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bgptools-tags.txt"), []byte(tagsTxt), 0o600); err != nil {
		t.Fatalf("write tags: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bgptools-asn-tags.txt"), []byte(asnTagsTxt), 0o600); err != nil {
		t.Fatalf("write asn tags: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bgptools-table.jsonl"), []byte(tableJSONL), 0o600); err != nil {
		t.Fatalf("write table: %v", err)
	}

	restoreMgr := SetManagerForTest(pkgnetintel.NewManager())
	defer restoreMgr()

	if err := ReloadFromDir(context.Background(), dir); err != nil {
		t.Fatalf("ReloadFromDir failed: %v", err)
	}

	res := LookupIP("1.1.1.1")
	if res == nil || res.ASN != 13335 || res.ASNName != "Cloudflare, Inc." {
		t.Fatalf("unexpected lookup result: %+v", res)
	}

	node := &model.OpenFlareNode{
		NodeID: "edge-1",
		IP:     "1.1.1.1",
	}
	if !EnrichNodeASN(node) {
		t.Fatalf("expected EnrichNodeASN to mutate node")
	}
	if node.ASN != 13335 || node.ASNOrg != "Cloudflare, Inc." {
		t.Fatalf("unexpected node ASN fields: %+v", node)
	}
	if EnrichNodeASN(node) {
		t.Fatalf("expected second EnrichNodeASN call to report no change")
	}
}

func TestSyncDatasetsFromMockServer(t *testing.T) {
	_, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()
	if err := db.DB(context.Background()).AutoMigrate(&model.OpenFlareNode{}); err != nil {
		t.Fatalf("automigrate node table: %v", err)
	}

	restoreMgr := SetManagerForTest(pkgnetintel.NewManager())
	defer restoreMgr()

	dir := t.TempDir()
	ctx := context.Background()
	fallbackCountry := readFallbackCountryMMDB()
	if len(fallbackCountry) == 0 {
		fallbackCountry = []byte("mock-mmdb")
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/mmdb/country.mmdb", "/mmdb/city.mmdb", "/mmdb/asn.mmdb":
			_, _ = w.Write(fallbackCountry)
		case "/asns.csv":
			if r.Header.Get("User-Agent") == "" {
				http.Error(w, "missing User-Agent", http.StatusForbidden)
				return
			}
			_, _ = w.Write([]byte("asn,name,class,cc\nAS13335,Cloudflare,Content,US\nAS174,Cogent,Transit,US\n"))
		case "/tags.txt":
			_, _ = w.Write([]byte("cdn,1\nempty,0\n"))
		case "/tags/cdn.txt":
			_, _ = w.Write([]byte("AS13335\n"))
		case "/table.jsonl":
			_, _ = w.Write([]byte("{\"CIDR\":\"1.1.1.0/24\",\"ASN\":13335,\"Hits\":999}\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	restoreEP := SetSyncEndpointsForTest(SyncEndpoints{
		FyralabsCountryURL:  srv.URL + "/mmdb/country.mmdb",
		FyralabsCityURL:     srv.URL + "/mmdb/city.mmdb",
		FyralabsASNURL:      srv.URL + "/mmdb/asn.mmdb",
		BGPToolsBaseURL:     srv.URL,
		CAIDASerial2BaseURL: srv.URL + "/caida/",
	})
	defer restoreEP()

	if err := repository.SaveOrUpdateSystemConfig(ctx, model.ConfigKeyNetIntelDataDir, dir); err != nil {
		t.Fatalf("save data dir config: %v", err)
	}
	if err := repository.SaveOrUpdateSystemConfig(ctx, model.ConfigKeyNetIntelGeoIPSource, "fyralabs"); err != nil {
		t.Fatalf("save geoip source config: %v", err)
	}
	if err := repository.SaveOrUpdateSystemConfig(ctx, model.ConfigKeyNetIntelBGPToolsUA, "OpenFlare-Test/1.0"); err != nil {
		t.Fatalf("save ua config: %v", err)
	}

	node := &model.OpenFlareNode{
		NodeID:           "node-backfill-1",
		Name:             "backfill-edge",
		IP:               "1.1.1.1",
		AccessToken:      "token-backfill-1234567890",
		Status:           "online",
		NodeType:         "edge_node",
		CapabilitiesJSON: "[]",
	}
	if err := repository.CreateOpenFlareNode(ctx, node); err != nil {
		t.Fatalf("create node: %v", err)
	}

	summary, err := SyncDatasets(ctx, true)
	if err != nil {
		t.Fatalf("SyncDatasets failed: %v", err)
	}
	if summary.CAIDAEnabled {
		t.Fatalf("expected CAIDA disabled by default")
	}
	if len(summary.Errors) != 0 {
		t.Fatalf("unexpected sync errors: %+v", summary.Errors)
	}
	if summary.BackfilledNodes != 1 {
		t.Fatalf("expected 1 backfilled node, got %d", summary.BackfilledNodes)
	}

	updatedNode, err := repository.GetOpenFlareNodeByNodeID(ctx, "node-backfill-1")
	if err != nil {
		t.Fatalf("load backfilled node: %v", err)
	}
	if updatedNode.ASN != 13335 || updatedNode.ASNOrg != "Cloudflare" {
		t.Fatalf("unexpected backfilled node ASN: %+v", updatedNode)
	}

	if res := LookupIP("1.1.1.1"); res == nil || res.ASN != 13335 {
		t.Fatalf("expected reloaded LookupIP for 1.1.1.1 -> AS13335, got %+v", res)
	}
	if info := LookupASN(13335); info == nil || info.Name != "Cloudflare" || len(info.Tags) != 1 || info.Tags[0] != "cdn" {
		t.Fatalf("unexpected LookupASN result: %+v", info)
	}

	if _, err := os.Stat(filepath.Join(dir, "manifest.json")); err != nil {
		t.Fatalf("expected manifest.json to exist: %v", err)
	}

	// Second run with force=false skips fresh files
	second, err := SyncDatasets(ctx, false)
	if err != nil {
		t.Fatalf("second SyncDatasets failed: %v", err)
	}
	if len(second.SkippedFiles) == 0 {
		t.Fatalf("expected fresh files to be skipped on non-forced sync")
	}
}

func TestNetIntelSyncHandlerValidateAndExecute(t *testing.T) {
	handler := &SyncHandler{}

	normalized, err := handler.ValidatePayload(nil)
	if err != nil {
		t.Fatalf("empty ValidatePayload failed: %v", err)
	}
	var payload SyncPayload
	if err := json.Unmarshal(normalized, &payload); err != nil || payload.Force {
		t.Fatalf("unexpected normalized payload: %s (%v)", string(normalized), err)
	}

	if _, err := handler.ValidatePayload([]byte("{invalid")); err == nil {
		t.Fatalf("expected error on invalid JSON payload")
	}

	// Confirm verifyBzip2Readable rejects garbage without panic.
	badBZ2 := filepath.Join(t.TempDir(), "bad.bz2")
	_ = os.WriteFile(badBZ2, []byte("not-bz2"), 0o600)
	if err := verifyBzip2Readable(badBZ2); err == nil {
		t.Fatalf("expected verifyBzip2Readable to fail on non-bz2 bytes")
	} else {
		_ = bzip2.NewReader(bytes.NewReader(nil))
	}
}
