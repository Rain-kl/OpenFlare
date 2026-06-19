// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package origin

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"unicode"
)

func normalizeOriginAddress(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func validateOriginAddress(address string) error {
	if address == "" {
		return errors.New(errOriginAddressRequired)
	}
	if strings.Contains(address, "://") || strings.ContainsAny(address, "/?#") {
		return errors.New(errOriginAddressInvalid)
	}
	if strings.HasPrefix(address, "[") || strings.HasSuffix(address, "]") {
		return errors.New(errOriginAddressInvalid)
	}
	if ip := net.ParseIP(address); ip != nil {
		return nil
	}
	if len(address) > 253 {
		return errors.New(errOriginAddressInvalid)
	}
	labels := strings.Split(address, ".")
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 {
			return errors.New(errOriginAddressInvalid)
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return errors.New(errOriginAddressInvalid)
		}
		for _, r := range label {
			if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' {
				continue
			}
			return errors.New(errOriginAddressInvalid)
		}
	}
	return nil
}

func normalizeOriginName(name string, address string) string {
	normalized := strings.TrimSpace(name)
	if normalized != "" {
		return normalized
	}
	return address
}

func formatOriginHost(address string, port string) string {
	return net.JoinHostPort(address, port)
}

func rewriteOriginURLAddress(rawURL string, newAddress string) (string, error) {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(rawURL))
	if err != nil {
		return "", fmt.Errorf("%s: %w", errOriginAddressInvalid, err)
	}
	address := normalizeOriginAddress(newAddress)
	if err := validateOriginAddress(address); err != nil {
		return "", err
	}
	port := parsed.Port()
	if port == "" {
		return "", errors.New(errOriginMissingPort)
	}
	parsed.Host = formatOriginHost(address, port)
	return parsed.String(), nil
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "unique")
}
