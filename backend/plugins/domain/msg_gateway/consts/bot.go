// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package consts defines constants, error types, and identifiers for msg_gateway.
package consts

// Bot Channel type and scope constants.
const (
	ChannelTypeTelegram        = "telegram"
	ChannelTypeQQ              = "qq"
	MessageChannelTypeTelegram = "telegram"
	MessageChannelTypeQQ       = "qq"
	MessageOwnerScopeSystem    = "system"
)

// Bot Task and Schedule identifier constants.
const (
	TaskCleanupPairingCodes = "msg_gateway:cleanup_pairing_codes"
	TaskDispatchBotMsg      = "msg_gateway:dispatch_bot_msg"
	TaskTypeDispatchBotMsg  = "dispatch_bot_msg"
)

// Pairing code constants.
const (
	CodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	CodeLength   = 8
)
