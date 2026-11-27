// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package smartdns

import (
	"errors"
	"net/http"

	"github.com/Rain-kl/Wavelet/internal/shared/response"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"github.com/gin-gonic/gin"
)

const (
	errSmartDNSDisabled        = "SmartDNS service is currently disabled"
	errInstanceNotFound        = "DNS instance not found"
	errInstanceEmbeddedLocked  = "embedded SmartDNS instance cannot be deleted or downgraded"
	errInstanceIDInvalid       = "invalid DNS instance ID"
	errInstanceNameRequired    = "DNS instance name is required and must not exceed 128 characters"
	errInstanceListenInvalid   = "invalid DNS instance listen address"
	errInstanceQuarantined     = "DNS instance is quarantined"
	errTokenInvalid            = "invalid DNS instance authentication token" //nolint:gosec // user-facing error message, not a credential
	errDiscoveryTokenInvalid   = "invalid DNS discovery token"               //nolint:gosec // user-facing error message, not a credential
	errPolicyNotFound          = "DNS routing policy not found"
	errPolicyNameRequired      = "policy name is required and must not exceed 128 characters"
	errPolicyDefaultLocked     = "default DNS policy cannot be deleted"
	errPolicyInUse             = "policy is referenced by Cloudflare SmartDNS pointing groups; unlink it first"
	errPolicyGraphInvalid      = "invalid policy decision graph or JDM configuration"
	errPolicyStrategyInvalid   = "unsupported scheduling strategy type"
	errPolicyFallbackInvalid   = "unsupported fallback strategy type"
	errPolicyScopeInvalid      = "unsupported policy scope type"
	errPolicyScopeDomainEmpty  = "at least one domain pattern is required for domain-scoped policies"
	errPolicyTTLInvalid        = "policy TTL must be between 10 and 86400 seconds"
	errPolicyMaxAnswersInvalid = "policy max_answers must be between 1 and 16"
	errSimulatorDomainInvalid  = "valid domain is required for DNS resolution simulation"
	errSimulatorIPInvalid      = "valid IPv4 or IPv6 client address is required"
	errSimulatorQTypeInvalid   = "only A and AAAA record types are supported for simulation"
	errSettingsInvalid         = "invalid SmartDNS configuration parameters"
	errDelegationDomainInvalid = "invalid global delegation domain format"
	errOverviewQueryFailed     = "failed to load SmartDNS overview statistics"
	errListInstancesFailed     = "failed to list DNS instances"
	errCreateInstanceFailed    = "failed to create DNS instance"
	errUpdateInstanceFailed    = "failed to update DNS instance"
	errDeleteInstanceFailed    = "failed to delete DNS instance"
	errListPoliciesFailed      = "failed to list DNS policies"
	errGetPolicyFailed         = "failed to load DNS policy details"
	errCreatePolicyFailed      = "failed to create DNS policy"
	errUpdatePolicyFailed      = "failed to update DNS policy"
	errDeletePolicyFailed      = "failed to delete DNS policy"
	errGetSettingsFailed       = "failed to load SmartDNS settings"
	errSaveSettingsFailed      = "failed to save SmartDNS settings"
	errVerifyDelegationFailed  = "failed to verify DNS delegation status"
	errHeartbeatFailed         = "failed to record DNS instance heartbeat"
	errSnapshotLoadFailed      = "failed to generate SmartDNS runtime snapshot"
)

type logicError struct {
	status int
	msg    string
}

func (e *logicError) Error() string {
	return e.msg
}

func newLogicError(status int, msg string) error {
	return &logicError{status: status, msg: msg}
}

func abortSmartDNSError(c *gin.Context, err error, internalMsg string) {
	ctx := c.Request.Context()
	var le *logicError
	if errors.As(err, &le) {
		switch le.status {
		case http.StatusBadRequest:
			response.AbortBadRequest(c, le.msg)
		case http.StatusUnauthorized:
			response.AbortUnauthorized(c, le.msg)
		case http.StatusForbidden:
			response.AbortForbidden(c, le.msg)
		case http.StatusNotFound:
			response.AbortNotFound(c, le.msg)
		case http.StatusConflict:
			response.AbortConflict(c, le.msg)
		default:
			response.AbortInternal(c, le.msg)
		}
		return
	}
	logger.ErrorF(ctx, "%s: %v", internalMsg, err)
	response.AbortInternal(c, internalMsg)
}
