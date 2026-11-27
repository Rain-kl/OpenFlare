// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package netintel

import (
	"bufio"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	pkgnetintel "github.com/Rain-kl/Wavelet/pkg/netintel"
)

const (
	defaultBGPToolsUserAgent = "OpenFlare-NetIntel/1.0 (+https://github.com/Rain-kl/OpenFlare)"
	geoIPSourceFyralabs      = "fyralabs"
	geoIPSourceSapics        = "sapics"
	defaultSkipFreshWindow   = time.Hour
	maxDownloadBytes         = 512 << 20 // 512 MiB cap per dataset file
	maxSmallDatasetBytes     = 32 << 20  // 32 MiB cap for index / tags text
	maxBackfillNodesBatch    = 500
	downloadHTTPTimeout      = 180 * time.Second
	netIntelDirPerm          = 0o750
	netIntelFilePerm         = 0o600
	caidaFreshWindow         = 12 * time.Hour
	tagLineMaxParts          = 2
	streamBufferBytes        = 32 * 1024
	gzipHeaderLen            = 2
	bz2VerifyBufSize         = 512
	licenseGeoLite2          = "CC BY-SA 4.0 / MaxMind GeoLite2 EULA"
	licenseSapics            = "PDDL-1.0 (sapics/ip-location-db)"
	licenseBGPTools          = "bgp.tools (Attribution Required)"
	licenseCAIDARuntime      = "CAIDA Acceptable Use Agreement (Runtime Opt-In)"
)

var (
	validTagNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	asnTokenPattern     = regexp.MustCompile(`AS([0-9]+)`)
	caidaRelFilePattern = regexp.MustCompile(`[0-9]{8}\.as-rel2\.txt\.bz2`)
)

// SyncEndpoints configures upstream download URLs for NetIntel datasets (overridable in tests).
type SyncEndpoints struct {
	FyralabsCountryURL  string
	FyralabsCityURL     string
	FyralabsASNURL      string
	SapicsCountryURL    string
	SapicsASNURL        string
	BGPToolsBaseURL     string
	CAIDASerial2BaseURL string
}

var (
	endpointsMu     sync.RWMutex
	activeEndpoints = SyncEndpoints{
		FyralabsCountryURL:  "https://github.com/FyraLabs/geolite2/releases/latest/download/GeoLite2-Country.mmdb",
		FyralabsCityURL:     "https://github.com/FyraLabs/geolite2/releases/latest/download/GeoLite2-City.mmdb",
		FyralabsASNURL:      "https://github.com/FyraLabs/geolite2/releases/latest/download/GeoLite2-ASN.mmdb",
		SapicsCountryURL:    "https://cdn.jsdelivr.net/npm/@ip-location-db/geo-whois-asn-country-mmdb/geo-whois-asn-country.mmdb",
		SapicsASNURL:        "https://cdn.jsdelivr.net/npm/@ip-location-db/asn-mmdb/asn.mmdb",
		BGPToolsBaseURL:     "https://bgp.tools",
		CAIDASerial2BaseURL: "https://publicdata.caida.org/datasets/as-relationships/serial-2/",
	}
)

// SetSyncEndpointsForTest overrides download URLs for unit tests and returns a cleanup callback.
func SetSyncEndpointsForTest(ep SyncEndpoints) func() {
	endpointsMu.Lock()
	previous := activeEndpoints
	activeEndpoints = ep
	endpointsMu.Unlock()
	return func() {
		endpointsMu.Lock()
		activeEndpoints = previous
		endpointsMu.Unlock()
	}
}

func currentEndpoints() SyncEndpoints {
	endpointsMu.RLock()
	ep := activeEndpoints
	endpointsMu.RUnlock()
	return ep
}

// SyncSummary summarizes one run of NetIntel database synchronization.
type SyncSummary struct {
	DataDir         string            `json:"data_dir"`
	GeoIPSource     string            `json:"geoip_source"`
	CAIDAEnabled    bool              `json:"caida_enabled"`
	UpdatedFiles    []string          `json:"updated_files"`
	SkippedFiles    []string          `json:"skipped_files,omitempty"`
	Errors          map[string]string `json:"errors,omitempty"`
	BackfilledNodes int               `json:"backfilled_nodes"`
	DurationMs      int64             `json:"duration_ms"`
}

type syncOptions struct {
	dataDir      string
	geoIPSource  string
	bgpToolsUA   string
	caidaEnabled bool
}

func readSyncOptions(ctx context.Context) syncOptions {
	opts := syncOptions{
		dataDir:     defaultDataDir,
		geoIPSource: geoIPSourceFyralabs,
		bgpToolsUA:  defaultBGPToolsUserAgent,
	}
	if db.DB(ctx) == nil {
		return opts
	}
	if cfg, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeyNetIntelDataDir); err == nil && strings.TrimSpace(cfg.Value) != "" {
		opts.dataDir = strings.TrimSpace(cfg.Value)
	}
	if cfg, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeyNetIntelGeoIPSource); err == nil {
		switch strings.ToLower(strings.TrimSpace(cfg.Value)) {
		case geoIPSourceFyralabs, geoIPSourceSapics:
			opts.geoIPSource = strings.ToLower(strings.TrimSpace(cfg.Value))
		}
	}
	if cfg, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeyNetIntelBGPToolsUA); err == nil && strings.TrimSpace(cfg.Value) != "" {
		opts.bgpToolsUA = strings.TrimSpace(cfg.Value)
	}
	if cfg, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeyNetIntelCAIDAEnabled); err == nil {
		opts.caidaEnabled = strings.EqualFold(strings.TrimSpace(cfg.Value), "true")
	}
	return opts
}

// SyncDatasets downloads updated GeoIP/ASN, bgp.tools, and optional CAIDA datasets into dataDir,
// writes manifest.json, atomically reloads the in-memory NetIntel manager, and backfills nodes missing ASN.
func SyncDatasets(ctx context.Context, force bool) (*SyncSummary, error) {
	started := time.Now()
	opts := readSyncOptions(ctx)
	ep := currentEndpoints()

	if err := os.MkdirAll(opts.dataDir, netIntelDirPerm); err != nil {
		return nil, fmt.Errorf("create netintel dir: %w", err)
	}

	summary := &SyncSummary{
		DataDir:      opts.dataDir,
		GeoIPSource:  opts.geoIPSource,
		CAIDAEnabled: opts.caidaEnabled,
		UpdatedFiles: []string{},
		SkippedFiles: []string{},
		Errors:       map[string]string{},
	}

	manifest := loadExistingManifest(opts.dataDir)
	manifest.GeoIPSource = opts.geoIPSource
	if manifest.Datasets == nil {
		manifest.Datasets = make(map[string]pkgnetintel.ManifestEntry)
	}

	client := &http.Client{Timeout: downloadHTTPTimeout}
	syncGeoIPDatasets(ctx, client, opts, ep, force, manifest, summary)
	syncBGPToolsDatasets(ctx, client, opts, ep, force, manifest, summary)
	if opts.caidaEnabled {
		syncCAIDADataset(ctx, client, opts, ep, force, manifest, summary)
	}

	manifest.GeneratedAtUnix = time.Now().UTC().Unix()
	if err := writeManifestAtomic(opts.dataDir, manifest); err != nil {
		summary.Errors["manifest"] = err.Error()
		logger.WarnF(ctx, "[NetIntel] write manifest.json failed: %v", err)
	}

	if err := Reload(ctx); err != nil {
		summary.Errors["reload"] = err.Error()
	}

	summary.BackfilledNodes = BackfillMissingNodeASNs(ctx)
	if len(summary.Errors) == 0 {
		summary.Errors = nil
	}
	summary.DurationMs = time.Since(started).Milliseconds()
	return summary, nil
}

type datasetTarget struct {
	key      string
	filename string
	url      string
	license  string
	ua       string
}

func syncGeoIPDatasets(
	ctx context.Context,
	client *http.Client,
	opts syncOptions,
	ep SyncEndpoints,
	force bool,
	manifest *pkgnetintel.Manifest,
	summary *SyncSummary,
) {
	var targets []datasetTarget
	if opts.geoIPSource == geoIPSourceSapics {
		targets = []datasetTarget{
			{key: "geoip_country", filename: "sapics-country.mmdb", url: ep.SapicsCountryURL, license: licenseSapics},
			{key: "geoip_asn", filename: "sapics-asn.mmdb", url: ep.SapicsASNURL, license: licenseSapics},
		}
	} else {
		targets = []datasetTarget{
			{key: "geoip_country", filename: "GeoLite2-Country.mmdb", url: ep.FyralabsCountryURL, license: licenseGeoLite2},
			{key: "geoip_city", filename: "GeoLite2-City.mmdb", url: ep.FyralabsCityURL, license: licenseGeoLite2},
			{key: "geoip_asn", filename: "GeoLite2-ASN.mmdb", url: ep.FyralabsASNURL, license: licenseGeoLite2},
		}
	}
	for _, target := range targets {
		syncDirectTarget(ctx, client, opts.dataDir, target, force, manifest, summary)
	}
}

func syncBGPToolsDatasets(
	ctx context.Context,
	client *http.Client,
	opts syncOptions,
	ep SyncEndpoints,
	force bool,
	manifest *pkgnetintel.Manifest,
	summary *SyncSummary,
) {
	base := strings.TrimRight(ep.BGPToolsBaseURL, "/")
	if base == "" {
		return
	}

	asnsTarget := datasetTarget{
		key:      "bgptools_asns",
		filename: "bgptools-asns.csv",
		url:      base + "/asns.csv",
		license:  licenseBGPTools,
		ua:       opts.bgpToolsUA,
	}
	syncDirectTarget(ctx, client, opts.dataDir, asnsTarget, force, manifest, summary)

	tagsTarget := datasetTarget{
		key:      "bgptools_tags",
		filename: "bgptools-tags.txt",
		url:      base + "/tags.txt",
		license:  licenseBGPTools,
		ua:       opts.bgpToolsUA,
	}
	syncDirectTarget(ctx, client, opts.dataDir, tagsTarget, force, manifest, summary)

	if err := syncBGPToolsTagMembers(ctx, client, opts.dataDir, base, opts.bgpToolsUA, force, manifest); err != nil {
		summary.Errors["bgptools_asn_tags"] = err.Error()
		logger.WarnF(ctx, "[NetIntel] bgp.tools tag memberships failed: %v", err)
	} else {
		summary.UpdatedFiles = append(summary.UpdatedFiles, "bgptools-asn-tags.txt")
	}

	tableDest := filepath.Join(opts.dataDir, "bgptools-table.jsonl.gz")
	if !force && isFreshFile(tableDest, defaultSkipFreshWindow) {
		summary.SkippedFiles = append(summary.SkippedFiles, "bgptools-table.jsonl.gz")
		return
	}
	entry, err := downloadBGPToolsTableGzip(ctx, client, base+"/table.jsonl", tableDest, opts.bgpToolsUA)
	if err != nil {
		summary.Errors["bgptools_table"] = err.Error()
		logger.WarnF(ctx, "[NetIntel] download bgp.tools table.jsonl failed: %v", err)
		return
	}
	manifest.Datasets["bgptools_table"] = entry
	summary.UpdatedFiles = append(summary.UpdatedFiles, entry.Filename)
}

func syncCAIDADataset(
	ctx context.Context,
	client *http.Client,
	opts syncOptions,
	ep SyncEndpoints,
	force bool,
	manifest *pkgnetintel.Manifest,
	summary *SyncSummary,
) {
	base := strings.TrimRight(ep.CAIDASerial2BaseURL, "/") + "/"
	dest := filepath.Join(opts.dataDir, "caida-as-rel2.txt.bz2")
	if !force && isFreshFile(dest, caidaFreshWindow) {
		summary.SkippedFiles = append(summary.SkippedFiles, "caida-as-rel2.txt.bz2")
		return
	}

	indexBytes, err := downloadBytes(ctx, client, base, "", maxSmallDatasetBytes)
	if err != nil {
		summary.Errors["caida_as_rel"] = err.Error()
		logger.WarnF(ctx, "[NetIntel] fetch CAIDA index failed: %v", err)
		return
	}
	matches := caidaRelFilePattern.FindAllString(string(indexBytes), -1)
	if len(matches) == 0 {
		summary.Errors["caida_as_rel"] = "no serial-2 as-rel2.txt.bz2 found in CAIDA directory index"
		return
	}
	latestName := matches[0]
	for _, m := range matches[1:] {
		if m > latestName {
			latestName = m
		}
	}
	sourceURL := base + latestName
	tmpPath := dest + ".tmp"
	size, shaHex, dlErr := downloadStreamToFile(ctx, client, sourceURL, "", tmpPath, maxDownloadBytes)
	if dlErr != nil {
		_ = os.Remove(tmpPath)
		summary.Errors["caida_as_rel"] = dlErr.Error()
		return
	}
	// Verify bzip2 framing before replacing active dataset.
	if err := verifyBzip2Readable(tmpPath); err != nil {
		_ = os.Remove(tmpPath)
		summary.Errors["caida_as_rel"] = fmt.Sprintf("invalid caida bzip2 archive: %v", err)
		return
	}
	if err := os.Rename(tmpPath, dest); err != nil {
		_ = os.Remove(tmpPath)
		summary.Errors["caida_as_rel"] = err.Error()
		return
	}
	manifest.Datasets["caida_as_rel"] = pkgnetintel.ManifestEntry{
		Filename:      "caida-as-rel2.txt.bz2",
		SourceURL:     sourceURL,
		License:       licenseCAIDARuntime,
		SHA256:        shaHex,
		SizeBytes:     size,
		FetchedAtUnix: time.Now().UTC().Unix(),
	}
	summary.UpdatedFiles = append(summary.UpdatedFiles, "caida-as-rel2.txt.bz2")
}

func syncDirectTarget(
	ctx context.Context,
	client *http.Client,
	dir string,
	target datasetTarget,
	force bool,
	manifest *pkgnetintel.Manifest,
	summary *SyncSummary,
) {
	if strings.TrimSpace(target.url) == "" {
		return
	}
	dest := filepath.Join(dir, target.filename)
	if !force && isFreshFile(dest, defaultSkipFreshWindow) {
		summary.SkippedFiles = append(summary.SkippedFiles, target.filename)
		return
	}
	tmpPath := dest + ".tmp"
	size, shaHex, err := downloadStreamToFile(ctx, client, target.url, target.ua, tmpPath, maxDownloadBytes)
	if err != nil {
		_ = os.Remove(tmpPath)
		summary.Errors[target.key] = err.Error()
		logger.WarnF(ctx, "[NetIntel] download %s failed: %v", target.filename, err)
		return
	}
	if err := os.Rename(tmpPath, dest); err != nil {
		_ = os.Remove(tmpPath)
		summary.Errors[target.key] = err.Error()
		return
	}
	manifest.Datasets[target.key] = pkgnetintel.ManifestEntry{
		Filename:      target.filename,
		SourceURL:     target.url,
		License:       target.license,
		SHA256:        shaHex,
		SizeBytes:     size,
		FetchedAtUnix: time.Now().UTC().Unix(),
	}
	summary.UpdatedFiles = append(summary.UpdatedFiles, target.filename)
}

func syncBGPToolsTagMembers(
	ctx context.Context,
	client *http.Client,
	dir string,
	baseURL string,
	ua string,
	force bool,
	manifest *pkgnetintel.Manifest,
) error {
	dest := filepath.Join(dir, "bgptools-asn-tags.txt")
	if !force && isFreshFile(dest, defaultSkipFreshWindow) {
		return nil
	}
	tagsFile, err := os.Open(filepath.Join(dir, "bgptools-tags.txt")) //nolint:gosec // validated fixed path under dataDir
	if err != nil {
		return err
	}
	defer func() { _ = tagsFile.Close() }()

	var out bytes.Buffer
	scanner := bufio.NewScanner(tagsFile)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ",", tagLineMaxParts)
		tagName := strings.TrimSpace(parts[0])
		if !validTagNamePattern.MatchString(tagName) {
			continue
		}
		if len(parts) == tagLineMaxParts && strings.TrimSpace(parts[1]) == "0" {
			continue
		}
		raw, dlErr := downloadBytes(ctx, client, baseURL+"/tags/"+tagName+".txt", ua, maxSmallDatasetBytes)
		if dlErr != nil {
			logger.WarnF(ctx, "[NetIntel] skip tag %s: %v", tagName, dlErr)
			continue
		}
		matches := asnTokenPattern.FindAllString(string(raw), -1)
		if len(matches) == 0 {
			continue
		}
		out.WriteString(tagName)
		out.WriteByte(' ')
		out.WriteString(strings.Join(matches, ","))
		out.WriteByte('\n')
	}
	if scanErr := scanner.Err(); scanErr != nil {
		return scanErr
	}

	tmpPath := dest + ".tmp"
	if err := os.WriteFile(tmpPath, out.Bytes(), netIntelFilePerm); err != nil { //nolint:gosec // temp file inside dataDir
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, dest); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	sum := sha256.Sum256(out.Bytes())
	manifest.Datasets["bgptools_asn_tags"] = pkgnetintel.ManifestEntry{
		Filename:      "bgptools-asn-tags.txt",
		SourceURL:     baseURL + "/tags/*.txt",
		License:       licenseBGPTools,
		SHA256:        hex.EncodeToString(sum[:]),
		SizeBytes:     int64(out.Len()),
		FetchedAtUnix: time.Now().UTC().Unix(),
	}
	return nil
}

func downloadBGPToolsTableGzip(
	ctx context.Context,
	client *http.Client,
	url string,
	dest string,
	ua string,
) (pkgnetintel.ManifestEntry, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return pkgnetintel.ManifestEntry{}, err
	}
	if ua != "" {
		req.Header.Set("User-Agent", ua)
	}
	resp, err := client.Do(req)
	if err != nil {
		return pkgnetintel.ManifestEntry{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return pkgnetintel.ManifestEntry{}, fmt.Errorf("unexpected status %d from %s", resp.StatusCode, url)
	}

	tmpPath := dest + ".tmp"
	file, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, netIntelFilePerm) //nolint:gosec // temp file inside dataDir
	if err != nil {
		return pkgnetintel.ManifestEntry{}, err
	}
	cleanupTmp := func() {
		_ = file.Close()
		_ = os.Remove(tmpPath)
	}

	limited := io.LimitReader(resp.Body, maxDownloadBytes)
	br := bufio.NewReaderSize(limited, streamBufferBytes)
	head, peekErr := br.Peek(gzipHeaderLen)
	if peekErr != nil && peekErr != io.EOF {
		cleanupTmp()
		return pkgnetintel.ManifestEntry{}, peekErr
	}
	alreadyGzip := len(head) == gzipHeaderLen && head[0] == 0x1f && head[1] == 0x8b

	hasher := sha256.New()
	multiOut := io.MultiWriter(file, hasher)
	var written int64
	if alreadyGzip {
		written, err = io.Copy(multiOut, br)
	} else {
		gzw := gzip.NewWriter(multiOut)
		written, err = io.Copy(gzw, br)
		if closeErr := gzw.Close(); err == nil {
			err = closeErr
		}
	}
	if err != nil {
		cleanupTmp()
		return pkgnetintel.ManifestEntry{}, err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return pkgnetintel.ManifestEntry{}, err
	}
	info, statErr := os.Stat(tmpPath)
	if statErr != nil || info.Size() == 0 || written == 0 {
		_ = os.Remove(tmpPath)
		return pkgnetintel.ManifestEntry{}, fmt.Errorf("downloaded empty bgp.tools table")
	}
	if err := os.Rename(tmpPath, dest); err != nil {
		_ = os.Remove(tmpPath)
		return pkgnetintel.ManifestEntry{}, err
	}
	return pkgnetintel.ManifestEntry{
		Filename:      filepath.Base(dest),
		SourceURL:     url,
		License:       licenseBGPTools,
		SHA256:        hex.EncodeToString(hasher.Sum(nil)),
		SizeBytes:     info.Size(),
		FetchedAtUnix: time.Now().UTC().Unix(),
	}, nil
}

func downloadBytes(ctx context.Context, client *http.Client, url, ua string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if ua != "" {
		req.Header.Set("User-Agent", ua)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status %d from %s", resp.StatusCode, url)
	}
	return io.ReadAll(io.LimitReader(resp.Body, limit))
}

func downloadStreamToFile(ctx context.Context, client *http.Client, url, ua, dest string, limit int64) (int64, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, "", err
	}
	if ua != "" {
		req.Header.Set("User-Agent", ua)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, "", fmt.Errorf("unexpected status %d from %s", resp.StatusCode, url)
	}

	file, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, netIntelFilePerm) //nolint:gosec // path is controlled temp destination
	if err != nil {
		return 0, "", err
	}
	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(file, hasher), io.LimitReader(resp.Body, limit))
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return 0, "", err
	}
	if written == 0 {
		return 0, "", fmt.Errorf("downloaded empty body from %s", url)
	}
	return written, hex.EncodeToString(hasher.Sum(nil)), nil
}

func verifyBzip2Readable(path string) error {
	file, err := os.Open(path) //nolint:gosec // temp file inside dataDir
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	buf := make([]byte, bz2VerifyBufSize)
	_, err = bzip2.NewReader(file).Read(buf)
	if err == io.EOF {
		return nil
	}
	return err
}

func isFreshFile(path string, maxAge time.Duration) bool {
	info, err := os.Stat(path)
	if err != nil || info.Size() == 0 {
		return false
	}
	return time.Since(info.ModTime()) < maxAge
}

func loadExistingManifest(dir string) *pkgnetintel.Manifest {
	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json")) //nolint:gosec // fixed filename under dataDir
	if err != nil {
		return &pkgnetintel.Manifest{Datasets: map[string]pkgnetintel.ManifestEntry{}}
	}
	var parsed pkgnetintel.Manifest
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return &pkgnetintel.Manifest{Datasets: map[string]pkgnetintel.ManifestEntry{}}
	}
	if parsed.Datasets == nil {
		parsed.Datasets = map[string]pkgnetintel.ManifestEntry{}
	}
	return &parsed
}

func writeManifestAtomic(dir string, manifest *pkgnetintel.Manifest) error {
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	dest := filepath.Join(dir, "manifest.json")
	tmpPath := dest + ".tmp"
	if err := os.WriteFile(tmpPath, encoded, netIntelFilePerm); err != nil { //nolint:gosec // temp file inside dataDir
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, dest); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

// BackfillMissingNodeASNs resolves ASN/ASNOrg for existing nodes with missing ASN metadata.
func BackfillMissingNodeASNs(ctx context.Context) int {
	if db.DB(ctx) == nil {
		return 0
	}
	nodes, err := repository.ListOpenFlareNodesMissingASN(ctx, maxBackfillNodesBatch)
	if err != nil {
		logger.WarnF(ctx, "[NetIntel] list nodes missing ASN failed: %v", err)
		return 0
	}
	updated := 0
	for i := range nodes {
		node := nodes[i]
		if !EnrichNodeASN(&node) {
			continue
		}
		if err := repository.UpdateOpenFlareNodeASN(ctx, node.NodeID, node.ASN, node.ASNOrg); err != nil {
			logger.WarnF(ctx, "[NetIntel] backfill node ASN failed for %s: %v", node.NodeID, err)
			continue
		}
		updated++
	}
	return updated
}
