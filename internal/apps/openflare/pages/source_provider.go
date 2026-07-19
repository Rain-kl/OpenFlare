// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pages

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/pkg/httppool"
	"github.com/Rain-kl/Wavelet/pkg/pagesarchive"
)

const (
	remoteSourceDownloadTimeout       = 10 * time.Minute
	remoteSourceResponseHeaderTimeout = 30 * time.Second
	remoteSourceDialTimeout           = 30 * time.Second
	remoteSourceDialKeepAlive         = 30 * time.Second
	remoteSourceMaxRedirects          = 5
	remoteSourceMagicSniffBytes       = 512
	remoteSourceMaxSafeLabelBytes     = 255
	remoteSourceFallbackLabel         = "package"
	remoteSourceUserAgent             = "OpenFlare Pages Source/2"
	remoteSourceSchemeHTTP            = "http"
	remoteSourceSchemeHTTPS           = "https"
)

type remoteProviderError string

func (providerError remoteProviderError) Error() string {
	return string(providerError)
}

	const (
	errRemoteProviderInvalidLimit  remoteProviderError = "远程来源部署包大小限制无效"
	errRemoteProviderRedirectLimit remoteProviderError = "远程来源重定向次数超过限制"
	errRemoteProviderDownloadFailed remoteProviderError = errPagesPackageURLDownloadFailed
	errRemoteProviderTooLarge       remoteProviderError = errPagesPackageURLTooLarge
	errRemoteProviderEmpty          remoteProviderError = errPagesPackageEmpty
	errRemoteProviderUnsupported    remoteProviderError = errPagesPackageUnsupported
	errRemoteProviderCleanupFailed  remoteProviderError = "清理远程来源临时文件失败"
)

// RemoteSourceRequest describes one immutable Remote URL package fetch.
type RemoteSourceRequest struct {
	URL             string
	AllowInsecure   bool
	MaxPackageBytes int64
}

// SourceCandidate is a constrained, immutable archive downloaded to a
// provider-owned temporary file. The caller owns the file after a successful
// fetch and must call Cleanup when processing finishes.
type SourceCandidate struct {
	TempPath    string
	Checksum    string
	PackageSize int64
	Format      pagesarchive.Format
	SafeLabel   string
}

// Cleanup removes the candidate temporary file. It is safe to call repeatedly.
func (candidate *SourceCandidate) Cleanup() error {
	if candidate == nil || candidate.TempPath == "" {
		return nil
	}
	tempPath := candidate.TempPath
	err := os.Remove(tempPath)
	if err == nil || errors.Is(err, os.ErrNotExist) {
		candidate.TempPath = ""
		return nil
	}
	return errRemoteProviderCleanupFailed
}

type remoteSourceResolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}

type remoteSourceDependencies struct {
	resolver    remoteSourceResolver
	dialContext func(context.Context, string, string) (net.Conn, error)
	createTemp  func(string, string) (*os.File, error)
}

// FetchRemoteSource downloads a Remote URL package without writing deployment
// state. Errors are reduced to safe domain messages and never contain the raw
// URL, query, response headers or response body.
func FetchRemoteSource(ctx context.Context, request RemoteSourceRequest) (*SourceCandidate, error) {
	dialer := &net.Dialer{
		Timeout:   remoteSourceDialTimeout,
		KeepAlive: remoteSourceDialKeepAlive,
	}
	dependencies := remoteSourceDependencies{
		resolver:    net.DefaultResolver,
		dialContext: dialer.DialContext,
		createTemp:  os.CreateTemp,
	}
	return fetchRemoteSource(ctx, request, dependencies)
}

func fetchRemoteSource(ctx context.Context, request RemoteSourceRequest, dependencies remoteSourceDependencies) (*SourceCandidate, error) {
	if request.MaxPackageBytes <= 0 {
		return nil, errRemoteProviderInvalidLimit
	}
	if dependencies.dialContext == nil || dependencies.createTemp == nil {
		return nil, errRemoteProviderDownloadFailed
	}
	parsed, err := parseRemoteSourceURL(request.URL)
	if err != nil {
		return nil, err
	}
	if err := validateRemoteSourceTarget(ctx, parsed); err != nil {
		return nil, sanitizeRemoteProviderError(ctx, err)
	}

	safeLabel, namedFormat := remoteSourceLabel(parsed)
	client := newRemoteSourceClient(request.AllowInsecure, dependencies)
	defer client.CloseIdleConnections()
	response, err := requestRemoteSource(ctx, client, parsed)
	if err != nil {
		return nil, err
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("%w: HTTP %d", errRemoteProviderDownloadFailed, response.StatusCode)
	}
	if response.ContentLength > request.MaxPackageBytes {
		return nil, errRemoteProviderTooLarge
	}

	tempPath, checksum, packageSize, err := streamRemoteSourcePackage(
		response.Body,
		request.MaxPackageBytes,
		dependencies.createTemp,
	)
	if err != nil {
		return nil, sanitizeRemoteProviderError(ctx, err)
	}
	format, safeLabel, err := detectRemoteSourceFormat(tempPath, safeLabel, namedFormat)
	if err != nil {
		if removeErr := os.Remove(tempPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return nil, errRemoteProviderCleanupFailed
		}
		return nil, err
	}

	return &SourceCandidate{
		TempPath:    tempPath,
		Checksum:    checksum,
		PackageSize: packageSize,
		Format:      format,
		SafeLabel:   safeLabel,
	}, nil
}

func newRemoteSourceClient(allowInsecure bool, dependencies remoteSourceDependencies) *http.Client {
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if allowInsecure {
		// Explicit administrator choice for self-signed or private CA endpoints.
		tlsConfig.InsecureSkipVerify = true //nolint:gosec // required allow_insecure semantics
	}

	client := &http.Client{
		Timeout: remoteSourceDownloadTimeout,
		Transport: httppool.NewTransport(httppool.TransportOptions{
			Proxy:                 nil,
			DialContext:           dependencies.dialContext,
			TLSClientConfig:       tlsConfig,
			ResponseHeaderTimeout: remoteSourceResponseHeaderTimeout,
			TraceFilter:           remoteSourceTraceFilter,
		}),
	}
	client.CheckRedirect = func(next *http.Request, previous []*http.Request) error {
		if len(previous) > remoteSourceMaxRedirects {
			return errRemoteProviderRedirectLimit
		}
		stripRemoteSourceRedirectHeaders(next)
		if err := validateRemoteSourceTarget(next.Context(), next.URL); err != nil {
			return err
		}
		applyRemoteSourceHeaders(next)
		return nil
	}
	return client
}

func requestRemoteSource(ctx context.Context, client *http.Client, parsed *url.URL) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, errors.New(errPagesSourceRemoteURLInvalid)
	}
	applyRemoteSourceHeaders(request)
	response, err := client.Do(request) //nolint:gosec // scheme and every dial target are validated above
	if err == nil {
		return response, nil
	}
	if response != nil && response.Body != nil {
		_ = response.Body.Close()
	}
	return nil, sanitizeRemoteProviderError(ctx, err)
}

func applyRemoteSourceHeaders(request *http.Request) {
	request.Header.Set("User-Agent", remoteSourceUserAgent)
	request.Header.Set("Accept", "application/octet-stream,application/zip,application/x-tar,application/gzip,*/*;q=0.1")
	// Preserve the artifact bytes exactly as stored. Automatic HTTP gzip
	// decompression would change the checksum, size and archive format.
	request.Header.Set("Accept-Encoding", "identity")
}

func stripRemoteSourceRedirectHeaders(request *http.Request) {
	request.Header.Del("Authorization")
	request.Header.Del("Cookie")
	request.Header.Del("Proxy-Authorization")
	request.Header.Del("Referer")
}

func remoteSourceTraceFilter(request *http.Request) bool {
	// otelhttp records url.full. Signed query strings must never enter traces.
	return request.URL == nil || request.URL.RawQuery == ""
}

func validateRemoteSourceTarget(_ context.Context, target *url.URL) error {
	if target == nil || target.User != nil || target.Fragment != "" || target.Opaque != "" {
		return errors.New(errPagesSourceRemoteURLInvalid)
	}
	scheme := strings.ToLower(strings.TrimSpace(target.Scheme))
	if (scheme != remoteSourceSchemeHTTP && scheme != remoteSourceSchemeHTTPS) || strings.TrimSpace(target.Hostname()) == "" {
		return errors.New(errPagesSourceRemoteURLInvalid)
	}
	return nil
}

func streamRemoteSourcePackage(
	body io.Reader,
	maxPackageBytes int64,
	createTemp func(string, string) (*os.File, error),
) (tempPath string, checksum string, packageSize int64, err error) {
	if createTemp == nil {
		return "", "", 0, errRemoteProviderDownloadFailed
	}
	tempFile, err := createTemp("", "openflare-pages-source-*")
	if err != nil {
		return "", "", 0, errRemoteProviderDownloadFailed
	}
	createdTempPath := tempFile.Name()
	tempPath = createdTempPath
	defer func() {
		closeErr := tempFile.Close()
		if err == nil && closeErr != nil {
			err = errRemoteProviderDownloadFailed
		}
		if err != nil {
			if removeErr := os.Remove(createdTempPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				err = errRemoteProviderCleanupFailed
			}
		}
	}()

	hasher := sha256.New()
	readLimit := maxPackageBytes
	if readLimit < math.MaxInt64 {
		readLimit++
	}
	packageSize, err = io.Copy(io.MultiWriter(tempFile, hasher), io.LimitReader(body, readLimit))
	if err != nil {
		return "", "", 0, errRemoteProviderDownloadFailed
	}
	if packageSize > maxPackageBytes {
		return "", "", 0, errRemoteProviderTooLarge
	}
	if packageSize == 0 {
		return "", "", 0, errRemoteProviderEmpty
	}
	checksum = hex.EncodeToString(hasher.Sum(nil))
	return tempPath, checksum, packageSize, nil
}

func detectRemoteSourceFormat(
	tempPath string,
	safeLabel string,
	namedFormat pagesarchive.Format,
) (pagesarchive.Format, string, error) {
	if namedFormat != "" {
		return namedFormat, safeLabel, nil
	}
	tempFile, err := os.Open(tempPath) //nolint:gosec // path is a provider-created temporary file
	if err != nil {
		return "", safeLabel, errRemoteProviderDownloadFailed
	}
	defer func() { _ = tempFile.Close() }()

	head := make([]byte, remoteSourceMagicSniffBytes)
	readBytes, readErr := io.ReadFull(tempFile, head)
	if readErr != nil && !errors.Is(readErr, io.EOF) && !errors.Is(readErr, io.ErrUnexpectedEOF) {
		return "", safeLabel, errRemoteProviderDownloadFailed
	}
	format, ok := pagesarchive.DetectFormatFromBytes(head[:readBytes])
	if !ok {
		return "", safeLabel, errRemoteProviderUnsupported
	}
	return format, appendRemoteSourceLabelExtension(safeLabel, format), nil
}

func remoteSourceLabel(parsed *url.URL) (string, pagesarchive.Format) {
	baseName := path.Base(parsed.Path)
	if baseName == "" || baseName == "." || baseName == "/" {
		baseName = remoteSourceFallbackLabel
	}
	safeLabel := sanitizeRemoteSourceLabel(baseName)
	format, _ := pagesarchive.DetectFormatFromName(safeLabel)
	return limitRemoteSourceLabel(safeLabel, format), format
}

func sanitizeRemoteSourceLabel(label string) string {
	var builder strings.Builder
	lastReplacement := false
	for _, character := range label {
		if isRemoteSourceLabelCharacter(character) {
			builder.WriteRune(character)
			lastReplacement = false
			continue
		}
		if !lastReplacement {
			builder.WriteByte('-')
			lastReplacement = true
		}
	}
	safeLabel := strings.TrimSpace(builder.String())
	if safeLabel == "" || strings.Trim(safeLabel, "._-") == "" {
		return remoteSourceFallbackLabel
	}
	return safeLabel
}

func isRemoteSourceLabelCharacter(character rune) bool {
	return character >= 'a' && character <= 'z' ||
		character >= 'A' && character <= 'Z' ||
		character >= '0' && character <= '9' ||
		character == '.' || character == '-' || character == '_'
}

func limitRemoteSourceLabel(label string, format pagesarchive.Format) string {
	if len(label) <= remoteSourceMaxSafeLabelBytes {
		return label
	}
	if format == "" {
		return strings.TrimRight(label[:remoteSourceMaxSafeLabelBytes], ".-_")
	}
	extension := "." + pagesarchive.Extension(format)
	prefixLength := remoteSourceMaxSafeLabelBytes - len(extension)
	prefix := strings.TrimRight(label[:prefixLength], ".-_")
	if prefix == "" {
		prefix = remoteSourceFallbackLabel
	}
	return prefix + extension
}

func appendRemoteSourceLabelExtension(label string, format pagesarchive.Format) string {
	extension := "." + pagesarchive.Extension(format)
	maxPrefixLength := remoteSourceMaxSafeLabelBytes - len(extension)
	if len(label) > maxPrefixLength {
		label = strings.TrimRight(label[:maxPrefixLength], ".-_")
	}
	if label == "" {
		label = remoteSourceFallbackLabel
	}
	return label + extension
}

func sanitizeRemoteProviderError(ctx context.Context, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return fmt.Errorf("%w: %w", errRemoteProviderDownloadFailed, ctxErr)
	}
	for _, safeError := range []error{
		errRemoteProviderInvalidLimit,
		errRemoteProviderRedirectLimit,
		errRemoteProviderTooLarge,
		errRemoteProviderEmpty,
		errRemoteProviderUnsupported,
		errRemoteProviderCleanupFailed,
	} {
		if errors.Is(err, safeError) {
			return safeError
		}
	}
	return errRemoteProviderDownloadFailed
}
