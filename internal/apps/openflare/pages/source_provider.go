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
	// RemoteNetworkPolicyPublic only permits publicly routable targets and
	// performs DNS validation again for every connection.
	RemoteNetworkPolicyPublic = "public"
	// RemoteNetworkPolicyTrustedInternal permits private targets and self-signed
	// TLS certificates. It is an explicit administrator trust boundary.
	RemoteNetworkPolicyTrustedInternal = "trusted_internal"

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
	errRemoteProviderInvalidPolicy  remoteProviderError = "远程来源网络策略无效"
	errRemoteProviderInvalidLimit   remoteProviderError = "远程来源部署包大小限制无效"
	errRemoteProviderBlockedAddress remoteProviderError = "远程来源 public 策略禁止访问非公网地址"
	errRemoteProviderResolveFailed  remoteProviderError = "远程来源地址解析失败"
	errRemoteProviderRedirectLimit  remoteProviderError = "远程来源重定向次数超过限制"
	errRemoteProviderDownloadFailed remoteProviderError = errPagesPackageURLDownloadFailed
	errRemoteProviderTooLarge       remoteProviderError = errPagesPackageURLTooLarge
	errRemoteProviderEmpty          remoteProviderError = errPagesPackageEmpty
	errRemoteProviderUnsupported    remoteProviderError = errPagesPackageUnsupported
	errRemoteProviderCleanupFailed  remoteProviderError = "清理远程来源临时文件失败"
)

var remoteSourceNonPublicPrefixes = []netip.Prefix{
	// IPv4 special-use, private, link-local, documentation, multicast and
	// reserved ranges. A conservative deny list is intentional for SSRF safety.
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("169.254.0.0/16"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("224.0.0.0/4"),
	netip.MustParsePrefix("240.0.0.0/4"),
	// IPv6 protocol-assignment, documentation and transition ranges that are
	// not acceptable as direct public artifact origins.
	netip.MustParsePrefix("2001::/23"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("3fff::/20"),
}

var remoteSourcePublicIPv6Prefix = netip.MustParsePrefix("2000::/3")

// RemoteSourceRequest describes one immutable Remote URL package fetch.
type RemoteSourceRequest struct {
	URL             string
	NetworkPolicy   string
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
	policy, err := normalizeRemoteNetworkPolicy(request.NetworkPolicy)
	if err != nil {
		return nil, err
	}
	parsed, err := parseRemoteSourceURL(request.URL)
	if err != nil {
		return nil, err
	}
	if err := validateRemoteSourceTarget(ctx, parsed, policy, dependencies.resolver); err != nil {
		return nil, sanitizeRemoteProviderError(ctx, err)
	}

	safeLabel, namedFormat := remoteSourceLabel(parsed)
	client := newRemoteSourceClient(policy, dependencies)
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

func normalizeRemoteNetworkPolicy(policy string) (string, error) {
	switch strings.TrimSpace(policy) {
	case "", RemoteNetworkPolicyPublic:
		return RemoteNetworkPolicyPublic, nil
	case RemoteNetworkPolicyTrustedInternal:
		return RemoteNetworkPolicyTrustedInternal, nil
	default:
		return "", errRemoteProviderInvalidPolicy
	}
}

func newRemoteSourceClient(policy string, dependencies remoteSourceDependencies) *http.Client {
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	dialContext := dependencies.dialContext
	if policy == RemoteNetworkPolicyPublic {
		dialContext = newPublicRemoteSourceDialer(dependencies.resolver, dependencies.dialContext)
	} else {
		// trusted_internal is an explicit administrator-selected boundary for
		// private artifact services using an internal CA or self-signed cert.
		tlsConfig.InsecureSkipVerify = true //nolint:gosec // required trusted_internal semantics
	}

	client := &http.Client{
		Timeout: remoteSourceDownloadTimeout,
		Transport: httppool.NewTransport(httppool.TransportOptions{
			Proxy:                 nil,
			DialContext:           dialContext,
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
		if err := validateRemoteSourceTarget(next.Context(), next.URL, policy, dependencies.resolver); err != nil {
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

func validateRemoteSourceTarget(ctx context.Context, target *url.URL, policy string, resolver remoteSourceResolver) error {
	if target == nil || target.User != nil || target.Fragment != "" || target.Opaque != "" {
		return errors.New(errPagesSourceRemoteURLInvalid)
	}
	scheme := strings.ToLower(strings.TrimSpace(target.Scheme))
	if (scheme != remoteSourceSchemeHTTP && scheme != remoteSourceSchemeHTTPS) || strings.TrimSpace(target.Hostname()) == "" {
		return errors.New(errPagesSourceRemoteURLInvalid)
	}
	if policy != RemoteNetworkPolicyPublic {
		return nil
	}
	_, err := resolvePublicRemoteSourceIPs(ctx, resolver, target.Hostname())
	return err
}

func newPublicRemoteSourceDialer(
	resolver remoteSourceResolver,
	directDial func(context.Context, string, string) (net.Conn, error),
) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network string, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, errRemoteProviderDownloadFailed
		}
		addresses, err := resolvePublicRemoteSourceIPs(ctx, resolver, host)
		if err != nil {
			return nil, err
		}
		for _, address := range addresses {
			if !remoteSourceIPMatchesNetwork(address, network) {
				continue
			}
			connection, dialErr := directDial(ctx, network, net.JoinHostPort(address.String(), port))
			if dialErr == nil {
				return connection, nil
			}
		}
		return nil, errRemoteProviderDownloadFailed
	}
}

func resolvePublicRemoteSourceIPs(ctx context.Context, resolver remoteSourceResolver, host string) ([]netip.Addr, error) {
	if strings.Contains(host, "%") {
		return nil, errRemoteProviderBlockedAddress
	}
	if literal, parseErr := netip.ParseAddr(host); parseErr == nil {
		if !isPublicRemoteSourceIP(literal) {
			return nil, errRemoteProviderBlockedAddress
		}
		return []netip.Addr{literal}, nil
	}
	if resolver == nil {
		return nil, errRemoteProviderResolveFailed
	}
	addresses, err := resolver.LookupNetIP(ctx, "ip", host)
	if err != nil || len(addresses) == 0 {
		return nil, errRemoteProviderResolveFailed
	}
	for _, address := range addresses {
		if !isPublicRemoteSourceIP(address) {
			return nil, errRemoteProviderBlockedAddress
		}
	}
	return addresses, nil
}

func isPublicRemoteSourceIP(address netip.Addr) bool {
	if !address.IsValid() || address.Zone() != "" {
		return false
	}
	address = address.Unmap()
	if !address.IsGlobalUnicast() {
		return false
	}
	if address.Is6() && !remoteSourcePublicIPv6Prefix.Contains(address) {
		return false
	}
	for _, prefix := range remoteSourceNonPublicPrefixes {
		if prefix.Contains(address) {
			return false
		}
	}
	return true
}

func remoteSourceIPMatchesNetwork(address netip.Addr, network string) bool {
	switch network {
	case "tcp4":
		return address.Unmap().Is4()
	case "tcp6":
		return address.Unmap().Is6()
	default:
		return true
	}
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
		errRemoteProviderInvalidPolicy,
		errRemoteProviderInvalidLimit,
		errRemoteProviderBlockedAddress,
		errRemoteProviderResolveFailed,
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
