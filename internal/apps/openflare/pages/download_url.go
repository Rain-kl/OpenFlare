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
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/pkg/pagesarchive"
)

const (
	pagesURLDownloadTimeout   = 10 * time.Minute
	pagesURLMaxRedirects      = 5
	pagesMagicSniffBytes      = 512
	pagesURLDialTimeout       = 30 * time.Second
	pagesURLTLSHandshake      = 15 * time.Second
	pagesBrowserUserAgent     = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
	pagesBrowserAccept        = "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7"
	pagesBrowserAcceptLang    = "zh-CN,zh;q=0.9,en-US;q=0.8,en;q=0.7"
	pagesBrowserSecCHUA       = `"Google Chrome";v="131", "Chromium";v="131", "Not_A Brand";v="24"`
	pagesBrowserSecCHUAMobile = "?0"
	pagesBrowserSecCHUAPlat   = `"Windows"`
)

// downloadPagesPackageFromURL fetches a remote archive with browser-like headers
// and writes it to a temp file. Allows private/LAN hosts and insecure TLS certs
// (self-signed / internal CA) so operators can pull from internal artifact stores.
func downloadPagesPackageFromURL(ctx context.Context, rawURL string, maxPackageBytes int64) (tempPath string, checksum string, size int64, format pagesarchive.Format, fileName string, err error) {
	parsed, err := parseAndValidatePagesDownloadURL(rawURL)
	if err != nil {
		return "", "", 0, "", "", err
	}

	resp, err := doBrowserDownload(ctx, parsed)
	if err != nil {
		return "", "", 0, "", "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", 0, "", "", fmt.Errorf("%s: HTTP %d", errPagesPackageURLDownloadFailed, resp.StatusCode)
	}
	if resp.ContentLength > 0 && resp.ContentLength > maxPackageBytes {
		return "", "", 0, "", "", errors.New(errPagesPackageURLTooLarge)
	}

	fileName = fileNameFromDownload(resp, parsed)
	format, _ = pagesarchive.DetectFormatFromName(fileName)

	tempPath, checksum, size, err = writeLimitedPackageTemp(resp.Body, format, maxPackageBytes)
	if err != nil {
		return "", "", 0, "", "", err
	}
	format, fileName, err = ensurePackageFormat(tempPath, format, fileName)
	if err != nil {
		_ = os.Remove(tempPath)
		return "", "", 0, "", "", err
	}
	return tempPath, checksum, size, format, fileName, nil
}

func newPagesURLDownloadClient() *http.Client {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   pagesURLDialTimeout,
			KeepAlive: pagesURLDialTimeout,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          32,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   pagesURLTLSHandshake,
		ExpectContinueTimeout: time.Second,
		// Allow self-signed / internal certificates for artifact hosts.
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // intentional for internal/dev artifact URLs
	}
	client := &http.Client{
		Timeout:   pagesURLDownloadTimeout,
		Transport: transport,
	}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= pagesURLMaxRedirects {
			return errors.New(errPagesPackageURLDownloadFailed)
		}
		if err := validatePagesDownloadURLValue(req.URL); err != nil {
			return err
		}
		applyBrowserDownloadHeaders(req, via[0].URL.String())
		return nil
	}
	return client
}

func doBrowserDownload(ctx context.Context, parsed *url.URL) (*http.Response, error) {
	client := newPagesURLDownloadClient()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, errors.New(errPagesPackageURLInvalid)
	}
	applyBrowserDownloadHeaders(req, "")
	resp, err := client.Do(req) //nolint:gosec // scheme validated; private hosts and insecure TLS intentionally allowed
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errPagesPackageURLDownloadFailed, err)
	}
	return resp, nil
}

func writeLimitedPackageTemp(body io.Reader, format pagesarchive.Format, maxPackageBytes int64) (tempPath, checksum string, size int64, err error) {
	temp, err := os.CreateTemp("", "openflare-pages-url-*."+safeTempSuffixOrBin(format))
	if err != nil {
		return "", "", 0, err
	}
	tempPath = temp.Name()
	defer func() {
		_ = temp.Close()
		if err != nil {
			_ = os.Remove(tempPath)
		}
	}()

	hash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(temp, hash), io.LimitReader(body, maxPackageBytes+1))
	if copyErr != nil {
		err = fmt.Errorf("%s: %w", errPagesPackageURLDownloadFailed, copyErr)
		return "", "", 0, err
	}
	if written > maxPackageBytes {
		err = errors.New(errPagesPackageURLTooLarge)
		return "", "", 0, err
	}
	if written == 0 {
		err = errors.New(errPagesPackageEmpty)
		return "", "", 0, err
	}
	return tempPath, hex.EncodeToString(hash.Sum(nil)), written, nil
}

func ensurePackageFormat(tempPath string, format pagesarchive.Format, fileName string) (pagesarchive.Format, string, error) {
	if format != "" {
		return format, fileName, nil
	}
	detected, ok := sniffPackageFormat(tempPath)
	if !ok {
		return "", fileName, errors.New(errPagesPackageUnsupported)
	}
	if !strings.Contains(strings.ToLower(fileName), ".") {
		fileName = fileName + "." + pagesarchive.Extension(detected)
	}
	return detected, fileName, nil
}

func sniffPackageFormat(tempPath string) (pagesarchive.Format, bool) {
	file, err := os.Open(tempPath) //nolint:gosec // temp path created by us
	if err != nil {
		return "", false
	}
	defer func() { _ = file.Close() }()
	head := make([]byte, pagesMagicSniffBytes)
	n, _ := io.ReadFull(file, head)
	if n <= 0 {
		return "", false
	}
	return pagesarchive.DetectFormatFromBytes(head[:n])
}

func safeTempSuffixOrBin(format pagesarchive.Format) string {
	if format == "" {
		return "bin"
	}
	return safeTempSuffix(format)
}

func applyBrowserDownloadHeaders(req *http.Request, referer string) {
	if req == nil {
		return
	}
	req.Header.Set("User-Agent", pagesBrowserUserAgent)
	req.Header.Set("Accept", pagesBrowserAccept)
	req.Header.Set("Accept-Language", pagesBrowserAcceptLang)
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Sec-Ch-Ua", pagesBrowserSecCHUA)
	req.Header.Set("Sec-Ch-Ua-Mobile", pagesBrowserSecCHUAMobile)
	req.Header.Set("Sec-Ch-Ua-Platform", pagesBrowserSecCHUAPlat)
	if referer != "" {
		req.Header.Set("Referer", referer)
		req.Header.Set("Sec-Fetch-Site", "cross-site")
		return
	}
	if req.URL != nil {
		req.Header.Set("Referer", req.URL.Scheme+"://"+req.URL.Host+"/")
	}
}

func parseAndValidatePagesDownloadURL(raw string) (*url.URL, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, errors.New(errPagesPackageURLRequired)
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return nil, errors.New(errPagesPackageURLInvalid)
	}
	if err := validatePagesDownloadURLValue(parsed); err != nil {
		return nil, err
	}
	return parsed, nil
}

func validatePagesDownloadURLValue(parsed *url.URL) error {
	if parsed == nil {
		return errors.New(errPagesPackageURLInvalid)
	}
	scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
	if scheme != "http" && scheme != "https" {
		return errors.New(errPagesPackageURLInvalid)
	}
	if strings.TrimSpace(parsed.Hostname()) == "" {
		return errors.New(errPagesPackageURLInvalid)
	}
	return nil
}

func fileNameFromDownload(resp *http.Response, parsed *url.URL) string {
	if name := fileNameFromContentDisposition(resp); name != "" {
		return name
	}
	if parsed != nil {
		base := path.Base(parsed.Path)
		if base != "" && base != "." && base != "/" {
			return base
		}
	}
	return "package.bin"
}

func fileNameFromContentDisposition(resp *http.Response) string {
	if resp == nil {
		return ""
	}
	cd := resp.Header.Get("Content-Disposition")
	if cd == "" {
		return ""
	}
	_, params, err := mime.ParseMediaType(cd)
	if err != nil {
		return ""
	}
	name := strings.TrimSpace(params["filename"])
	if name == "" {
		return ""
	}
	return path.Base(filepath.ToSlash(name))
}
