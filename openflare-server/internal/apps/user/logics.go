// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/task"
	pkgu "github.com/Rain-kl/Wavelet/pkg/util"
)

// LoginEmailVerificationStatus 登录邮箱验证的处理结果。
type LoginEmailVerificationStatus int

const (
	// LoginEmailVerificationPassed 验证通过，可继续登录流程。
	LoginEmailVerificationPassed LoginEmailVerificationStatus = iota
	// LoginEmailVerificationPending 需要用户输入邮箱验证码。
	LoginEmailVerificationPending
	// LoginEmailVerificationRejected 验证被拒绝（验证码错误、临时码提示等）。
	LoginEmailVerificationRejected
)

// LoginEmailVerificationResult 登录邮箱验证的业务结果。
type LoginEmailVerificationResult struct {
	Status  LoginEmailVerificationStatus
	Message string
}

type updateProfileInput struct {
	Nickname  string
	Email     string
	AvatarURL string
	Bio       string
	Phone     string
	Gender    string
	Website   string
	Location  string
}

func isPasswordLoginEnabled(ctx context.Context) bool {
	enabled, err := repository.GetBoolByKey(ctx, model.ConfigKeyPasswordLoginEnabled)
	if err != nil {
		return true
	}
	return enabled
}

func isPasswordRegisterEnabled(ctx context.Context) bool {
	enabled, err := repository.GetBoolByKey(ctx, model.ConfigKeyPasswordRegisterEnabled)
	if err != nil {
		return true
	}
	return enabled
}

func isRegistrationEnabled(ctx context.Context) bool {
	enabled, err := repository.GetBoolByKey(ctx, model.ConfigKeyRegistrationEnabled)
	if err != nil {
		return true
	}
	return enabled
}

func isEmailLoginVerificationEnabled(ctx context.Context) bool {
	enabled, err := repository.GetBoolByKey(ctx, model.ConfigKeyEmailLoginVerificationEnabled)
	if err != nil {
		return false
	}
	return enabled
}

func isEmailRegisterVerificationEnabled(ctx context.Context) bool {
	enabled, err := repository.GetBoolByKey(ctx, model.ConfigKeyEmailRegisterVerificationEnabled)
	if err != nil {
		return false
	}
	return enabled
}

func isSMTPConfigured(ctx context.Context) bool {
	scHost, errHost := repository.GetSystemConfigByKey(ctx, model.ConfigKeySMTPHost)
	scPort, errPort := repository.GetSystemConfigByKey(ctx, model.ConfigKeySMTPPort)
	scUser, errUser := repository.GetSystemConfigByKey(ctx, model.ConfigKeySMTPUsername)
	scPass, errPass := repository.GetSystemConfigByKey(ctx, model.ConfigKeySMTPPassword)
	if errHost != nil || errPort != nil || errUser != nil || errPass != nil {
		return false
	}
	return scHost.Value != "" && scPort.Value != "" && scUser.Value != "" && scPass.Value != ""
}

func generateVerificationCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(verificationCodeRange))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()+verificationCodeOffset), nil
}

func getEmailCodeKey(scene, email string) string {
	return fmt.Sprintf("email_code:%s:%s", scene, email)
}

func getEmailCooldownKey(scene, email string) string {
	return fmt.Sprintf("email_code:cooldown:%s:%s", scene, email)
}

func sendEmailVerificationCode(ctx context.Context, email, scene, templateName string) error {
	if !isSMTPConfigured(ctx) {
		return errors.New(errSMTPConfigIncomplete)
	}

	code, err := generateVerificationCode()
	if err != nil {
		return errors.New(errGenerateEmailCodeFailed)
	}
	codeKey := getEmailCodeKey(scene, email)
	cooldownKey := getEmailCooldownKey(scene, email)

	tmpl, err := repository.GetTemplateByKey(ctx, templateName)
	if err != nil {
		return fmt.Errorf("模板 %s 不存在或不可用: %w", templateName, err)
	}
	emailSubject, emailBody, err := tmpl.Render(map[string]any{"Code": code})
	if err != nil {
		return fmt.Errorf(errRenderEmailTemplateFailed, err)
	}

	if err := db.SetJSON(ctx, codeKey, code, emailCodeExpiry); err != nil {
		return errors.New(errGenerateEmailCodeFailed)
	}
	_ = db.SetJSON(ctx, cooldownKey, "1", emailCodeCooldown)

	payload := SendEmailPayload{
		To:      email,
		Subject: emailSubject,
		Body:    emailBody,
	}
	payloadBytes, _ := json.Marshal(payload)
	_, err = task.DispatchTask(ctx, TaskTypeSendEmail, payloadBytes, "system")
	if err != nil {
		return errors.New(errDispatchEmailTaskFailed)
	}
	return nil
}

func verifyEmailCode(ctx context.Context, email, scene, code string) bool {
	codeKey := getEmailCodeKey(scene, email)
	var storedCode string
	if err := db.GetJSON(ctx, codeKey, &storedCode); err != nil {
		return false
	}
	if storedCode != code {
		return false
	}
	_ = db.Redis.Del(ctx, db.PrefixedKey(codeKey)).Err()
	return true
}

func processLoginEmailVerification(ctx context.Context, code string, user *model.User) (LoginEmailVerificationResult, error) {
	if code != "" {
		if !verifyEmailCode(ctx, user.Email, "login", code) {
			return LoginEmailVerificationResult{
				Status:  LoginEmailVerificationRejected,
				Message: errEmailCodeInvalidOrExpired,
			}, nil
		}
		return LoginEmailVerificationResult{Status: LoginEmailVerificationPassed}, nil
	}

	// 如果 SMTP 未配置，或者用户没有绑定邮箱（无法发送验证码），则使用临时码 888888
	if !isSMTPConfigured(ctx) || user.Email == "" {
		codeKey := getEmailCodeKey("login", user.Email)
		if err := db.SetJSON(ctx, codeKey, "888888", emailCodeExpiry); err != nil {
			return LoginEmailVerificationResult{}, errors.New(errGenerateEmailCodeFailed)
		}
		var msg string
		if !isSMTPConfigured(ctx) {
			msg = errSMTPInvalidUseTempCodePrefix + errSMTPInvalidUseTempCode
		} else {
			msg = errSMTPInvalidUseTempCodePrefix + "该账号未绑定邮箱，使用临时码登录"
		}
		return LoginEmailVerificationResult{
			Status:  LoginEmailVerificationRejected,
			Message: msg,
		}, nil
	}

	cooldownKey := getEmailCooldownKey("login", user.Email)
	var temp string
	if err := db.GetJSON(ctx, cooldownKey, &temp); err != nil {
		if err := sendEmailVerificationCode(ctx, user.Email, "login", "login_email"); err != nil {
			return LoginEmailVerificationResult{}, err
		}
	}

	maskedEmail := pkgu.MaskEmail(user.Email)
	return LoginEmailVerificationResult{
		Status:  LoginEmailVerificationPending,
		Message: errNeedEmailCodePrefix + maskedEmail,
	}, nil
}

func sendRegisterEmailCode(ctx context.Context, email string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return errors.New(errEmailRequired)
	}

	var count int64
	if err := db.DB(ctx).Model(&model.User{}).Where("email = ?", email).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New(errEmailAlreadyRegistered)
	}

	cooldownKey := getEmailCooldownKey("register", email)
	var temp string
	if err := db.GetJSON(ctx, cooldownKey, &temp); err == nil {
		return errors.New(errEmailCodeCooldown)
	}

	return sendEmailVerificationCode(ctx, email, "register", "register_email")
}

func validateRegisterEmailVerification(ctx context.Context, email, code string) error {
	if !isEmailRegisterVerificationEnabled(ctx) {
		return nil
	}
	if email == "" || code == "" {
		return errors.New(errEmailOrCodeRequired)
	}
	if !verifyEmailCode(ctx, email, "register", code) {
		return errors.New(errEmailCodeInvalidOrExpired)
	}
	return nil
}

func updateUserProfile(ctx context.Context, userID uint64, input updateProfileInput) (*model.User, error) {
	var dbUser model.User
	if err := db.DB(ctx).Where("id = ?", userID).First(&dbUser).Error; err != nil {
		return nil, errors.New(errUserNotFound)
	}

	input.Email = strings.TrimSpace(input.Email)
	if input.Email != "" && input.Email != dbUser.Email {
		if !strings.Contains(input.Email, "@") || !strings.Contains(input.Email, ".") {
			return nil, errors.New(errEmailFormatInvalid)
		}

		var count int64
		if err := db.DB(ctx).Model(&model.User{}).Where("email = ? AND id != ?", input.Email, dbUser.ID).Count(&count).Error; err != nil {
			return nil, err
		}
		if count > 0 {
			return nil, errors.New(errEmailAlreadyBound)
		}
	}

	dbUser.Nickname = strings.TrimSpace(input.Nickname)
	if dbUser.Nickname == "" {
		dbUser.Nickname = dbUser.Username
	}
	dbUser.Email = input.Email
	dbUser.AvatarURL = input.AvatarURL
	dbUser.Bio = input.Bio
	dbUser.Phone = strings.TrimSpace(input.Phone)
	dbUser.Gender = strings.TrimSpace(input.Gender)
	dbUser.Website = strings.TrimSpace(input.Website)
	dbUser.Location = strings.TrimSpace(input.Location)

	if err := db.DB(ctx).Save(&dbUser).Error; err != nil {
		return nil, err
	}
	return &dbUser, nil
}
