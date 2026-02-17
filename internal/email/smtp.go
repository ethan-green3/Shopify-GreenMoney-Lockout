package email

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"
	"time"
)

type SMTPConfig struct {
	Host string
	Port int
	User string
	Pass string
	From string
}

func Send(cfg SMTPConfig, to, subject, body string) error {
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	// Connect to SMTP server
	conn, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}
	defer conn.Close()

	// STARTTLS
	if ok, _ := conn.Extension("STARTTLS"); ok {
		tlsConfig := &tls.Config{
			ServerName: cfg.Host,
		}
		if err = conn.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("starttls: %w", err)
		}
	}

	// Auth
	auth := smtp.PlainAuth("", cfg.User, cfg.Pass, cfg.Host)
	if ok, _ := conn.Extension("AUTH"); ok {
		if err = conn.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}

	// From
	if err = conn.Mail(cfg.User); err != nil {
		return fmt.Errorf("mail from: %w", err)
	}

	// To
	if err = conn.Rcpt(to); err != nil {
		return fmt.Errorf("rcpt to: %w", err)
	}

	// Data
	w, err := conn.Data()
	if err != nil {
		return fmt.Errorf("data: %w", err)
	}

	message := buildMessage(cfg.From, to, subject, body)

	_, err = w.Write([]byte(message))
	if err != nil {
		return fmt.Errorf("write body: %w", err)
	}

	if err = w.Close(); err != nil {
		return fmt.Errorf("close writer: %w", err)
	}

	return nil
}

func buildMessage(from, to, subject, body string) string {
	headers := map[string]string{
		"From":         from,
		"To":           to,
		"Subject":      subject,
		"Reply-To":     from,
		"Date":         time.Now().Format(time.RFC1123Z),
		"MIME-Version": "1.0",
		"Content-Type": "text/plain; charset=UTF-8",
	}

	var msg strings.Builder
	for k, v := range headers {
		msg.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	msg.WriteString("\r\n")
	msg.WriteString(body)

	return msg.String()
}
