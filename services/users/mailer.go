package main

import (
	"errors"
	"net"
	"net/smtp"
	"strings"
)

type Mailer struct {
	host string
	port string
	from string
}

func (m *Mailer) Send(to, subject, body string) error {
	if strings.ContainsAny(to+subject, "\r\n") {
		return errors.New("invalid characters in mail header")
	}
	msg := "From: " + m.from + "\r\n" +
		"To: " + to + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"\r\n" + body
	return smtp.SendMail(net.JoinHostPort(m.host, m.port), nil, m.from, []string{to}, []byte(msg))
}
