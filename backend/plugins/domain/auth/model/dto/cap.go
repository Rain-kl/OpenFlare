// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package dto provides data transfer objects and views for the auth plugin.
package dto

import (
	"Wavelet/plugins/domain/auth/pow"
)

// ChallengeResponse is a local type alias for the pow.ChallengeResponse struct
type ChallengeResponse = pow.ChallengeResponse

// ChallengeRequest is the CAPTCHA challenge request payload.
type ChallengeRequest struct {
	Scope string `json:"scope" form:"scope"`
}

// RedeemRequest is the CAPTCHA redeem request payload.
type RedeemRequest struct {
	Token     string `json:"token" binding:"required"`
	Solutions []int  `json:"solutions" binding:"required"`
	Scope     string `json:"scope" form:"scope"`
}

// RedeemResponse is returned to the client on redeem.
type RedeemResponse struct {
	Success bool   `json:"success"`
	Token   string `json:"token,omitempty"`
	Expires int64  `json:"expires,omitempty"`
	Error   string `json:"error,omitempty"`
}
