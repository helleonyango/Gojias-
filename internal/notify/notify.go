// Package notify sends the workshop an email when a new order arrives.
//
// SMTP is configured through environment variables. When they are absent the
// notification is written to the log instead of failing the order — a customer
// should never lose an order because mail is misconfigured.
package notify

import (
	"fmt"
	"log"
	"net/smtp"
	"os"
	"strings"
)

// Mailer holds the SMTP settings read from the environment.
type Mailer struct {
	Host string
	Port string
	User string
	Pass string
	From string
	To   string
}

// FromEnv reads SMTP_HOST, SMTP_PORT, SMTP_USER, SMTP_PASS, MAIL_FROM and
// MAIL_TO. Only SMTP_HOST and MAIL_TO are required to enable sending.
func FromEnv() Mailer {
	m := Mailer{
		Host: os.Getenv("SMTP_HOST"),
		Port: os.Getenv("SMTP_PORT"),
		User: os.Getenv("SMTP_USER"),
		Pass: os.Getenv("SMTP_PASS"),
		From: os.Getenv("MAIL_FROM"),
		To:   os.Getenv("MAIL_TO"),
	}
	if m.Port == "" {
		m.Port = "587"
	}
	if m.From == "" {
		m.From = m.User
	}
	return m
}

// Configured reports whether the mailer has enough settings to send.
func (m Mailer) Configured() bool { return m.Host != "" && m.To != "" && m.From != "" }

// SendOrder emails the shop about a new order. It always logs a one-line
// summary, so local development shows exactly what would have been sent.
func (m Mailer) SendOrder(subject, body string) error {
	log.Printf("NEW ORDER NOTIFICATION: %s", strings.ReplaceAll(subject, "\n", " "))
	if !m.Configured() {
		log.Printf("smtp not configured; order notification logged only")
		return nil
	}
	return m.send(m.To, subject, body)
}

// SendTo emails a single recipient (used when the shop notifies a customer
// about a status change).
func (m Mailer) SendTo(to, subject, body string) error {
	if strings.TrimSpace(to) == "" {
		return fmt.Errorf("no recipient")
	}
	if !m.Configured() {
		log.Printf("smtp not configured; message to %s logged only: %s", to, subject)
		return nil
	}
	return m.send(to, subject, body)
}

func (m Mailer) send(to, subject, body string) error {
	recipients := strings.Split(to, ",")
	for i := range recipients {
		recipients[i] = strings.TrimSpace(recipients[i])
	}

	headers := []string{
		"From: " + m.From,
		"To: " + strings.Join(recipients, ", "),
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
	}
	msg := []byte(strings.Join(headers, "\r\n") + "\r\n\r\n" + body + "\r\n")

	var auth smtp.Auth
	if m.User != "" {
		auth = smtp.PlainAuth("", m.User, m.Pass, m.Host)
	}
	addr := m.Host + ":" + m.Port
	if err := smtp.SendMail(addr, auth, m.From, recipients, msg); err != nil {
		return fmt.Errorf("send mail: %w", err)
	}
	log.Printf("notification emailed to %s", to)
	return nil
}
