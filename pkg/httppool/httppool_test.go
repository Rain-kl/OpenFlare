// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package httppool

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDefaultTransport(t *testing.T) {
	tr1 := DefaultTransport()
	if tr1 == nil {
		t.Fatal("DefaultTransport() returned nil")
	}

	tr2 := DefaultTransport()
	if tr1 != tr2 {
		t.Error("DefaultTransport() did not return a singleton instance")
	}
}

func TestNewClient(t *testing.T) {
	timeout := 15 * time.Second
	client := NewClient(timeout)
	if client == nil {
		t.Fatal("NewClient() returned nil")
	}

	if client.Timeout != timeout {
		t.Errorf("NewClient() timeout = %v, want %v", client.Timeout, timeout)
	}

	if client.Transport != DefaultTransport() {
		t.Error("NewClient() is not configured with the default transport")
	}
}

func TestNewTransportUsesConfiguredDirectDialer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("ok"))
	}))
	t.Cleanup(server.Close)

	var dialedAddress string
	dialer := &net.Dialer{}
	transport := NewTransport(TransportOptions{
		Proxy: nil,
		DialContext: func(ctx context.Context, network string, address string) (net.Conn, error) {
			dialedAddress = address
			return dialer.DialContext(ctx, network, server.Listener.Addr().String())
		},
	})
	client := &http.Client{Transport: transport}
	t.Cleanup(client.CloseIdleConnections)

	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://artifact.example/site.zip", nil)
	if err != nil {
		t.Fatalf("NewRequestWithContext() error = %v", err)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("client.Do() error = %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	if _, err := io.ReadAll(response.Body); err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if dialedAddress != "artifact.example:80" {
		t.Fatalf("DialContext address = %q, want direct target", dialedAddress)
	}
}

func TestNewTransportClonesTLSConfig(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	tlsConfig := &tls.Config{InsecureSkipVerify: true} //nolint:gosec // test-only self-signed server
	client := &http.Client{Transport: NewTransport(TransportOptions{TLSClientConfig: tlsConfig})}
	t.Cleanup(client.CloseIdleConnections)
	tlsConfig.InsecureSkipVerify = false

	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("NewRequestWithContext() error = %v", err)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("client.Do() error = %v", err)
	}
	_ = response.Body.Close()
}
