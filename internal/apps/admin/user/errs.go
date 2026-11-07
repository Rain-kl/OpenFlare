// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package user 提供用户管理功能
package user

const (
	userNotFound          = "user was not found"
	cannotDisable         = "administrator accounts cannot be disabled"
	cannotDelete          = "administrator accounts cannot be deleted"
	cannotDeleteSelf      = "the currently signed-in user cannot be deleted"
	updateUserFailed      = "Failed to update user status"
	deleteUserFailed      = "Failed to delete user"
	usernameExists        = "username already exists"
	usernameRequired      = "username must not be empty"
	passwordTooShort      = "password must contain at least 8 characters" //nolint:gosec // false positive: this is an error message, not hardcoded credentials
	createUserFailed      = "Failed to create user"
	emailRequired         = "email address must not be empty"
	emailExists           = "email address is already registered"
	cannotRevokeSelfAdmin = "administrator privileges cannot be revoked from the currently signed-in user"
	updateUserInfoFailed  = "Failed to update user information"
)
