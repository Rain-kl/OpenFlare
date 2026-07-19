// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package githubrelease resolves and downloads public GitHub Release assets.
// It deliberately does not know about Pages projects, deployments or runtime
// state so other callers can reuse the same constrained HTTP contract.
package githubrelease

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	// APIVersion is the GitHub REST API contract used by this package.
	APIVersion = "2026-03-10"

	// SelectorLatest uses GitHub's repository latest-release endpoint.
	SelectorLatest Selector = "latest"
	// SelectorTag resolves one exact GitHub release tag.
	SelectorTag Selector = "tag"

	defaultAPIBaseURL   = "https://api.github.com"
	defaultUserAgent    = "OpenFlare-GitHubRelease/1.0"
	metadataAccept      = "application/vnd.github+json"
	assetAccept         = "application/octet-stream"
	maxMetadataBytes    = 4 << 20
	maxAssetErrorNames  = 10
	maxSafeTextBytes    = 255
	maxSafeAssetNameLen = 96
	maxDigestBytes      = 96
	maxETagBytes        = 512
	safePartsCapacity   = 6
)

var (
	errInvalidRequest = errors.New("GitHub Release 请求参数无效")
	errMetadata       = errors.New("GitHub Release 元数据响应无效")
	errAssetMissing   = errors.New("GitHub Release 中未找到指定的已上传 asset")
	errDownload       = errors.New("GitHub Release asset 下载失败")
	errTooLarge       = errors.New("GitHub Release asset 超过大小限制")
	errEmptyAsset     = errors.New("GitHub Release asset 内容为空")
	errDigest         = errors.New("GitHub Release asset digest 无效或校验失败")
	errCleanup        = errors.New("GitHub Release 临时文件清理失败")

	ownerPattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,37}[A-Za-z0-9])?$`)
	repoPattern  = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	hexPattern   = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)
)

var (
	// ErrInvalidRequest identifies caller configuration errors.
	ErrInvalidRequest = errInvalidRequest
	// ErrMetadata identifies malformed, unavailable or failed Release metadata requests.
	ErrMetadata = errMetadata
	// ErrAssetNotFound identifies an otherwise valid Release without the exact uploaded asset.
	ErrAssetNotFound = errAssetMissing
	// ErrDownload identifies network or HTTP failures while downloading an asset.
	ErrDownload = errDownload
	// ErrAssetTooLarge identifies assets that exceed the caller's hard byte limit.
	ErrAssetTooLarge = errTooLarge
	// ErrEmptyAsset identifies an empty downloaded asset.
	ErrEmptyAsset = errEmptyAsset
	// ErrDigestMismatch identifies malformed or mismatched declared SHA-256 digests.
	ErrDigestMismatch = errDigest
)

// Selector identifies GitHub's own latest endpoint or one exact tag.
type Selector string

// ResolveRequest describes one public repository release asset lookup.
type ResolveRequest struct {
	Repository string
	Selector   Selector
	Tag        string
	AssetName  string
	ETag       string
}

// Release contains only metadata safe and necessary for source resolution.
type Release struct {
	ID          string    `json:"release_id"`
	Tag         string    `json:"tag"`
	Name        string    `json:"name,omitempty"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
	PublishedAt time.Time `json:"published_at,omitempty"`
}

// Asset contains the immutable target metadata returned by a resolve call.
type Asset struct {
	ID        string    `json:"asset_id"`
	Name      string    `json:"asset_name"`
	State     string    `json:"state"`
	Size      int64     `json:"size"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
	Digest    string    `json:"digest,omitempty"`
}

// ResolveResult is either a selected uploaded asset or a not-modified marker.
type ResolveResult struct {
	NotModified bool       `json:"not_modified"`
	ETag        string     `json:"etag,omitempty"`
	Release     Release    `json:"release,omitempty"`
	Asset       Asset      `json:"asset,omitempty"`
	RetryAt     *time.Time `json:"retry_at,omitempty"`
}

// DownloadRequest identifies an already resolved asset. Asset IDs never come
// from an untrusted URL and the download endpoint is built locally.
type DownloadRequest struct {
	Repository string
	Asset      Asset
	MaxBytes   int64
}

// DownloadResult owns a temporary file. Call Cleanup after ingestion.
type DownloadResult struct {
	Path           string
	Size           int64
	SHA256         string
	DeclaredDigest string
}

// Cleanup removes the temporary file and is safe to call more than once.
func (result *DownloadResult) Cleanup() error {
	if result == nil || result.Path == "" {
		return nil
	}
	name := result.Path
	err := os.Remove(name)
	if err == nil || errors.Is(err, os.ErrNotExist) {
		result.Path = ""
		return nil
	}
	return errCleanup
}

// Error is a safe provider error. It never retains a response body, request
// URL, redirect location or request headers.
type Error struct {
	Kind            error
	StatusCode      int
	RequestID       string
	Repository      string
	Tag             string
	AssetName       string
	AvailableAssets []string
	RetryAt         *time.Time
}

func (providerError *Error) Error() string {
	if providerError == nil {
		return "GitHub Release 请求失败"
	}
	message := "GitHub Release 请求失败"
	if providerError.Kind != nil {
		message = providerError.Kind.Error()
	}
	parts := make([]string, 0, safePartsCapacity)
	if providerError.StatusCode != 0 {
		parts = append(parts, "status="+strconv.Itoa(providerError.StatusCode))
	}
	if providerError.RequestID != "" {
		parts = append(parts, "request_id="+providerError.RequestID)
	}
	if providerError.Repository != "" {
		parts = append(parts, "repo="+providerError.Repository)
	}
	if providerError.Tag != "" {
		parts = append(parts, "tag="+providerError.Tag)
	}
	if providerError.AssetName != "" {
		parts = append(parts, "asset="+providerError.AssetName)
	}
	if len(providerError.AvailableAssets) > 0 {
		parts = append(parts, "available="+strings.Join(providerError.AvailableAssets, ","))
	}
	if len(parts) == 0 {
		return message
	}
	return message + " (" + strings.Join(parts, " ") + ")"
}

func (providerError *Error) Unwrap() error {
	if providerError == nil {
		return nil
	}
	return providerError.Kind
}

// RetryAt extracts the server-directed retry deadline from an error.
func RetryAt(err error) (time.Time, bool) {
	var providerError *Error
	if !errors.As(err, &providerError) || providerError.RetryAt == nil {
		return time.Time{}, false
	}
	return *providerError.RetryAt, true
}

// RetryTime is retained as a compatibility alias for early callers.
//
// Deprecated: use RetryAt.
func RetryTime(err error) (time.Time, bool) {
	return RetryAt(err)
}

// IsNotFound reports both a missing Release endpoint and a Release that lacks
// the exact uploaded asset requested by the caller.
func IsNotFound(err error) bool {
	if errors.Is(err, ErrAssetNotFound) {
		return true
	}
	var providerError *Error
	return errors.As(err, &providerError) && providerError.StatusCode == http.StatusNotFound
}

// IsDigestError reports malformed or mismatched declared asset digests.
func IsDigestError(err error) bool {
	return errors.Is(err, ErrDigestMismatch)
}

// IsRetryable classifies provider failures without relying on localized error
// strings. Configuration, not-found, size, empty-content and digest failures
// are permanent. Network failures, 408/425/429 and 5xx responses are retryable.
func IsRetryable(err error) bool {
	if err == nil || errors.Is(err, ErrInvalidRequest) || IsNotFound(err) ||
		errors.Is(err, ErrAssetTooLarge) || errors.Is(err, ErrEmptyAsset) || IsDigestError(err) {
		return false
	}
	var providerError *Error
	if !errors.As(err, &providerError) {
		return false
	}
	if providerError.StatusCode == 0 {
		return errors.Is(err, ErrMetadata) || errors.Is(err, ErrDownload) || errors.Is(err, errCleanup)
	}
	if providerError.StatusCode < http.StatusBadRequest {
		return errors.Is(err, ErrMetadata) || errors.Is(err, ErrDownload) || errors.Is(err, errCleanup)
	}
	if providerError.RetryAt != nil {
		return true
	}
	return providerError.StatusCode == http.StatusRequestTimeout ||
		providerError.StatusCode == http.StatusTooEarly ||
		providerError.StatusCode == http.StatusTooManyRequests ||
		providerError.StatusCode >= http.StatusInternalServerError
}

// Client accesses public GitHub Releases using a fixed, constrained transport.
type Client struct {
	httpClient *http.Client
	baseURL    string
	createTemp func(string, string) (*os.File, error)
	now        func() time.Time
}

// NewClient constructs a production client for api.github.com. Public
// repositories do not require or send a token.
func NewClient() *Client {
	return newClient(defaultClientOptions())
}

// Resolve calls GitHub's latest or exact-tag endpoint and selects one exact,
// case-sensitive uploaded asset. It never falls back to source archives.
func (client *Client) Resolve(ctx context.Context, request ResolveRequest) (ResolveResult, error) {
	repository, tag, endpoint, err := normalizeResolveRequest(client.baseURL, request)
	if err != nil {
		return ResolveResult{}, safeError(errInvalidRequest, 0, "", repository, tag, validErrorAssetName(request.AssetName), nil, nil)
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return ResolveResult{}, safeError(errInvalidRequest, 0, "", repository, tag, validErrorAssetName(request.AssetName), nil, nil)
	}
	applyMetadataHeaders(httpRequest, request.ETag)
	response, err := client.httpClient.Do(httpRequest) //nolint:gosec // endpoint and every dial target are constrained
	if err != nil {
		return ResolveResult{}, safeError(errMetadata, 0, "", repository, tag, request.AssetName, nil, nil)
	}
	defer func() { _ = response.Body.Close() }()

	retryAt := responseRetryAt(response, client.now())
	etag := safeETag(response.Header.Get("ETag"))
	if response.StatusCode == http.StatusNotModified {
		if etag == "" {
			etag = safeETag(request.ETag)
		}
		return ResolveResult{NotModified: true, ETag: etag, RetryAt: retryAt}, nil
	}
	if response.StatusCode != http.StatusOK {
		return ResolveResult{}, safeHTTPError(errMetadata, response, repository, tag, request.AssetName, retryAt)
	}

	body, readErr := io.ReadAll(io.LimitReader(response.Body, maxMetadataBytes+1))
	if readErr != nil || len(body) > maxMetadataBytes || !utf8.Valid(body) {
		return ResolveResult{}, safeHTTPError(errMetadata, response, repository, tag, request.AssetName, retryAt)
	}
	var payload releasePayload
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return ResolveResult{}, safeHTTPError(errMetadata, response, repository, tag, request.AssetName, retryAt)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return ResolveResult{}, safeHTTPError(errMetadata, response, repository, tag, request.AssetName, retryAt)
	}
	release, assets, err := convertRelease(payload)
	if err != nil {
		return ResolveResult{}, safeHTTPError(errMetadata, response, repository, tag, request.AssetName, retryAt)
	}
	for _, asset := range assets {
		if asset.State == "uploaded" && asset.Name == request.AssetName {
			return ResolveResult{
				ETag:    etag,
				Release: release,
				Asset:   asset,
				RetryAt: retryAt,
			}, nil
		}
	}
	available := safeAssetNames(assets)
	return ResolveResult{}, safeError(
		errAssetMissing,
		response.StatusCode,
		response.Header.Get("X-GitHub-Request-Id"),
		repository,
		release.Tag,
		request.AssetName,
		available,
		retryAt,
	)
}

// Download streams an asset into a package-owned temporary file while
// enforcing a hard byte limit and verifying GitHub's declared sha256 digest.
func (client *Client) Download(ctx context.Context, request DownloadRequest) (*DownloadResult, error) {
	repository, err := normalizeRepository(request.Repository)
	if err != nil || request.MaxBytes <= 0 || !validPositiveID(request.Asset.ID) ||
		!validAssetName(request.Asset.Name) || request.Asset.Size < 0 {
		return nil, safeError(errInvalidRequest, 0, "", repository, "", validErrorAssetName(request.Asset.Name), nil, nil)
	}
	if request.Asset.Size > request.MaxBytes {
		return nil, safeError(errTooLarge, 0, "", repository, "", validErrorAssetName(request.Asset.Name), nil, nil)
	}
	endpoint := strings.TrimRight(client.baseURL, "/") + "/repos/" + repository + "/releases/assets/" + request.Asset.ID
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, safeError(errInvalidRequest, 0, "", repository, "", request.Asset.Name, nil, nil)
	}
	applyAssetHeaders(httpRequest)
	response, err := client.httpClient.Do(httpRequest) //nolint:gosec // endpoint and every dial target are constrained
	if err != nil {
		return nil, safeError(errDownload, 0, "", repository, "", request.Asset.Name, nil, nil)
	}
	defer func() { _ = response.Body.Close() }()
	retryAt := responseRetryAt(response, client.now())
	if response.StatusCode != http.StatusOK {
		return nil, safeHTTPError(errDownload, response, repository, "", request.Asset.Name, retryAt)
	}
	if response.ContentLength > request.MaxBytes {
		return nil, safeHTTPError(errTooLarge, response, repository, "", request.Asset.Name, retryAt)
	}

	result, err := client.streamAsset(response.Body, request.MaxBytes, request.Asset.Digest)
	if err != nil {
		return nil, safeError(err, response.StatusCode, response.Header.Get("X-GitHub-Request-Id"), repository, "", request.Asset.Name, nil, retryAt)
	}
	return result, nil
}

func (client *Client) streamAsset(body io.Reader, maxBytes int64, declaredDigest string) (result *DownloadResult, resultErr error) {
	tempFile, err := client.createTemp("", "openflare-github-release-*")
	if err != nil {
		return nil, errDownload
	}
	tempPath := tempFile.Name()
	defer func() {
		closeErr := tempFile.Close()
		if resultErr == nil && closeErr != nil {
			resultErr = errDownload
		}
		if resultErr != nil {
			if removeErr := os.Remove(tempPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				resultErr = errCleanup
			}
		}
	}()

	hasher := sha256.New()
	readLimit := maxBytes
	if readLimit < math.MaxInt64 {
		readLimit++
	}
	size, err := io.Copy(io.MultiWriter(tempFile, hasher), io.LimitReader(body, readLimit))
	if err != nil {
		return nil, errDownload
	}
	if size > maxBytes {
		return nil, errTooLarge
	}
	if size == 0 {
		return nil, errEmptyAsset
	}
	checksum := hex.EncodeToString(hasher.Sum(nil))
	if err := verifyDeclaredDigest(declaredDigest, checksum); err != nil {
		return nil, err
	}
	return &DownloadResult{
		Path:           tempPath,
		Size:           size,
		SHA256:         checksum,
		DeclaredDigest: strings.ToLower(strings.TrimSpace(declaredDigest)),
	}, nil
}

type releasePayload struct {
	ID          json.Number    `json:"id"`
	Tag         string         `json:"tag_name"`
	Name        string         `json:"name"`
	Draft       bool           `json:"draft"`
	Prerelease  bool           `json:"prerelease"`
	PublishedAt string         `json:"published_at"`
	Assets      []assetPayload `json:"assets"`
}

type assetPayload struct {
	ID        json.Number `json:"id"`
	Name      string      `json:"name"`
	State     string      `json:"state"`
	Size      int64       `json:"size"`
	UpdatedAt string      `json:"updated_at"`
	Digest    string      `json:"digest"`
}

func convertRelease(payload releasePayload) (Release, []Asset, error) {
	releaseID, err := positiveJSONID(payload.ID)
	if err != nil {
		return Release{}, nil, err
	}
	if !validReleaseDisplayTag(payload.Tag) {
		return Release{}, nil, errMetadata
	}
	publishedAt, err := parseOptionalTime(payload.PublishedAt)
	if err != nil {
		return Release{}, nil, err
	}
	release := Release{
		ID:          releaseID,
		Tag:         payload.Tag,
		Name:        safeText(payload.Name, maxSafeTextBytes),
		Draft:       payload.Draft,
		Prerelease:  payload.Prerelease,
		PublishedAt: publishedAt,
	}
	assets := make([]Asset, 0, len(payload.Assets))
	for _, rawAsset := range payload.Assets {
		assetID, assetErr := positiveJSONID(rawAsset.ID)
		if assetErr != nil || rawAsset.Size < 0 {
			return Release{}, nil, errMetadata
		}
		updatedAt, assetErr := parseOptionalTime(rawAsset.UpdatedAt)
		if assetErr != nil {
			return Release{}, nil, errMetadata
		}
		assets = append(assets, Asset{
			ID:        assetID,
			Name:      rawAsset.Name,
			State:     rawAsset.State,
			Size:      rawAsset.Size,
			UpdatedAt: updatedAt,
			Digest:    safeText(rawAsset.Digest, maxDigestBytes),
		})
	}
	return release, assets, nil
}

func normalizeResolveRequest(baseURL string, request ResolveRequest) (string, string, string, error) {
	repository, err := normalizeRepository(request.Repository)
	if err != nil || !validAssetName(request.AssetName) {
		return repository, validErrorTag(request.Tag), "", errInvalidRequest
	}
	baseURL = strings.TrimRight(baseURL, "/")
	switch request.Selector {
	case SelectorLatest:
		if strings.TrimSpace(request.Tag) != "" {
			return repository, "", "", errInvalidRequest
		}
		return repository, "latest", baseURL + "/repos/" + repository + "/releases/latest", nil
	case SelectorTag:
		if !validTag(request.Tag) {
			return repository, validErrorTag(request.Tag), "", errInvalidRequest
		}
		return repository, request.Tag, baseURL + "/repos/" + repository + "/releases/tags/" + url.PathEscape(request.Tag), nil
	default:
		return repository, validErrorTag(request.Tag), "", errInvalidRequest
	}
}

func normalizeRepository(repository string) (string, error) {
	repository = strings.TrimSpace(repository)
	parts := strings.Split(repository, "/")
	if len(parts) != 2 || !ownerPattern.MatchString(parts[0]) || !repoPattern.MatchString(parts[1]) ||
		len(parts[1]) > 100 || parts[1] == "." || parts[1] == ".." {
		return "", errInvalidRequest
	}
	return parts[0] + "/" + parts[1], nil
}

func validAssetName(assetName string) bool {
	return validLogText(assetName, maxSafeTextBytes, false) && path.Base(assetName) == assetName &&
		assetName != "." && assetName != ".." && !strings.ContainsAny(assetName, `/\`)
}

func validTag(tag string) bool {
	if !validLogText(tag, maxSafeTextBytes, false) || strings.ContainsAny(tag, " ~^:?*[\\") ||
		strings.Contains(tag, "..") || strings.Contains(tag, "@{") || strings.Contains(tag, "//") ||
		strings.HasPrefix(tag, "/") || strings.HasSuffix(tag, "/") || strings.HasSuffix(tag, ".") {
		return false
	}
	for _, component := range strings.Split(tag, "/") {
		if component == "" || strings.HasPrefix(component, ".") || strings.HasSuffix(component, ".lock") {
			return false
		}
	}
	return true
}

func validReleaseDisplayTag(tag string) bool {
	return validLogText(tag, maxSafeTextBytes, false)
}

func validLogText(value string, maxBytes int, allowEmpty bool) bool {
	if (!allowEmpty && value == "") || len(value) > maxBytes || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if isLogControl(character) {
			return false
		}
	}
	return true
}

func isLogControl(character rune) bool {
	if unicode.IsControl(character) || character == '\u2028' || character == '\u2029' {
		return true
	}
	switch character {
	case '\u061c', '\u200e', '\u200f',
		'\u202a', '\u202b', '\u202c', '\u202d', '\u202e',
		'\u2066', '\u2067', '\u2068', '\u2069':
		return true
	default:
		return false
	}
}

func validErrorTag(tag string) string {
	if !validTag(tag) || containsSecretDelimiter(tag) {
		return ""
	}
	return tag
}

func validErrorAssetName(assetName string) string {
	if !validAssetName(assetName) || containsSecretDelimiter(assetName) {
		return ""
	}
	return assetName
}

func containsSecretDelimiter(value string) bool {
	return strings.ContainsAny(value, "?&=#") || strings.Contains(value, "://")
}

func validPositiveID(id string) bool {
	parsed, err := strconv.ParseInt(id, 10, 64)
	return err == nil && parsed > 0 && strconv.FormatInt(parsed, 10) == id
}

func positiveJSONID(id json.Number) (string, error) {
	parsed, err := strconv.ParseInt(id.String(), 10, 64)
	if err != nil || parsed <= 0 {
		return "", errMetadata
	}
	return strconv.FormatInt(parsed, 10), nil
}

func parseOptionalTime(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, errMetadata
	}
	return parsed, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); errors.Is(err, io.EOF) {
		return nil
	}
	return errMetadata
}

func verifyDeclaredDigest(declaredDigest string, checksum string) error {
	declaredDigest = strings.TrimSpace(declaredDigest)
	if declaredDigest == "" {
		return nil
	}
	algorithm, digest, ok := strings.Cut(declaredDigest, ":")
	if !ok || !strings.EqualFold(algorithm, "sha256") || !hexPattern.MatchString(digest) ||
		!strings.EqualFold(digest, checksum) {
		return errDigest
	}
	return nil
}

func safeAssetNames(assets []Asset) []string {
	count := len(assets)
	if count > maxAssetErrorNames {
		count = maxAssetErrorNames
	}
	names := make([]string, 0, count)
	for _, asset := range assets[:count] {
		name := safeText(asset.Name, maxSafeAssetNameLen)
		if containsSecretDelimiter(name) {
			name = "<redacted>"
		}
		names = append(names, name)
	}
	return names
}

func safeText(value string, maxBytes int) string {
	var builder strings.Builder
	for _, character := range value {
		if isLogControl(character) {
			builder.WriteByte('?')
			continue
		}
		builder.WriteRune(character)
		if builder.Len() >= maxBytes {
			break
		}
	}
	result := builder.String()
	for len(result) > maxBytes {
		_, size := utf8.DecodeLastRuneInString(result)
		result = result[:len(result)-size]
	}
	return result
}

func safeETag(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > maxETagBytes || safeText(value, maxETagBytes) != value {
		return ""
	}
	return value
}

func safeHTTPError(kind error, response *http.Response, repository string, tag string, assetName string, retryAt *time.Time) error {
	return safeError(
		kind,
		response.StatusCode,
		response.Header.Get("X-GitHub-Request-Id"),
		repository,
		tag,
		assetName,
		nil,
		retryAt,
	)
}

func safeError(
	kind error,
	statusCode int,
	requestID string,
	repository string,
	tag string,
	assetName string,
	availableAssets []string,
	retryAt *time.Time,
) error {
	return &Error{
		Kind:            kind,
		StatusCode:      statusCode,
		RequestID:       safeErrorToken(requestID, maxSafeTextBytes),
		Repository:      safeErrorToken(repository, maxSafeTextBytes),
		Tag:             safeErrorToken(tag, maxSafeTextBytes),
		AssetName:       safeErrorToken(assetName, maxSafeAssetNameLen),
		AvailableAssets: availableAssets,
		RetryAt:         retryAt,
	}
}

func safeErrorToken(value string, maxBytes int) string {
	if !validLogText(value, maxBytes, true) {
		return ""
	}
	value = safeText(value, maxBytes)
	if containsSecretDelimiter(value) {
		return ""
	}
	return value
}
