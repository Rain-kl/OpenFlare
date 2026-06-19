// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package httppool manages shared, optimized HTTP transports to reuse TCP connections.
package httppool

import (
	"crypto/tls"
	"net"
	"net/http"
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

// DefaultTransport returns a globally shared, optimized http.RoundTripper
// with OTel instrumentation. It maintains a pool of idle TCP connections
// across hosts.
func DefaultTransport() http.RoundTripper {
	once.Do(func() {
		transport := &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   dialTimeout,
				KeepAlive: dialKeepAlive,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          maxIdleConns,
			MaxIdleConnsPerHost:   maxIdleConnsPerHost,
			IdleConnTimeout:       idleConnTimeout,
			TLSHandshakeTimeout:   tlsHandshakeTimeout,
			ExpectContinueTimeout: expectContinueTimeout,
			TLSClientConfig: &tls.Config{
				ClientSessionCache: tls.NewLRUClientSessionCache(tlsSessionCacheSize),
			},
		}
		defaultTransport = otelhttp.NewTransport(transport)
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
