// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package mail

import (
	"bufio"
	"context"
	"net"
	"net/textproto"
	"testing"
)

func TestSendMailMock(t *testing.T) {
	// Start a mock SMTP server
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start mock smtp server: %v", err)
	}
	defer func() { _ = l.Close() }()

	port := l.Addr().(*net.TCPAddr).Port

	go func() {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()

		writer := bufio.NewWriter(conn)
		reader := bufio.NewReader(conn)
		tp := textproto.NewReader(reader)

		// 220 Ready
		_, _ = writer.WriteString("220 mock.smtp.com SMTP Ready\r\n")
		_ = writer.Flush()

		// Read HELO/EHLO
		_, _ = tp.ReadLine()
		_, _ = writer.WriteString("250-mock.smtp.com\r\n250 AUTH PLAIN\r\n")
		_ = writer.Flush()

		// Read AUTH PLAIN
		_, _ = tp.ReadLine()
		_, _ = writer.WriteString("235 Authentication successful\r\n")
		_ = writer.Flush()

		// Read MAIL FROM
		_, _ = tp.ReadLine()
		_, _ = writer.WriteString("250 OK\r\n")
		_ = writer.Flush()

		// Read RCPT TO
		_, _ = tp.ReadLine()
		_, _ = writer.WriteString("250 OK\r\n")
		_ = writer.Flush()

		// Read DATA
		_, _ = tp.ReadLine()
		_, _ = writer.WriteString("354 Start mail input\r\n")
		_ = writer.Flush()

		// Read body lines until dot
		for {
			line, err := tp.ReadLine()
			if err != nil || line == "." {
				break
			}
		}
		_, _ = writer.WriteString("250 OK\r\n")
		_ = writer.Flush()

		// Read QUIT
		_, _ = tp.ReadLine()
		_, _ = writer.WriteString("221 Bye\r\n")
		_ = writer.Flush()
	}()

	cfg := Config{
		Host:     "127.0.0.1",
		Port:     port,
		Username: "test@example.com",
		Password: "password",
	}

	err = SendMail(context.Background(), cfg, "recipient@example.com", "Test Subject", "<h1>Test Body</h1>")
	if err != nil {
		t.Errorf("failed to send mail: %v", err)
	}
}
