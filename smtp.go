package gomail

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	smtpDefaultPort      = 587         // mail submission port (STARTTLS)
	smtpImplicitTLSPort  = 465         // SMTPS port (implicit TLS)
	smtpDefaultLocalName = "localhost" // name sent with EHLO/HELO when none is configured
)

// SMTPConfig configures an SMTPProvider
//
// DO NOT CHANGE ORDER - Optimized for memory (maligned)
type SMTPConfig struct {
	TLSConfig   *tls.Config // optional TLS settings (ServerName defaults to Host)
	Host        string      // ie: smtp.example.com
	LocalName   string      // name sent with EHLO/HELO (default: localhost)
	Password    string      // ie: secretPassword
	Username    string      // when empty, no authentication is attempted (ie: a local relay)
	Port        int         // default 587, or 465 when ImplicitTLS is set
	ImplicitTLS bool        // connect over TLS (SMTPS); always on for port 465. Otherwise STARTTLS is used when offered
}

// SMTPProvider sends email over SMTP.
//
// Every send opens a new connection that honors the context deadline and
// cancellation. STARTTLS is used whenever the server offers it, and
// credentials are never sent over an unencrypted connection (except to
// localhost). PLAIN authentication is preferred, with LOGIN as a fallback for
// servers that only offer LOGIN.
type SMTPProvider struct {
	now    func() time.Time
	config SMTPConfig
}

// NewSMTPProvider creates an SMTP provider from the given configuration
func NewSMTPProvider(config SMTPConfig) *SMTPProvider {
	if config.Port == 0 {
		config.Port = smtpDefaultPort
		if config.ImplicitTLS {
			config.Port = smtpImplicitTLSPort
		}
	}
	if config.Port == smtpImplicitTLSPort {
		config.ImplicitTLS = true
	}
	if len(config.LocalName) == 0 {
		config.LocalName = smtpDefaultLocalName
	}
	return &SMTPProvider{config: config, now: time.Now}
}

// Send sends the email over SMTP; the result MessageID is the Message-ID header
func (p *SMTPProvider) Send(ctx context.Context, email *Email) (*SendResult, error) {
	env, err := parseEnvelope(email)
	if err != nil {
		return nil, err
	}

	var attachments []attachmentData
	if attachments, err = readAttachments(email); err != nil {
		return nil, err
	}

	// Use the caller's Message-ID if one was set, otherwise generate one
	messageID, generatedID := customMessageID(email), ""
	if len(messageID) == 0 {
		if generatedID, err = newMessageID(&env.from); err != nil {
			return nil, err
		}
		messageID = generatedID
	}

	var message []byte
	if message, err = buildMIME(email, env, attachments, generatedID, p.now()); err != nil {
		return nil, err
	}

	if err = p.deliver(ctx, env.from.Address, env.allRecipients(), message); err != nil {
		return nil, err
	}
	return &SendResult{MessageID: messageID}, nil
}

// SupportsFeature reports whether SMTP supports the feature (it supports none)
func (p *SMTPProvider) SupportsFeature(_ Feature) bool {
	return false
}

// deliver runs one SMTP transaction for the message
func (p *SMTPProvider) deliver(ctx context.Context, from string, recipients []string, message []byte) error {
	conn, err := p.dial(ctx)
	if err != nil {
		return smtpError(ctx, "connect", err)
	}

	// Bound every read/write by the context: its deadline, and cancellation
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	stop := context.AfterFunc(ctx, func() { _ = conn.SetDeadline(time.Unix(1, 0)) })
	defer stop()

	client, err := smtp.NewClient(conn, p.config.Host)
	if err != nil {
		_ = conn.Close()
		return smtpError(ctx, "greeting", err)
	}
	defer func() { _ = client.Close() }()

	if err = p.startSession(client); err != nil {
		return smtpError(ctx, "session", err)
	}

	if err = client.Mail(from); err != nil {
		return smtpError(ctx, "MAIL FROM", err)
	}
	for _, recipient := range recipients {
		if err = client.Rcpt(recipient); err != nil {
			return smtpError(ctx, "RCPT TO", err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return smtpError(ctx, "DATA", err)
	}
	if _, err = w.Write(message); err != nil {
		_ = w.Close()
		return smtpError(ctx, "DATA", err)
	}
	if err = w.Close(); err != nil {
		return smtpError(ctx, "DATA", err)
	}

	// The message has been accepted; a failed QUIT does not un-send it
	_ = client.Quit()
	return nil
}

// dial connects to the server, over TLS when implicit TLS is enabled
func (p *SMTPProvider) dial(ctx context.Context) (net.Conn, error) {
	addr := net.JoinHostPort(p.config.Host, strconv.Itoa(p.config.Port))
	if p.config.ImplicitTLS {
		dialer := &tls.Dialer{Config: p.tlsConfig()}
		return dialer.DialContext(ctx, "tcp", addr)
	}
	dialer := &net.Dialer{}
	return dialer.DialContext(ctx, "tcp", addr)
}

// startSession sends EHLO, upgrades to TLS when offered, and authenticates
func (p *SMTPProvider) startSession(client *smtp.Client) error {
	if err := client.Hello(p.config.LocalName); err != nil {
		return err
	}

	if !p.config.ImplicitTLS {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(p.tlsConfig()); err != nil {
				return err
			}
		}
	}

	if len(p.config.Username) == 0 {
		return nil
	}
	if ok, _ := client.Extension("AUTH"); !ok {
		return fmt.Errorf("%w: credentials are configured but the server does not support AUTH", ErrSMTPError)
	}
	return client.Auth(p.auth(client))
}

// auth picks the authentication mechanism: LOGIN when it is offered and PLAIN
// is not, otherwise PLAIN
func (p *SMTPProvider) auth(client *smtp.Client) smtp.Auth {
	_, advertised := client.Extension("AUTH")
	mechanisms := strings.Fields(strings.ToUpper(advertised))
	if !slices.Contains(mechanisms, "PLAIN") && slices.Contains(mechanisms, "LOGIN") {
		return &smtpLoginAuth{host: p.config.Host, password: p.config.Password, username: p.config.Username}
	}
	return smtp.PlainAuth("", p.config.Username, p.config.Password, p.config.Host)
}

// tlsConfig returns the TLS configuration for the server
func (p *SMTPProvider) tlsConfig() *tls.Config {
	var config *tls.Config
	if p.config.TLSConfig != nil {
		config = p.config.TLSConfig.Clone()
	} else {
		config = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	if len(config.ServerName) == 0 {
		config.ServerName = p.config.Host
	}
	return config
}

// smtpError wraps an SMTP failure with the stage it happened in (and the
// context error, when the context ended the transaction)
func smtpError(ctx context.Context, stage string, err error) error {
	if ctxErr := contextError(ctx); ctxErr != nil {
		return fmt.Errorf("%w: %s: %w: %w", ErrSMTPError, stage, ctxErr, err)
	}
	return fmt.Errorf("%w: %s: %w", ErrSMTPError, stage, err)
}

// contextError returns the context error, treating a passed deadline as
// exceeded (the connection deadline can fire just before the context reports it)
func contextError(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if deadline, ok := ctx.Deadline(); ok && !time.Now().Before(deadline) {
		return context.DeadlineExceeded
	}
	return nil
}

// smtpLoginAuth implements the LOGIN authentication mechanism, which some
// servers (ie: Microsoft 365) offer instead of PLAIN
type smtpLoginAuth struct {
	host     string
	password string
	username string
}

// Start begins LOGIN authentication, refusing to send credentials in the clear
func (a *smtpLoginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	if !server.TLS && !isLocalhost(server.Name) {
		return "", nil, fmt.Errorf("%w: refusing to authenticate over an unencrypted connection", ErrSMTPError)
	}
	if server.Name != a.host {
		return "", nil, fmt.Errorf("%w: wrong host name %q", ErrSMTPError, server.Name)
	}
	return "LOGIN", nil, nil
}

// Next answers the server's username and password challenges
func (a *smtpLoginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}

	challenge := strings.ToLower(string(fromServer))
	switch {
	case strings.Contains(challenge, "user"):
		return []byte(a.username), nil
	case strings.Contains(challenge, "pass"):
		return []byte(a.password), nil
	default:
		return nil, fmt.Errorf("%w: unexpected LOGIN challenge %q", ErrSMTPError, fromServer)
	}
}

// isLocalhost reports whether the host is the local machine
func isLocalhost(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}
