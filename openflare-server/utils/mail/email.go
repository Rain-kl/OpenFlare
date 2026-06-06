package mail

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net/smtp"
	"strings"
)

// SMTPConfig holds all the configuration parameters required to send an email.
type SMTPConfig struct {
	Server     string
	Port       int
	Account    string
	Token      string
	SystemName string
}

// SendEmail sends an HTML email to the receiver using the provided SMTP configuration.
func SendEmail(config SMTPConfig, subject string, receiver string, content string) error {
	encodedSubject := fmt.Sprintf("=?UTF-8?B?%s?=", base64.StdEncoding.EncodeToString([]byte(subject)))
	mail := []byte(fmt.Sprintf("To: %s\r\n"+
		"From: %s<%s>\r\n"+
		"Subject: %s\r\n"+
		"Content-Type: text/html; charset=UTF-8\r\n\r\n%s\r\n",
		receiver, config.SystemName, config.Account, encodedSubject, content))
	auth := smtp.PlainAuth("", config.Account, config.Token, config.Server)
	addr := fmt.Sprintf("%s:%d", config.Server, config.Port)
	to := strings.Split(receiver, ";")
	var err error
	if config.Port == 465 {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         config.Server,
		}
		conn, err := tls.Dial("tcp", fmt.Sprintf("%s:%d", config.Server, config.Port), tlsConfig)
		if err != nil {
			return err
		}
		client, err := smtp.NewClient(conn, config.Server)
		if err != nil {
			return err
		}
		defer client.Close()
		if err = client.Auth(auth); err != nil {
			return err
		}
		if err = client.Mail(config.Account); err != nil {
			return err
		}
		receiverEmails := strings.Split(receiver, ";")
		for _, r := range receiverEmails {
			if err = client.Rcpt(r); err != nil {
				return err
			}
		}
		w, err := client.Data()
		if err != nil {
			return err
		}
		_, err = w.Write(mail)
		if err != nil {
			return err
		}
		err = w.Close()
		if err != nil {
			return err
		}
	} else {
		err = smtp.SendMail(addr, auth, config.Account, to, mail)
	}
	return err
}
