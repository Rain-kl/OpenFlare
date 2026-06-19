// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package mail 提供 SMTP 邮件发送功能。
package mail

const (
	errDialTLSFailed            = "dial tls failed: %w"
	errSMTPClientCreationFailed = "smtp client creation failed: %w"
	errSMTPAuthFailed           = "smtp auth failed: %w"
	errSMTPMailCommandFailed    = "smtp mail command failed: %w"
	errSMTPRcptCommandFailed    = "smtp rcpt command failed: %w"
	errSMTPDataCommandFailed    = "smtp data command failed: %w"
	errSMTPWritingBodyFailed    = "smtp writing body failed: %w" //nolint:gosec // false positive: this is an error message, not hardcoded credentials
	errSendMailFailed           = "send mail failed: %w"
)
