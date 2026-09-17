// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package mail

import (
	"bufio"
	"context"
	"net"
	"net/textproto"
	"strings"
	"testing"
)

func startMockSMTPServer(t *testing.T) (int, func()) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start mock smtp server: %v", err)
	}

	port := l.Addr().(*net.TCPAddr).Port

	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go handleMockSMTPConn(conn)
		}
	}()

	return port, func() { _ = l.Close() }
}

func handleMockSMTPConn(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	writer := bufio.NewWriter(conn)
	reader := bufio.NewReader(conn)
	tp := textproto.NewReader(reader)

	// 220 Ready
	_, _ = writer.WriteString("220 mock.smtp.com SMTP Ready\r\n")
	_ = writer.Flush()

	for {
		line, err := tp.ReadLine()
		if err != nil {
			return
		}
		upper := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(upper, "EHLO") || strings.HasPrefix(upper, "HELO"):
			_, _ = writer.WriteString("250-mock.smtp.com\r\n250 AUTH PLAIN\r\n")
			_ = writer.Flush()
		case strings.HasPrefix(upper, "AUTH PLAIN"):
			_, _ = writer.WriteString("235 2.7.0 Authentication successful\r\n")
			_ = writer.Flush()
		case strings.HasPrefix(upper, "MAIL FROM:"):
			_, _ = writer.WriteString("250 2.1.0 Ok\r\n")
			_ = writer.Flush()
		case strings.HasPrefix(upper, "RCPT TO:"):
			_, _ = writer.WriteString("250 2.1.5 Ok\r\n")
			_ = writer.Flush()
		case strings.HasPrefix(upper, "DATA"):
			_, _ = writer.WriteString("354 Start mail input; end with <CRLF>.<CRLF>\r\n")
			_ = writer.Flush()
			for {
				dataLine, err := tp.ReadLine()
				if err != nil || dataLine == "." {
					break
				}
			}
			_, _ = writer.WriteString("250 2.0.0 Ok: queued\r\n")
			_ = writer.Flush()
		case strings.HasPrefix(upper, "QUIT"):
			_, _ = writer.WriteString("221 2.0.0 Bye\r\n")
			_ = writer.Flush()
			return
		default:
			_, _ = writer.WriteString("250 Ok\r\n")
			_ = writer.Flush()
		}
	}
}

func TestSendMailMock(t *testing.T) {
	port, cleanup := startMockSMTPServer(t)
	defer cleanup()

	cfg := Config{
		Host:     "127.0.0.1",
		Port:     port,
		Username: "test@example.com",
		Password: "password",
		FromName: "Wavelet Notifier",
	}

	err := SendMail(context.Background(), cfg, "recipient@example.com", "Test Subject", "<h1>Test Body</h1>")
	if err != nil {
		t.Fatalf("failed to send mail: %v", err)
	}
}

func TestSendMailWithLog(t *testing.T) {
	port, cleanup := startMockSMTPServer(t)
	defer cleanup()

	cfg := Config{
		Host:     "127.0.0.1",
		Port:     port,
		Username: "test@example.com",
		Password: "password",
	}

	logs, err := SendMailWithLog(context.Background(), cfg, "recipient@example.com", "Test Subject", "<p>Test Log</p>")
	if err != nil {
		t.Fatalf("failed to send mail with log: %v, log output:\n%s", err, logs)
	}

	if !strings.Contains(logs, "[System] Connecting to") {
		t.Errorf("expected connection log in output, got: %s", logs)
	}
	if !strings.Contains(logs, "[System] Mail sent successfully!") {
		t.Errorf("expected success log in output, got: %s", logs)
	}
}

func TestSendMailInvalidAddress(t *testing.T) {
	cfg := Config{
		Host:     "127.0.0.1",
		Port:     25,
		Username: "test@example.com",
		Password: "password",
	}

	err := SendMail(context.Background(), cfg, "invalid address with \n newline", "Subject", "Body")
	if err == nil {
		t.Errorf("expected error for invalid address, got nil")
	}
}
