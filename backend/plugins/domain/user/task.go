// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package user

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/pkg/logger"
	pkgmail "Wavelet/pkg/mail"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

const (
	// TaskSendEmailCode is the queue pattern for email verification codes.
	TaskSendEmailCode = "user:send_email_code"
	// TaskTypeSendEmailCode is the admin type identifier for email verification codes.
	TaskTypeSendEmailCode = "send_email_code"
	// TaskSendMail is the queue pattern for generic outbound mail.
	TaskSendMail = "mail:send"
	// TaskTypeSendMail is the admin type identifier for generic outbound mail.
	TaskTypeSendMail = "send_email"
	// TaskCleanupInactive is the queue pattern for inactive-user cleanup.
	TaskCleanupInactive = "user:cleanup_inactive"
	// TaskTypeCleanupInactive is the admin type identifier for inactive-user cleanup.
	TaskTypeCleanupInactive = "cleanup_inactive_users"

	defaultUserTaskRetry    = 3
	emailCodeTTL            = 10 * time.Minute
	emailCodeCacheKeyPrefix = "user:email_code:"
	inactiveRetentionDays   = 30
	hoursPerDay             = 24
	inactiveRetention       = inactiveRetentionDays * hoursPerDay * time.Hour
	smtpConfigKeyHost       = "smtp_host"
	smtpConfigKeyPort       = "smtp_port"
	smtpConfigKeyUsername   = "smtp_username"
	smtpConfigKeyPassword   = "smtp_password"
	defaultSMTPPort         = 587
	emailCodeLength         = 6
	emailCodeModulo         = 1000000
	taskQueueDefault        = "default"
	taskParamTypeString     = "string"
	taskParamTypeText       = "text"
	paramNameEmail          = "email"
)

var smtpConfigKeys = []string{
	smtpConfigKeyHost, smtpConfigKeyPort, smtpConfigKeyUsername, smtpConfigKeyPassword,
}

var (
	cacheMu  sync.RWMutex
	cacheSvc contracts.CacheService
	taskMu   sync.RWMutex
	taskSvc  contracts.TaskService
)

// SetCacheService sets the cache contract used to store email verification codes.
func SetCacheService(s contracts.CacheService) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	cacheSvc = s
}

// SetTaskService sets the task contract used by HTTP handlers to enqueue mail jobs.
func SetTaskService(s contracts.TaskService) {
	taskMu.Lock()
	defer taskMu.Unlock()
	taskSvc = s
}

func getCache(ctx context.Context) contracts.CacheService {
	if s, err := core.InjectFrom[contracts.CacheService](ctx); err == nil && s != nil {
		return s
	}
	cacheMu.RLock()
	defer cacheMu.RUnlock()
	return cacheSvc
}

func getTaskService(ctx context.Context) contracts.TaskService {
	if s, err := core.InjectFrom[contracts.TaskService](ctx); err == nil && s != nil {
		return s
	}
	taskMu.RLock()
	defer taskMu.RUnlock()
	return taskSvc
}

func appendTaskLog(ctx context.Context, format string, args ...any) {
	if svc := getTaskService(ctx); svc != nil {
		svc.AppendLog(ctx, format, args...)
	}
}

// SendEmailCodeMeta describes the email verification-code task.
var SendEmailCodeMeta = contracts.TaskMetaDTO{
	Type:        TaskTypeSendEmailCode,
	AsynqTask:   TaskSendEmailCode,
	Name:        "发送邮箱验证码",
	DisplayName: "发送邮箱验证码",
	Description: "异步发送用户注册与验证邮箱验证码",
	Category:    "user",
	MaxRetry:    defaultUserTaskRetry,
	Queue:       taskQueueDefault,
	Retryable:   true,
	Params: []contracts.TaskParamDTO{
		{Name: paramNameEmail, Label: "目标邮箱", Type: taskParamTypeString, Required: true, Placeholder: "user@example.com", Description: "接收验证码的目标邮箱"},
		{Name: "code", Label: "验证码", Type: taskParamTypeString, Required: false, Placeholder: "123456", Description: "6 位数字验证码，留空则自动生成"},
	},
}

// SendMailMeta describes the generic outbound-mail task.
var SendMailMeta = contracts.TaskMetaDTO{
	Type:        TaskTypeSendMail,
	AsynqTask:   TaskSendMail,
	Name:        "发送邮件",
	DisplayName: "发送邮件",
	Description: "异步发送系统邮件",
	Category:    "mail",
	MaxRetry:    defaultUserTaskRetry,
	Queue:       taskQueueDefault,
	Retryable:   true,
	Params: []contracts.TaskParamDTO{
		{Name: "to", Label: "接收邮箱 (To)", Type: taskParamTypeString, Required: true, Placeholder: "receiver@example.com", Description: "接收邮件的目标邮箱地址"},
		{Name: "subject", Label: "邮件主题 (Subject)", Type: taskParamTypeString, Required: true, Placeholder: "请输入邮件主题", Description: "发送邮件的主题标题"},
		{Name: "body", Label: "邮件内容 (Body)", Type: taskParamTypeText, Required: true, Placeholder: "请输入邮件内容（支持 HTML格式）", Description: "发送邮件的内容主体"},
	},
}

// CleanupInactiveMeta describes the inactive-user cleanup task.
var CleanupInactiveMeta = contracts.TaskMetaDTO{
	Type:        TaskTypeCleanupInactive,
	AsynqTask:   TaskCleanupInactive,
	Name:        "清理未激活用户",
	DisplayName: "清理未激活用户",
	Description: "清理长期未登录的注册用户及其访问令牌",
	Category:    "user",
	Queue:       taskQueueDefault,
	Retryable:   true,
}

type sendEmailCodePayload struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

type sendMailPayload struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

// SendEmailCodeHandler sends a 6-digit email verification code and caches it.
type SendEmailCodeHandler struct{}

// ValidatePayload checks the destination address and optional code.
func (h *SendEmailCodeHandler) ValidatePayload(payload []byte) ([]byte, error) {
	p, err := parseSendEmailCodePayload(payload)
	if err != nil {
		return nil, err
	}
	return json.Marshal(p)
}

// Execute generates (if needed), caches, and emails the verification code.
func (h *SendEmailCodeHandler) Execute(ctx context.Context, payload []byte) (*contracts.TaskResultDTO, error) {
	p, err := parseSendEmailCodePayload(payload)
	if err != nil {
		return nil, err
	}
	if p.Code == "" {
		p.Code, err = generateEmailCode()
		if err != nil {
			return nil, err
		}
	}

	cache := getCache(ctx)
	if cache == nil {
		return nil, errors.New(errEmailCacheUnavailable)
	}
	if err := cache.Set(ctx, emailCodeCacheKey(p.Email), p.Code, emailCodeTTL); err != nil {
		return nil, fmt.Errorf("store email code: %w", err)
	}

	cfg, err := loadSMTPConfig(ctx)
	if err != nil {
		return nil, err
	}
	subject := "邮箱验证码"
	body := fmt.Sprintf("<p>您的验证码是 <b>%s</b>，%d 分钟内有效。</p>", p.Code, int(emailCodeTTL.Minutes()))
	appendTaskLog(ctx, "发送邮箱验证码到 %s", maskEmail(p.Email))
	if err := pkgmail.SendMail(ctx, cfg, p.Email, subject, body); err != nil {
		logger.ErrorF(ctx, "send email code failed: %v", err)
		return nil, errors.New(errSendEmailFailed)
	}
	return &contracts.TaskResultDTO{Message: fmt.Sprintf("验证码已发送至 %s", maskEmail(p.Email))}, nil
}

// SendMailHandler sends a generic HTML email through the configured SMTP server.
type SendMailHandler struct{}

// ValidatePayload checks to/subject/body.
func (h *SendMailHandler) ValidatePayload(payload []byte) ([]byte, error) {
	p, err := parseSendMailPayload(payload)
	if err != nil {
		return nil, err
	}
	return json.Marshal(p)
}

// Execute sends the mail.
func (h *SendMailHandler) Execute(ctx context.Context, payload []byte) (*contracts.TaskResultDTO, error) {
	p, err := parseSendMailPayload(payload)
	if err != nil {
		return nil, err
	}
	cfg, err := loadSMTPConfig(ctx)
	if err != nil {
		return nil, err
	}
	appendTaskLog(ctx, "发送邮件到 %s，主题: %s", maskEmail(p.To), p.Subject)
	if err := pkgmail.SendMail(ctx, cfg, p.To, p.Subject, p.Body); err != nil {
		logger.ErrorF(ctx, "send mail failed: %v", err)
		return nil, errors.New(errSendEmailFailed)
	}
	return &contracts.TaskResultDTO{Message: fmt.Sprintf("邮件已发送至 %s", maskEmail(p.To))}, nil
}

// CleanupInactiveHandler deletes users who registered long ago and never logged in.
type CleanupInactiveHandler struct{}

// Execute removes stale never-logged-in non-admin users and their access tokens.
func (h *CleanupInactiveHandler) Execute(ctx context.Context, _ []byte) (*contracts.TaskResultDTO, error) {
	cutoff := time.Now().Add(-inactiveRetention)
	ids, err := ListInactiveNeverLoggedInUserIDs(ctx, cutoff)
	if err != nil {
		return nil, err
	}
	appendTaskLog(ctx, "扫描到 %d 个超过 %d 天未登录的注册用户", len(ids), int(inactiveRetention.Hours()/float64(hoursPerDay)))
	deleted := 0
	for _, id := range ids {
		if err := DeleteUserWithRelations(ctx, id); err != nil {
			logger.ErrorF(ctx, "cleanup inactive user %d failed: %v", id, err)
			continue
		}
		deleted++
	}
	msg := fmt.Sprintf("已清理 %d 个长期未登录用户及其访问令牌", deleted)
	appendTaskLog(ctx, "%s", msg)
	return &contracts.TaskResultDTO{Message: msg}, nil
}

func parseSendEmailCodePayload(payload []byte) (sendEmailCodePayload, error) {
	var p sendEmailCodePayload
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &p); err != nil {
			return p, errors.New(errInvalidTaskPayload)
		}
	}
	p.Email = normalizeEmail(p.Email)
	if err := validateEmail(p.Email); err != nil {
		return p, err
	}
	p.Code = strings.TrimSpace(p.Code)
	if p.Code != "" && !isSixDigitCode(p.Code) {
		return p, errors.New(errInvalidEmailCode)
	}
	return p, nil
}

func parseSendMailPayload(payload []byte) (sendMailPayload, error) {
	var p sendMailPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return p, errors.New(errInvalidTaskPayload)
	}
	p.To = normalizeEmail(p.To)
	p.Subject = strings.TrimSpace(p.Subject)
	if err := validateEmail(p.To); err != nil {
		return p, err
	}
	if p.Subject == "" {
		return p, errors.New(errMailSubjectRequired)
	}
	if strings.TrimSpace(p.Body) == "" {
		return p, errors.New(errMailBodyRequired)
	}
	return p, nil
}

func loadSMTPConfig(ctx context.Context) (pkgmail.Config, error) {
	db := getDB(ctx)
	if db == nil {
		return pkgmail.Config{}, errors.New(errSMTPNotConfigured)
	}
	var rows []struct {
		Key   string
		Value string
	}
	if err := db.Table("w_system_configs").
		Select("key", "value").
		Where("key IN ?", smtpConfigKeys).
		Find(&rows).Error; err != nil {
		return pkgmail.Config{}, fmt.Errorf("read smtp config: %w", err)
	}
	cfg := pkgmail.Config{Port: defaultSMTPPort}
	for _, row := range rows {
		switch row.Key {
		case smtpConfigKeyHost:
			cfg.Host = strings.TrimSpace(row.Value)
		case smtpConfigKeyPort:
			if n, err := strconv.Atoi(strings.TrimSpace(row.Value)); err == nil && n > 0 {
				cfg.Port = n
			}
		case smtpConfigKeyUsername:
			cfg.Username = strings.TrimSpace(row.Value)
		case smtpConfigKeyPassword:
			cfg.Password = row.Value
		}
	}
	if cfg.Host == "" || cfg.Username == "" {
		return pkgmail.Config{}, errors.New(errSMTPNotConfigured)
	}
	return cfg, nil
}

func generateEmailCode() (string, error) {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	n := binary.BigEndian.Uint32(buf[:]) % emailCodeModulo
	return fmt.Sprintf("%06d", n), nil
}

func emailCodeCacheKey(email string) string {
	return emailCodeCacheKeyPrefix + normalizeEmail(email)
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func validateEmail(email string) error {
	if email == "" {
		return errors.New(errEmailEmpty)
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || !strings.EqualFold(addr.Address, email) {
		return errors.New(errInvalidEmail)
	}
	return nil
}

func isSixDigitCode(code string) bool {
	if len(code) != emailCodeLength {
		return false
	}
	for _, r := range code {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func maskEmail(email string) string {
	at := strings.IndexByte(email, '@')
	if at <= 1 {
		return "***"
	}
	return email[:1] + "***" + email[at:]
}
