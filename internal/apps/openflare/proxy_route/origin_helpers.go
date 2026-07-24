// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package proxy_route

import (
	"context"
	"errors"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/repository"

	"gorm.io/gorm"
)

func resolveStructuredOriginInput(ctx context.Context, input Input) (string, *uint, error) {
	scheme, err := normalizeOriginScheme(input.OriginScheme)
	if err != nil {
		return "", nil, err
	}
	port, err := normalizeOriginPort(input.OriginPort)
	if err != nil {
		return "", nil, err
	}
	uri, err := normalizeOriginURI(input.OriginURI)
	if err != nil {
		return "", nil, err
	}
	if input.OriginID != nil && *input.OriginID != 0 {
		return resolveOriginByID(ctx, scheme, port, uri, *input.OriginID)
	}
	return resolveOriginByAddress(ctx, scheme, port, uri, input.OriginAddress)
}

func resolveOriginByID(ctx context.Context, scheme, port, uri string, originID uint) (string, *uint, error) {
	origin, err := repository.GetOriginByID(ctx, originID)
	if err != nil {
		return "", nil, errors.New(errProxyRouteOriginNotFound)
	}
	originURL, err := buildOriginURLFromParts(scheme, origin.Address, port, uri)
	if err != nil {
		return "", nil, err
	}
	return originURL, &origin.ID, nil
}

func resolveOriginByAddress(ctx context.Context, scheme, port, uri, rawAddress string) (string, *uint, error) {
	address := normalizeOriginAddress(rawAddress)
	if err := validateOriginAddress(address); err != nil {
		return "", nil, err
	}
	originURL, err := buildOriginURLFromParts(scheme, address, port, uri)
	if err != nil {
		return "", nil, err
	}
	origin, err := getOrCreateOriginByAddress(ctx, address)
	if err != nil {
		return "", nil, err
	}
	return originURL, &origin.ID, nil
}

func resolveLegacyOriginInput(ctx context.Context, originURL string) (string, *uint, error) {
	if originURL == "" {
		return "", nil, errors.New(errProxyRouteOriginEmpty)
	}
	address, err := extractOriginAddress(originURL)
	if err != nil {
		return "", nil, err
	}
	origin, findErr := repository.GetOriginByAddress(ctx, address)
	if findErr == nil {
		return originURL, &origin.ID, nil
	}
	if !errors.Is(findErr, gorm.ErrRecordNotFound) {
		return "", nil, findErr
	}
	return originURL, nil, nil
}

func resolveProxyRoutePrimaryOrigin(ctx context.Context, input Input) (string, *uint, error) {
	if hasStructuredOriginInput(input) {
		return resolveStructuredOriginInput(ctx, input)
	}
	return resolveLegacyOriginInput(ctx, strings.TrimSpace(input.OriginURL))
}
