// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pages

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"github.com/Rain-kl/Wavelet/pkg/pagesarchive"
)

// downloadPagesPackageFromURL is the deprecated one-shot URL adapter. It uses
// the same bounded downloader as persisted sources and allows insecure TLS for
// operator-managed internal artifact services.
func downloadPagesPackageFromURL(
	ctx context.Context,
	rawURL string,
	maxPackageBytes int64,
) (tempPath string, checksum string, size int64, format pagesarchive.Format, fileName string, err error) {
	if _, err := parseAndValidatePagesDownloadURL(rawURL); err != nil {
		return "", "", 0, "", "", err
	}
	candidate, err := FetchRemoteSource(ctx, RemoteSourceRequest{
		URL:             strings.TrimSpace(rawURL),
		AllowInsecure:   true,
		MaxPackageBytes: maxPackageBytes,
	})
	if err != nil {
		if strings.Contains(err.Error(), errPagesSourceRemoteURLInvalid) {
			return "", "", 0, "", "", errors.New(errPagesPackageURLInvalid)
		}
		return "", "", 0, "", "", err
	}
	// Ownership transfers to the existing one-shot caller, which removes the
	// temporary file after the candidate deployment has been created.
	return candidate.TempPath, candidate.Checksum, candidate.PackageSize, candidate.Format, candidate.SafeLabel, nil
}

func parseAndValidatePagesDownloadURL(raw string) (*url.URL, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, errors.New(errPagesPackageURLRequired)
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.User != nil || parsed.Fragment != "" || parsed.Opaque != "" {
		return nil, errors.New(errPagesPackageURLInvalid)
	}
	scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
	if (scheme != remoteSourceSchemeHTTP && scheme != remoteSourceSchemeHTTPS) || strings.TrimSpace(parsed.Hostname()) == "" {
		return nil, errors.New(errPagesPackageURLInvalid)
	}
	return parsed, nil
}
