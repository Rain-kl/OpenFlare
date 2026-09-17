// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package mail

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	gomail "github.com/wneessen/go-mail"
	golog "github.com/wneessen/go-mail/log"
)

const (
	smtpSSLPort     = 465             // SMTP SSL 端口
	smtpDialTimeout = 5 * time.Second // SMTP 连接超时
)

// Config represents SMTP mail configuration
type Config struct {
	Host               string
	Port               int
	Username           string
	Password           string
	FromName           string // 可选发件人显示名称
	InsecureSkipVerify bool   // 是否跳过证书校验 (默认跳过以兼容自签名证书)
}

// Option modifies internal mail send options
type Option func(*clientOptions)

type clientOptions struct {
	debugLogger golog.Logger
}

func withLogger(l golog.Logger) Option {
	return func(co *clientOptions) {
		co.debugLogger = l
	}
}

// SendMail sends an HTML email using the provided config and message details
func SendMail(ctx context.Context, cfg Config, to, subject, body string) error {
	return SendMailHTML(ctx, cfg, to, subject, body)
}

// SendMailHTML sends an HTML format email
func SendMailHTML(ctx context.Context, cfg Config, to, subject, body string) error {
	return send(ctx, cfg, to, subject, body)
}

// SendMailWithLog sends a test email and records a detailed SMTP connection log
func SendMailWithLog(ctx context.Context, cfg Config, to, subject, body string) (string, error) {
	var logBuf bytes.Buffer
	logger := &bufferLogger{buf: &logBuf}

	fmt.Fprintf(&logBuf, "[System] Connecting to %s:%d...\n", cfg.Host, cfg.Port)

	err := send(ctx, cfg, to, subject, body, withLogger(logger))
	if err != nil {
		fmt.Fprintf(&logBuf, "[Error] Mail sending failed: %v\n", err)
		return logBuf.String(), err
	}

	fmt.Fprintf(&logBuf, "[System] Mail sent successfully!\n")
	return logBuf.String(), nil
}

func send(ctx context.Context, cfg Config, to, subject, body string, opts ...Option) error {
	var co clientOptions
	for _, opt := range opts {
		opt(&co)
	}

	msg := gomail.NewMsg()
	var err error
	if cfg.FromName != "" {
		err = msg.FromFormat(cfg.FromName, cfg.Username)
	} else {
		err = msg.From(cfg.Username)
	}
	if err != nil {
		return fmt.Errorf(errCreateMailMessageFailed, err)
	}

	if err = msg.To(to); err != nil {
		return fmt.Errorf(errCreateMailMessageFailed, err)
	}

	msg.Subject(subject)
	msg.SetBodyString(gomail.TypeTextHTML, body)

	clientOpts := []gomail.Option{
		gomail.WithPort(cfg.Port),
		gomail.WithTimeout(smtpDialTimeout),
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec // SMTP servers might use self-signed certificates
		ServerName:         cfg.Host,
	}
	clientOpts = append(clientOpts, gomail.WithTLSConfig(tlsConfig))

	if cfg.Port == smtpSSLPort {
		clientOpts = append(clientOpts, gomail.WithSSL())
	} else {
		clientOpts = append(clientOpts, gomail.WithTLSPolicy(gomail.TLSOpportunistic))
	}

	if cfg.Username != "" && cfg.Password != "" {
		clientOpts = append(clientOpts,
			gomail.WithSMTPAuth(gomail.SMTPAuthPlain),
			gomail.WithUsername(cfg.Username),
			gomail.WithPassword(cfg.Password),
		)
	}

	if co.debugLogger != nil {
		clientOpts = append(clientOpts, gomail.WithDebugLog(), gomail.WithLogger(co.debugLogger))
	}

	client, err := gomail.NewClient(cfg.Host, clientOpts...)
	if err != nil {
		return fmt.Errorf(errCreateMailClientFailed, err)
	}
	defer func() { _ = client.Close() }()

	if err = client.DialAndSendWithContext(ctx, msg); err != nil {
		return fmt.Errorf(errSendMailFailed, err)
	}

	return nil
}

type bufferLogger struct {
	buf *bytes.Buffer
}

func (l *bufferLogger) log(level string, entry golog.Log) {
	msg := fmt.Sprintf(entry.Format, entry.Messages...)
	msg = strings.TrimRight(msg, "\r\n")
	var dir string
	switch entry.Direction {
	case golog.DirClientToServer:
		dir = "C"
	case golog.DirServerToClient:
		dir = "S"
	default:
		dir = level
	}
	fmt.Fprintf(l.buf, "[%s] %s\n", dir, msg)
}

func (l *bufferLogger) Debugf(e golog.Log) { l.log("Debug", e) }
func (l *bufferLogger) Infof(e golog.Log)  { l.log("Info", e) }
func (l *bufferLogger) Warnf(e golog.Log)  { l.log("Warn", e) }
func (l *bufferLogger) Errorf(e golog.Log) { l.log("Error", e) }
