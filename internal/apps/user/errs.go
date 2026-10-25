// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package user

const (
	errBindParamsFailed             = "Failed to bind parameters"
	errInvalidParams                = "invalid parameters"
	errPasswordLoginDisabled        = "Password login is disabled by the administrator" //nolint:gosec // false positive: this is an error message, not hardcoded credentials
	errUsernameOrPasswordWrong      = "Incorrect username or password"                  //nolint:gosec // false positive: this is an error message, not hardcoded credentials
	errLoginEmailMissing            = "This account has no email address; contact an administrator to bind an email before signing in"
	errNeedEmailCodePrefix          = "need_email_code:"
	errSMTPInvalidUseTempCodePrefix = "smtp_invalid:"
	errSMTPInvalidUseTempCode       = "Invalid SMTP configuration; use a temporary code to sign in"
	errEmailCodeInvalidOrExpired    = "verification code is incorrect or expired"
	errSaveSessionFailed            = "Unable to save session; please try again"
	errRegistrationDisabled         = "Registration is disabled by the administrator"
	errPasswordTooShort             = "password must contain at least 8 characters" //nolint:gosec // false positive: this is an error message, not hardcoded credentials
	errEmailOrCodeRequired          = "email address or verification code is missing"
	errNewPasswordTooShort          = "New password must contain at least 8 characters" //nolint:gosec // false positive: this is an error message, not hardcoded credentials
	errLoginRequired                = "Please sign in first"
	errUserNotFound                 = "user was not found"
	errOldPasswordIncorrect         = "current password is incorrect"                //nolint:gosec // false positive: this is an error message, not hardcoded credentials
	errPasswordEncryptFailed        = "password encryption failed; please try again" //nolint:gosec // false positive: this is an error message, not hardcoded credentials
	errEmailRequired                = "email address must not be empty"
	errUnsupportedEmailScene        = "Unsupported verification scenario"
	errEmailAlreadyRegistered       = "email address is already registered"
	errEmailCodeCooldown            = "verification codes are being requested too frequently; try again later"
	errLoginRateLimited             = "Too many login attempts; try again later"
	errEmailFormatInvalid           = "invalid email address format"
	errEmailAlreadyBound            = "this email address is bound to another account"
	errRenderEmailTemplateFailed    = "failed to render verification email template: %w"
	errGenerateEmailCodeFailed      = "failed to generate verification code; please try again"
	errDispatchEmailTaskFailed      = "failed to enqueue verification email task; please try again"
	errTokenNameRequired            = "Token name must not be empty"                                        //nolint:gosec // false positive: this is an error message, not hardcoded credentials
	errAccessTokenLimitReached      = "Maximum number of access tokens reached"                             //nolint:gosec // false positive: this is an error message, not hardcoded credentials
	errGenerateTokenFailed          = "failed to generate token"                                            //nolint:gosec // false positive: this is an error message, not hardcoded credentials
	errInvalidTokenID               = "Invalid token ID"                                                    //nolint:gosec // false positive: this is an error message, not hardcoded credentials
	errTokenNotFoundOrForbidden     = "token was not found or access is not permitted"                      //nolint:gosec // false positive: this is an error message, not hardcoded credentials
	errAdminTokenRequiresAdmin      = "Only administrators can create tokens with administrator privileges" //nolint:gosec // false positive: this is an error message, not hardcoded credentials
	errTaskPayloadRequired          = "task parameters must not be empty"
	errInvalidJSONFormat            = "invalid JSON format: %w"
	errEmailTaskFieldsRequired      = "to, subject and body must not be empty"
	errParseEmailPayloadFailed      = "failed to parse email parameters: %w"
	errSMTPConfigIncomplete         = "system SMTP configuration is incomplete"
	errSendMailFailed               = "failed to send email: %w"
)
