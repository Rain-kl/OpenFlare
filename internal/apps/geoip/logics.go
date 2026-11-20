// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package geoip exposes the control-plane requester IP endpoint.
package geoip

import (
	"github.com/Rain-kl/Wavelet/pkg/geoip/iputil"
)

const unspecifiedClientIP = "0.0.0.0"

// ClientIPView describes the requester IP observed by the control plane.
type ClientIPView struct {
	IP string `json:"ip"`
}

// ResolveRequesterIP resolves and normalizes the requester IP from client and remote addresses.
func ResolveRequesterIP(clientIP, remoteAddr string) *ClientIPView {
	resolved := iputil.NormalizeIP(clientIP)
	if resolved == "" {
		resolved = iputil.NormalizeRemoteAddr(remoteAddr)
	}
	if resolved == "" {
		resolved = unspecifiedClientIP
	}
	return &ClientIPView{IP: resolved}
}
