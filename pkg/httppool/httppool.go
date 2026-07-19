// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package httppool manages shared, optimized HTTP transports to reuse TCP connections.
package httppool

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

const (
	dialTimeout           = 30 * time.Second
	dialKeepAlive         = 30 * time.Second
	maxIdleConns          = 200
	maxIdleConnsPerHost   = 32
	idleConnTimeout       = 90 * time.Second
	tlsHandshakeTimeout   = 10 * time.Second
	expectContinueTimeout = 1 * time.Second
	tlsSessionCacheSize   = 100
)

var (
	defaultTransport http.RoundTripper
	once             sync.Once
)

// TransportOptions configures the request-specific parts of a pooled HTTP
// transport. Pool sizes and timeout defaults remain managed by this package.
// A nil Proxy explicitly disables proxy use.
type TransportOptions struct {
	Proxy                 func(*http.Request) (*url.URL, error)
	DialContext           func(context.Context, string, string) (net.Conn, error)
	TLSClientConfig       *tls.Config
	ResponseHeaderTimeout time.Duration
	TraceFilter           func(*http.Request) bool
}

// NewTransport returns an independently configurable pooled transport wrapped
// with OTel instrumentation. The supplied TLS configuration is cloned before
// use so later caller mutations cannot change an active transport.
func NewTransport(options TransportOptions) http.RoundTripper {
	dialContext := options.DialContext
	if dialContext == nil {
		dialContext = (&net.Dialer{
			Timeout:   dialTimeout,
			KeepAlive: dialKeepAlive,
		}).DialContext
	}

	tlsConfig := options.TLSClientConfig
	if tlsConfig == nil {
		tlsConfig = &tls.Config{}
	} else {
		tlsConfig = tlsConfig.Clone()
	}
	if tlsConfig.ClientSessionCache == nil {
		tlsConfig.ClientSessionCache = tls.NewLRUClientSessionCache(tlsSessionCacheSize)
	}

	transport := &http.Transport{
		Proxy:                 options.Proxy,
		DialContext:           dialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          maxIdleConns,
		MaxIdleConnsPerHost:   maxIdleConnsPerHost,
		IdleConnTimeout:       idleConnTimeout,
		TLSHandshakeTimeout:   tlsHandshakeTimeout,
		ResponseHeaderTimeout: options.ResponseHeaderTimeout,
		ExpectContinueTimeout: expectContinueTimeout,
		TLSClientConfig:       tlsConfig,
	}
	otelOptions := make([]otelhttp.Option, 0, 1)
	if options.TraceFilter != nil {
		otelOptions = append(otelOptions, otelhttp.WithFilter(options.TraceFilter))
	}
	return otelhttp.NewTransport(transport, otelOptions...)
}

// DefaultTransport returns a globally shared, optimized http.RoundTripper
// with OTel instrumentation. It maintains a pool of idle TCP connections
// across hosts.
func DefaultTransport() http.RoundTripper {
	once.Do(func() {
		defaultTransport = NewTransport(TransportOptions{
			Proxy: http.ProxyFromEnvironment,
		})
	})
	return defaultTransport
}

// NewClient returns a new http.Client that shares the global connection pool
// but has its own timeout configuration.
func NewClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: DefaultTransport(),
	}
}
