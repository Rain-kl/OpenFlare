// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cloudflare

const (
	errConnectionNotConfigured = "no Cloudflare connection is configured"
	errConnectionSourceInvalid = "invalid Cloudflare connection source"
	errStandaloneInputRequired = "enter a Cloudflare API token"
	errStandaloneInputInvalid  = "configured Cloudflare API token is invalid"
	errDNSAccountInvalid       = "select a valid Cloudflare DNS account"
	errGroupNameRequired       = "group name must not be empty"
	errGroupNodeSame           = "primary and standby nodes must be different"
	errGroupTargetModeInvalid  = "select a valid target mode"
	errRecordTypeInvalid       = "select a CNAME, A or AAAA record type"
	errRecordContentInvalid    = "invalid DNS record value; check the domain name or IP address"
	errNodeInvalid             = "select a valid edge node"
	errNodeIPv4Required        = "the active node requires a valid IPv4 address"
	errGroupDisabled           = "target group is disabled"
	errMemberExists            = "this domain already belongs to another target group"
	errMultipleDNSRecords      = "multiple Cloudflare DNS records have the same name; remove the duplicates manually first"
	errSyncFailed              = "Cloudflare DNS synchronization failed"
	errDeleteRemoteFailed      = "failed to delete Cloudflare DNS record"
	errTaskDispatchFailed      = "unable to enqueue Cloudflare synchronization task"
	errTargetGroupSame         = "target group must differ from the current group"
	errTargetGroupInvalid      = "target group was not found"
	errNoMembersSelected       = "no members selected"
)
