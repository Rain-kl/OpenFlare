// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package githubrelease

import (
	"context"
	"crypto/tls"
	"errors"
	"math"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/pkg/httppool"
)

const (
	clientTimeout         = 10 * time.Minute
	dialTimeout           = 30 * time.Second
	dialKeepAlive         = 30 * time.Second
	responseHeaderTimeout = 30 * time.Second
	maxRedirects          = 5
	maxRetryAfterSeconds  = math.MaxInt64 / int64(time.Second)
)

var (
	errBlockedTarget = errors.New("GitHub Release 请求目标不是公网地址")
	errResolveTarget = errors.New("GitHub Release 请求目标解析失败")
	errRedirectLimit = errors.New("GitHub Release asset 重定向次数过多")

	publicIPv6Prefix  = netip.MustParsePrefix("2000::/3")
	nonPublicPrefixes = []netip.Prefix{
		netip.MustParsePrefix("0.0.0.0/8"),
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("100.64.0.0/10"),
		netip.MustParsePrefix("127.0.0.0/8"),
		netip.MustParsePrefix("169.254.0.0/16"),
		netip.MustParsePrefix("172.16.0.0/12"),
		netip.MustParsePrefix("192.0.0.0/24"),
		netip.MustParsePrefix("192.0.2.0/24"),
		netip.MustParsePrefix("192.168.0.0/16"),
		netip.MustParsePrefix("198.18.0.0/15"),
		netip.MustParsePrefix("198.51.100.0/24"),
		netip.MustParsePrefix("203.0.113.0/24"),
		netip.MustParsePrefix("224.0.0.0/4"),
		netip.MustParsePrefix("240.0.0.0/4"),
		netip.MustParsePrefix("::/128"),
		netip.MustParsePrefix("::1/128"),
		netip.MustParsePrefix("::ffff:0:0/96"),
		netip.MustParsePrefix("64:ff9b::/96"),
		netip.MustParsePrefix("100::/64"),
		netip.MustParsePrefix("2001:db8::/32"),
		netip.MustParsePrefix("fc00::/7"),
		netip.MustParsePrefix("fe80::/10"),
		netip.MustParsePrefix("ff00::/8"),
	}
)

type resolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}

type clientOptions struct {
	baseURL       string
	resolver      resolver
	dialContext   func(context.Context, string, string) (net.Conn, error)
	tlsConfig     *tls.Config
	allowHTTP     bool
	createTemp    func(string, string) (*os.File, error)
	now           func() time.Time
	clientTimeout time.Duration
}

func defaultClientOptions() clientOptions {
	dialer := &net.Dialer{Timeout: dialTimeout, KeepAlive: dialKeepAlive}
	return clientOptions{
		baseURL:       defaultAPIBaseURL,
		resolver:      net.DefaultResolver,
		dialContext:   dialer.DialContext,
		tlsConfig:     &tls.Config{MinVersion: tls.VersionTLS12},
		createTemp:    os.CreateTemp,
		now:           time.Now,
		clientTimeout: clientTimeout,
	}
}

func newClient(options clientOptions) *Client {
	if options.baseURL == "" {
		options.baseURL = defaultAPIBaseURL
	}
	if options.resolver == nil {
		options.resolver = net.DefaultResolver
	}
	if options.dialContext == nil {
		dialer := &net.Dialer{Timeout: dialTimeout, KeepAlive: dialKeepAlive}
		options.dialContext = dialer.DialContext
	}
	if options.tlsConfig == nil {
		options.tlsConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	if options.createTemp == nil {
		options.createTemp = os.CreateTemp
	}
	if options.now == nil {
		options.now = time.Now
	}
	if options.clientTimeout <= 0 {
		options.clientTimeout = clientTimeout
	}

	secureDial := publicDialer(options.resolver, options.dialContext)
	transport := httppool.NewTransport(httppool.TransportOptions{
		Proxy:                 nil,
		DialContext:           secureDial,
		TLSClientConfig:       options.tlsConfig,
		ResponseHeaderTimeout: responseHeaderTimeout,
		TraceFilter: func(request *http.Request) bool {
			return request.URL == nil || request.URL.RawQuery == ""
		},
	})
	httpClient := &http.Client{Timeout: options.clientTimeout, Transport: transport}
	httpClient.CheckRedirect = func(next *http.Request, previous []*http.Request) error {
		if len(previous) > maxRedirects {
			return errRedirectLimit
		}
		if err := validateTarget(next.Context(), next.URL, options.resolver, options.allowHTTP); err != nil {
			return err
		}
		if len(previous) > 0 && !sameHost(previous[len(previous)-1].URL, next.URL) {
			stripCrossHostHeaders(next)
		}
		return nil
	}
	return &Client{
		httpClient: httpClient,
		baseURL:    strings.TrimRight(options.baseURL, "/"),
		createTemp: options.createTemp,
		now:        options.now,
	}
}

func applyMetadataHeaders(request *http.Request, etag string) {
	request.Header.Set("Accept", metadataAccept)
	request.Header.Set("User-Agent", defaultUserAgent)
	request.Header.Set("X-GitHub-Api-Version", APIVersion)
	if etag = safeETag(etag); etag != "" {
		request.Header.Set("If-None-Match", etag)
	}
}

func applyAssetHeaders(request *http.Request) {
	request.Header.Set("Accept", assetAccept)
	request.Header.Set("Accept-Encoding", "identity")
	request.Header.Set("User-Agent", defaultUserAgent)
	request.Header.Set("X-GitHub-Api-Version", APIVersion)
}

func stripCrossHostHeaders(request *http.Request) {
	for _, header := range []string{
		"Authorization",
		"Cookie",
		"Proxy-Authorization",
		"Referer",
		"If-None-Match",
		"If-Modified-Since",
		"X-GitHub-Api-Version",
	} {
		request.Header.Del(header)
	}
}

func sameHost(left *url.URL, right *url.URL) bool {
	if left == nil || right == nil {
		return false
	}
	return strings.EqualFold(left.Hostname(), right.Hostname()) && effectivePort(left) == effectivePort(right)
}

func effectivePort(target *url.URL) string {
	if port := target.Port(); port != "" {
		return port
	}
	if strings.EqualFold(target.Scheme, "https") {
		return "443"
	}
	return "80"
}

func validateTarget(ctx context.Context, target *url.URL, targetResolver resolver, allowHTTP bool) error {
	if target == nil || target.User != nil || target.Fragment != "" || target.Opaque != "" || target.Hostname() == "" {
		return errBlockedTarget
	}
	isHTTPS := strings.EqualFold(target.Scheme, "https")
	isAllowedHTTP := allowHTTP && strings.EqualFold(target.Scheme, "http")
	if !isHTTPS && !isAllowedHTTP {
		return errBlockedTarget
	}
	_, err := resolvePublicIPs(ctx, targetResolver, target.Hostname())
	return err
}

func publicDialer(
	targetResolver resolver,
	directDial func(context.Context, string, string) (net.Conn, error),
) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network string, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, errResolveTarget
		}
		addresses, err := resolvePublicIPs(ctx, targetResolver, host)
		if err != nil {
			return nil, err
		}
		for _, resolved := range addresses {
			if !ipMatchesNetwork(resolved, network) {
				continue
			}
			connection, dialErr := directDial(ctx, network, net.JoinHostPort(resolved.String(), port))
			if dialErr == nil {
				return connection, nil
			}
		}
		return nil, errDownload
	}
}

func resolvePublicIPs(ctx context.Context, targetResolver resolver, host string) ([]netip.Addr, error) {
	if strings.Contains(host, "%") {
		return nil, errBlockedTarget
	}
	if literal, err := netip.ParseAddr(host); err == nil {
		if !isPublicIP(literal) {
			return nil, errBlockedTarget
		}
		return []netip.Addr{literal}, nil
	}
	if targetResolver == nil {
		return nil, errResolveTarget
	}
	addresses, err := targetResolver.LookupNetIP(ctx, "ip", host)
	if err != nil || len(addresses) == 0 {
		return nil, errResolveTarget
	}
	for _, address := range addresses {
		if !isPublicIP(address) {
			return nil, errBlockedTarget
		}
	}
	return addresses, nil
}

func isPublicIP(address netip.Addr) bool {
	if !address.IsValid() || address.Zone() != "" {
		return false
	}
	address = address.Unmap()
	if !address.IsGlobalUnicast() {
		return false
	}
	if address.Is6() && !publicIPv6Prefix.Contains(address) {
		return false
	}
	for _, prefix := range nonPublicPrefixes {
		if prefix.Contains(address) {
			return false
		}
	}
	return true
}

func ipMatchesNetwork(address netip.Addr, network string) bool {
	switch network {
	case "tcp4":
		return address.Unmap().Is4()
	case "tcp6":
		return address.Unmap().Is6()
	default:
		return true
	}
}

func responseRetryAt(response *http.Response, now time.Time) *time.Time {
	if response == nil {
		return nil
	}
	if retryAfter := strings.TrimSpace(response.Header.Get("Retry-After")); retryAfter != "" {
		if seconds, err := strconv.ParseInt(retryAfter, 10, 64); err == nil && seconds >= 0 && seconds <= maxRetryAfterSeconds {
			retryAt := now.Add(time.Duration(seconds) * time.Second)
			return &retryAt
		}
		if retryAt, err := http.ParseTime(retryAfter); err == nil {
			retryAt = retryAt.UTC()
			return &retryAt
		}
	}
	if strings.TrimSpace(response.Header.Get("X-RateLimit-Remaining")) != "0" {
		return nil
	}
	reset, err := strconv.ParseInt(strings.TrimSpace(response.Header.Get("X-RateLimit-Reset")), 10, 64)
	if err != nil || reset <= 0 {
		return nil
	}
	retryAt := time.Unix(reset, 0).UTC()
	return &retryAt
}
