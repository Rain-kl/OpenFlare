// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package nettools implements admin diagnostic operations and HTTP handlers for
// multi-location Ping/TCPing, Traceroute, ASN/IP intelligence lookup, DNS/ECS queries,
// network quality matrix inspection, and dataset synchronization.
package nettools

const (
	errTargetRequired        = "target is required"
	errTargetInvalid         = "invalid target hostname or IP address"
	errTargetPrivateBlocked  = "private, loopback, link-local, or internal targets are not permitted"
	errResolverInvalid       = "invalid DNS resolver host or port"
	errResolverPrivate       = "private or internal DNS resolvers are not permitted"
	errECSSubnetInvalid      = "invalid EDNS0 client subnet CIDR prefix"
	errDNSRecordTypeInvalid  = "unsupported DNS record type"
	errPingProtocolInvalid   = "ping protocol must be icmp, tcp, or http"
	errTraceProtocolInvalid  = "traceroute protocol must be icmp, udp, or tcp"
	errDomainRequired        = "domain is required"
	errInvalidASN            = "valid non-zero ASN is required"
	errNoEligibleNodes       = "no eligible online edge nodes available for probe dispatch"
	errTaskNotFound          = "network probe task was not found or has expired"
	errInvalidPort           = "port must be between 1 and 65535"
	errDatabaseUnavailable   = "database is not initialized"
	errLookupInputRequired   = "query IP, hostname, or ASN is required"
	errASPathTargetRequired  = "target_asn or target_ip is required"
	errPredictTargetRequired = "target_ip, target_asn, or target_country is required"
	errHostnameNoPublicIP    = "hostname did not resolve to any valid public IP address"
	errProbeTargetNotFound   = "probe target was not found"
	errProbeTargetDuplicate  = "probe target with same ip, protocol, and port already exists"
	errProbeTargetProto      = "probe_proto must be icmp, tcp, or traceroute"
	errProbeTargetType       = "target_type must be public_anchor, customer_isp, or manual"
	errProbeTargetAnycast    = "anycast_status must be unknown, unicast, or confirmed"
	errProbeTargetIPInvalid  = "probe target must be a valid public IPv4 or IPv6 address"
)
