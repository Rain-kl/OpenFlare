// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package httpclient

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDownloadPagesProjectLatestPackageRejectsChunkedBodyOverLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		_, _ = io.WriteString(w, "123456")
	}))
	defer server.Close()

	client := New(server.URL, "test-token", time.Second)
	var dst bytes.Buffer
	written, err := client.DownloadPagesProjectLatestPackage(
		context.Background(),
		7,
		&dst,
		4,
	)
	if err == nil || !strings.Contains(err.Error(), "body exceeds limit") {
		t.Fatalf("DownloadPagesProjectLatestPackage(chunked, limit=4) error = %v, want body limit error", err)
	}
	if written != 5 {
		t.Errorf("DownloadPagesProjectLatestPackage(chunked, limit=4) written = %d, want 5", written)
	}
}

func TestCopyPagesPackageResponseRejectsAdvertisedContentLengthBeforeWrite(t *testing.T) {
	response := &http.Response{
		Body:          io.NopCloser(strings.NewReader("123456")),
		ContentLength: 6,
	}
	var dst bytes.Buffer
	written, err := copyPagesPackageResponse(&dst, response, 4)
	if err == nil || !strings.Contains(err.Error(), "Content-Length") {
		t.Fatalf("copyPagesPackageResponse(Content-Length=6, limit=4) error = %v, want Content-Length limit error", err)
	}
	if written != 0 || dst.Len() != 0 {
		t.Errorf("copyPagesPackageResponse(Content-Length=6, limit=4) wrote (%d, %d buffered), want no writes", written, dst.Len())
	}
}

func TestCopyPagesPackageResponseRejectsForgedSmallContentLength(t *testing.T) {
	response := &http.Response{
		Body:          io.NopCloser(strings.NewReader("123456")),
		ContentLength: 2,
	}
	var dst bytes.Buffer
	written, err := copyPagesPackageResponse(&dst, response, 4)
	if err == nil || !strings.Contains(err.Error(), "body exceeds limit") {
		t.Fatalf("copyPagesPackageResponse(forged Content-Length=2, limit=4) error = %v, want body limit error", err)
	}
	if written != 5 {
		t.Errorf("copyPagesPackageResponse(forged Content-Length=2, limit=4) written = %d, want 5", written)
	}
}

func TestDownloadPagesProjectLatestPackageBoundsChunkedErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		_, _ = io.WriteString(w, strings.Repeat("x", int(pagesControlResponseMaxBytes+1)))
	}))
	defer server.Close()

	client := New(server.URL, "test-token", time.Second)
	var dst bytes.Buffer
	_, err := client.DownloadPagesProjectLatestPackage(context.Background(), 7, &dst, 1024)
	if err == nil || !strings.Contains(err.Error(), "control response body exceeds limit") {
		t.Fatalf("DownloadPagesProjectLatestPackage(large chunked 400) error = %v, want bounded response error", err)
	}
	if dst.Len() != 0 {
		t.Errorf("DownloadPagesProjectLatestPackage(large chunked 400) wrote %d package bytes, want 0", dst.Len())
	}
}

func TestGetPagesProjectLatestHashBoundsChunkedMetadataResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		_, _ = io.WriteString(w, strings.Repeat("x", int(pagesControlResponseMaxBytes+1)))
	}))
	defer server.Close()

	client := New(server.URL, "test-token", time.Second)
	_, err := client.GetPagesProjectLatestHash(context.Background(), 7)
	if err == nil || !strings.Contains(err.Error(), "control response body exceeds limit") {
		t.Fatalf("GetPagesProjectLatestHash(large chunked metadata) error = %v, want bounded response error", err)
	}
}
