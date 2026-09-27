package gomail

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"errors"
	"io"
	"math/big"
	"net"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fake SMTP server credentials
const (
	testSMTPUsername = "smtp-user"
	testSMTPPassword = "smtp-pass"
)

// fakeSMTPConfig configures the fake SMTP server behavior
type fakeSMTPConfig struct {
	tlsConfig  *tls.Config // enables STARTTLS, or implicit TLS when implicit is set
	authMechs  string      // advertised AUTH mechanisms (empty: no AUTH extension)
	rejectRcpt string      // recipient rejected with a 550
	implicit   bool        // serve TLS from the first byte
	rejectMail bool        // reject MAIL FROM
	rejectData bool        // reject DATA
	rejectBody bool        // reject the message after it is transferred
	rejectHelo bool        // reject EHLO and HELO
	silent     bool        // never send the greeting
}

// fakeSMTPSession records one SMTP transaction
type fakeSMTPSession struct {
	from   string
	helo   string
	data   string
	rcpts  []string
	authed bool
	tls    bool
}

// fakeSMTPServer is a minimal in-process SMTP server
type fakeSMTPServer struct {
	listener net.Listener
	sessions []fakeSMTPSession
	cfg      fakeSMTPConfig
	wg       sync.WaitGroup
	mu       sync.Mutex
}

// startFakeSMTPServer starts a fake SMTP server on a random localhost port
func startFakeSMTPServer(t *testing.T, cfg fakeSMTPConfig) *fakeSMTPServer {
	t.Helper()

	listener, err := new(net.ListenConfig).Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	if cfg.implicit {
		listener = tls.NewListener(listener, cfg.tlsConfig)
	}

	server := &fakeSMTPServer{cfg: cfg, listener: listener}
	server.wg.Add(1)
	go server.serve()
	t.Cleanup(func() {
		_ = listener.Close()
		server.wg.Wait()
	})
	return server
}

// port returns the port the server listens on
func (s *fakeSMTPServer) port() int {
	return listenerPort(s.listener)
}

// listenerPort returns the port of a listener (0 if it cannot be determined)
func listenerPort(listener net.Listener) int {
	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		return 0
	}
	number, _ := strconv.Atoi(port)
	return number
}

// session returns the only recorded session
func (s *fakeSMTPServer) session(t *testing.T) fakeSMTPSession {
	t.Helper()

	s.mu.Lock()
	defer s.mu.Unlock()
	require.Len(t, s.sessions, 1)
	return s.sessions[0]
}

// serve accepts connections until the listener is closed
func (s *fakeSMTPServer) serve() {
	defer s.wg.Done()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handle(conn)
		}()
	}
}

// fakeSMTPConn is the state of one fake SMTP connection
type fakeSMTPConn struct {
	conn    net.Conn
	tp      *textproto.Conn
	session fakeSMTPSession
}

// handle runs one SMTP session
func (s *fakeSMTPServer) handle(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))

	if s.cfg.silent {
		_, _ = bufio.NewReader(conn).ReadString('\n')
		return
	}

	c := &fakeSMTPConn{conn: conn, tp: textproto.NewConn(conn), session: fakeSMTPSession{tls: s.cfg.implicit}}
	defer func() {
		s.mu.Lock()
		s.sessions = append(s.sessions, c.session)
		s.mu.Unlock()
	}()

	_ = c.tp.PrintfLine("220 fake ESMTP ready")
	for {
		line, err := c.tp.ReadLine()
		if err != nil {
			return
		}
		verb, arg, _ := strings.Cut(line, " ")
		if done := s.command(c, strings.ToUpper(verb), arg); done {
			return
		}
	}
}

// command handles one SMTP command, returning true when the session is over
func (s *fakeSMTPServer) command(c *fakeSMTPConn, verb, arg string) bool {
	switch verb {
	case "EHLO", "HELO":
		if s.cfg.rejectHelo {
			_ = c.tp.PrintfLine("550 go away")
			return false
		}
		c.session.helo = arg
		s.writeExtensions(c.tp, c.session.tls)
	case "STARTTLS":
		_ = c.tp.PrintfLine("220 ready to start TLS")
		tlsConn := tls.Server(c.conn, s.cfg.tlsConfig)
		if err := tlsConn.HandshakeContext(context.Background()); err != nil {
			return true
		}
		c.conn, c.tp, c.session.tls = tlsConn, textproto.NewConn(tlsConn), true
	case "AUTH":
		c.session.authed = s.authenticate(c.tp, arg)
	case "MAIL":
		s.mailFrom(c, arg)
	case "RCPT":
		s.rcptTo(c, arg)
	case "DATA":
		return s.data(c)
	case "QUIT":
		_ = c.tp.PrintfLine("221 bye")
		return true
	default:
		_ = c.tp.PrintfLine("502 command not implemented")
	}
	return false
}

// mailFrom handles MAIL FROM
func (s *fakeSMTPServer) mailFrom(c *fakeSMTPConn, arg string) {
	if s.cfg.rejectMail {
		_ = c.tp.PrintfLine("550 sender rejected")
		return
	}
	c.session.from = extractPath(arg)
	_ = c.tp.PrintfLine("250 OK")
}

// rcptTo handles RCPT TO
func (s *fakeSMTPServer) rcptTo(c *fakeSMTPConn, arg string) {
	rcpt := extractPath(arg)
	if rcpt == s.cfg.rejectRcpt {
		_ = c.tp.PrintfLine("550 no such user")
		return
	}
	c.session.rcpts = append(c.session.rcpts, rcpt)
	_ = c.tp.PrintfLine("250 OK")
}

// data handles DATA, returning true when the connection failed
func (s *fakeSMTPServer) data(c *fakeSMTPConn) bool {
	if s.cfg.rejectData {
		_ = c.tp.PrintfLine("554 transaction failed")
		return false
	}
	_ = c.tp.PrintfLine("354 end data with <CR><LF>.<CR><LF>")
	data, err := c.tp.ReadDotBytes()
	if err != nil {
		return true
	}
	c.session.data = string(data)
	if s.cfg.rejectBody {
		_ = c.tp.PrintfLine("554 message rejected as spam")
		return false
	}
	_ = c.tp.PrintfLine("250 OK queued")
	return false
}

// writeExtensions answers EHLO with the configured extensions
func (s *fakeSMTPServer) writeExtensions(tp *textproto.Conn, isTLS bool) {
	lines := []string{"fake.example.com greets you"}
	if s.cfg.tlsConfig != nil && !isTLS {
		lines = append(lines, "STARTTLS")
	}
	if len(s.cfg.authMechs) > 0 {
		lines = append(lines, "AUTH "+s.cfg.authMechs)
	}
	for i, line := range lines {
		sep := "-"
		if i == len(lines)-1 {
			sep = " "
		}
		_ = tp.PrintfLine("250%s%s", sep, line)
	}
}

// authenticate handles AUTH PLAIN and AUTH LOGIN
func (s *fakeSMTPServer) authenticate(tp *textproto.Conn, arg string) bool {
	mechanism, initial, _ := strings.Cut(arg, " ")
	var username, password string
	switch strings.ToUpper(mechanism) {
	case "PLAIN":
		decoded, err := base64.StdEncoding.DecodeString(initial)
		if err != nil {
			_ = tp.PrintfLine("501 bad encoding")
			return false
		}
		parts := strings.Split(string(decoded), "\x00")
		if len(parts) == 3 {
			username, password = parts[1], parts[2]
		}
	case "LOGIN":
		username = s.loginPrompt(tp, "Username:")
		password = s.loginPrompt(tp, "Password:")
	default:
		_ = tp.PrintfLine("504 unrecognized mechanism")
		return false
	}

	if username != testSMTPUsername || password != testSMTPPassword {
		_ = tp.PrintfLine("535 authentication failed")
		return false
	}
	_ = tp.PrintfLine("235 authentication succeeded")
	return true
}

// loginPrompt sends a LOGIN challenge and returns the decoded answer
func (s *fakeSMTPServer) loginPrompt(tp *textproto.Conn, prompt string) string {
	_ = tp.PrintfLine("334 %s", base64.StdEncoding.EncodeToString([]byte(prompt)))
	line, err := tp.ReadLine()
	if err != nil {
		return ""
	}
	decoded, err := base64.StdEncoding.DecodeString(line)
	if err != nil {
		return ""
	}
	return string(decoded)
}

// extractPath returns the address in a MAIL FROM:<...> or RCPT TO:<...> argument
func extractPath(arg string) string {
	start, end := strings.IndexByte(arg, '<'), strings.IndexByte(arg, '>')
	if start < 0 || end < start {
		return ""
	}
	return arg[start+1 : end]
}

// newTestTLSConfigs returns a server TLS config with a self-signed certificate
// for 127.0.0.1, and a client TLS config that trusts it
func newTestTLSConfigs(t *testing.T) (*tls.Config, *tls.Config) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		NotAfter:              time.Now().Add(time.Hour),
		NotBefore:             time.Now().Add(-time.Hour),
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "go-mail test"},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)

	pool := x509.NewCertPool()
	pool.AddCert(cert)

	server := &tls.Config{
		Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}},
		MinVersion:   tls.VersionTLS12,
	}
	client := &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: pool}
	return server, client
}

// newTestSMTPProvider returns an SMTP provider pointed at the fake server
func newTestSMTPProvider(server *fakeSMTPServer, config SMTPConfig) *SMTPProvider {
	config.Host = "127.0.0.1"
	config.Port = server.port()
	provider := NewSMTPProvider(config)
	provider.now = testMIMEDate
	return provider
}

// newSMTPTestEmail returns an email with to, cc and bcc recipients
func newSMTPTestEmail() *Email {
	email := newValidEmail()
	email.HTMLContent = "<p>Hello</p>"
	email.Recipients = []string{"Recipient <to@example.com>"}
	email.RecipientsCc = []string{testCcAddress}
	email.RecipientsBcc = []string{testBccAddress}
	return email
}

// TestNewSMTPProvider checks the configuration defaults
func TestNewSMTPProvider(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		config       SMTPConfig
		expectedPort int
		expectedTLS  bool
	}{
		{"default port is submission", SMTPConfig{Host: "h"}, smtpDefaultPort, false},
		{"implicit tls defaults to 465", SMTPConfig{Host: "h", ImplicitTLS: true}, smtpImplicitTLSPort, true},
		{"port 465 enables implicit tls", SMTPConfig{Host: "h", Port: 465}, smtpImplicitTLSPort, true},
		{"explicit port is kept", SMTPConfig{Host: "h", Port: 25}, 25, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			provider := NewSMTPProvider(test.config)
			assert.Equal(t, test.expectedPort, provider.config.Port)
			assert.Equal(t, test.expectedTLS, provider.config.ImplicitTLS)
			assert.Equal(t, smtpDefaultLocalName, provider.config.LocalName)
		})
	}

	t.Run("local name is kept", func(t *testing.T) {
		assert.Equal(t, "mail.example.com", NewSMTPProvider(SMTPConfig{LocalName: "mail.example.com"}).config.LocalName)
	})
}

// TestSMTPProviderSendPlain sends through a server without TLS or authentication
func TestSMTPProviderSendPlain(t *testing.T) {
	t.Parallel()

	server := startFakeSMTPServer(t, fakeSMTPConfig{})
	provider := newTestSMTPProvider(server, SMTPConfig{LocalName: "client.example.com"})

	email := newSMTPTestEmail()
	result, err := provider.Send(context.Background(), email)
	require.NoError(t, err)
	assert.Regexp(t, `^<[0-9a-f]{32}@example\.com>$`, result.MessageID)

	session := server.session(t)
	assert.Equal(t, "client.example.com", session.helo)
	assert.Equal(t, "no-reply@example.com", session.from)
	assert.Equal(t, []string{"to@example.com", testCcAddress, testBccAddress}, session.rcpts)
	assert.False(t, session.authed)

	parsed := parseMIME(t, []byte(session.data))
	assert.Equal(t, result.MessageID, parsed.header.Get("Message-Id"))
	assert.Empty(t, parsed.header.Get("Bcc"), "bcc recipients must not be disclosed")
	assert.NotContains(t, session.data, testBccAddress)
	assert.Equal(t, "<p>Hello</p>", parsed.part(t, mimeTypeHTML).content)
}

// TestSMTPProviderCustomMessageID checks that a Message-ID header is kept
func TestSMTPProviderCustomMessageID(t *testing.T) {
	t.Parallel()

	server := startFakeSMTPServer(t, fakeSMTPConfig{})
	provider := newTestSMTPProvider(server, SMTPConfig{})

	email := newSMTPTestEmail()
	email.SetHeader("Message-ID", "<custom@example.com>")
	result, err := provider.Send(context.Background(), email)
	require.NoError(t, err)
	assert.Equal(t, "<custom@example.com>", result.MessageID)

	parsed := parseMIME(t, []byte(server.session(t).data))
	assert.Equal(t, []string{"<custom@example.com>"}, parsed.header["Message-Id"])
}

// TestSMTPProviderStartTLSAuth checks STARTTLS followed by authentication
func TestSMTPProviderStartTLSAuth(t *testing.T) {
	t.Parallel()

	serverTLS, clientTLS := newTestTLSConfigs(t)
	tests := []struct {
		name      string
		authMechs string
	}{
		{"plain", "PLAIN LOGIN"},
		{"login fallback", "LOGIN"},
		{"plain only", "PLAIN"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := startFakeSMTPServer(t, fakeSMTPConfig{tlsConfig: serverTLS, authMechs: test.authMechs})
			provider := newTestSMTPProvider(server, SMTPConfig{
				Password:  testSMTPPassword,
				TLSConfig: clientTLS,
				Username:  testSMTPUsername,
			})

			_, err := provider.Send(context.Background(), newSMTPTestEmail())
			require.NoError(t, err)

			session := server.session(t)
			assert.True(t, session.tls, "the connection must be upgraded with STARTTLS")
			assert.True(t, session.authed)
		})
	}
}

// TestSMTPProviderImplicitTLS checks SMTPS (TLS from the first byte)
func TestSMTPProviderImplicitTLS(t *testing.T) {
	t.Parallel()

	serverTLS, clientTLS := newTestTLSConfigs(t)
	server := startFakeSMTPServer(t, fakeSMTPConfig{tlsConfig: serverTLS, implicit: true, authMechs: "PLAIN"})
	provider := newTestSMTPProvider(server, SMTPConfig{
		ImplicitTLS: true,
		Password:    testSMTPPassword,
		TLSConfig:   clientTLS,
		Username:    testSMTPUsername,
	})

	_, err := provider.Send(context.Background(), newSMTPTestEmail())
	require.NoError(t, err)

	session := server.session(t)
	assert.True(t, session.tls)
	assert.True(t, session.authed)
}

// TestSMTPProviderRejectsUntrustedCertificate checks that TLS certificates are verified
func TestSMTPProviderRejectsUntrustedCertificate(t *testing.T) {
	t.Parallel()

	serverTLS, _ := newTestTLSConfigs(t)
	server := startFakeSMTPServer(t, fakeSMTPConfig{tlsConfig: serverTLS})
	provider := newTestSMTPProvider(server, SMTPConfig{})

	_, err := provider.Send(context.Background(), newSMTPTestEmail())
	require.ErrorIs(t, err, ErrSMTPError)
	var certErr *tls.CertificateVerificationError
	assert.ErrorAs(t, err, &certErr)
}

// TestSMTPProviderErrors checks the SMTP failure paths
func TestSMTPProviderErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cfg      fakeSMTPConfig
		config   SMTPConfig
		contains string
	}{
		{"sender rejected", fakeSMTPConfig{rejectMail: true}, SMTPConfig{}, "MAIL FROM"},
		{"recipient rejected", fakeSMTPConfig{rejectRcpt: testBccAddress}, SMTPConfig{}, "RCPT TO"},
		{"data rejected", fakeSMTPConfig{rejectData: true}, SMTPConfig{}, "DATA"},
		{"message rejected", fakeSMTPConfig{rejectBody: true}, SMTPConfig{}, "rejected as spam"},
		{"hello rejected", fakeSMTPConfig{rejectHelo: true}, SMTPConfig{}, "go away"},
		{"bad credentials", fakeSMTPConfig{authMechs: "PLAIN"}, SMTPConfig{Username: testSMTPUsername, Password: "wrong"}, "535"},
		{"server without auth", fakeSMTPConfig{}, SMTPConfig{Username: testSMTPUsername, Password: testSMTPPassword}, "AUTH"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := startFakeSMTPServer(t, test.cfg)
			provider := newTestSMTPProvider(server, test.config)

			_, err := provider.Send(context.Background(), newSMTPTestEmail())
			require.ErrorIs(t, err, ErrSMTPError)
			assert.Contains(t, err.Error(), test.contains)
		})
	}
}

// TestSMTPProviderConnectError checks a failed connection
func TestSMTPProviderConnectError(t *testing.T) {
	t.Parallel()

	listener, err := new(net.ListenConfig).Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listenerPort(listener)
	require.NoError(t, listener.Close())

	provider := NewSMTPProvider(SMTPConfig{Host: "127.0.0.1", Port: port})
	_, err = provider.Send(context.Background(), newSMTPTestEmail())
	require.ErrorIs(t, err, ErrSMTPError)
	assert.Contains(t, err.Error(), "connect")
}

// TestSMTPProviderHonorsContext checks that a hung server cannot block a send
func TestSMTPProviderHonorsContext(t *testing.T) {
	t.Parallel()

	t.Run("deadline", func(t *testing.T) {
		server := startFakeSMTPServer(t, fakeSMTPConfig{silent: true})
		provider := newTestSMTPProvider(server, SMTPConfig{})

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		start := time.Now()
		_, err := provider.Send(ctx, newSMTPTestEmail())
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.ErrorIs(t, err, ErrSMTPError)
		assert.Less(t, time.Since(start), 5*time.Second)
	})

	t.Run("cancel", func(t *testing.T) {
		server := startFakeSMTPServer(t, fakeSMTPConfig{silent: true})
		provider := newTestSMTPProvider(server, SMTPConfig{})

		ctx, cancel := context.WithCancel(context.Background())
		timer := time.AfterFunc(100*time.Millisecond, cancel)
		defer timer.Stop()

		_, err := provider.Send(ctx, newSMTPTestEmail())
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("already canceled", func(t *testing.T) {
		server := startFakeSMTPServer(t, fakeSMTPConfig{})
		provider := newTestSMTPProvider(server, SMTPConfig{})

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := provider.Send(ctx, newSMTPTestEmail())
		require.ErrorIs(t, err, context.Canceled)
	})
}

// TestSMTPProviderInvalidEmail checks that envelope and attachment errors are returned
func TestSMTPProviderInvalidEmail(t *testing.T) {
	t.Parallel()

	provider := NewSMTPProvider(SMTPConfig{Host: "127.0.0.1"})

	badFrom := newSMTPTestEmail()
	badFrom.FromAddress = "invalid"
	_, err := provider.Send(context.Background(), badFrom)
	require.ErrorIs(t, err, ErrInvalidFromAddress)

	badAttachment := newSMTPTestEmail()
	badAttachment.AddAttachment(testFileName, "text/plain", errReader{})
	_, err = provider.Send(context.Background(), badAttachment)
	require.ErrorIs(t, err, io.ErrUnexpectedEOF)
}

// TestSMTPProviderSupportsFeature checks that SMTP reports no optional features
func TestSMTPProviderSupportsFeature(t *testing.T) {
	t.Parallel()

	provider := NewSMTPProvider(SMTPConfig{})
	for _, feature := range []Feature{FeatureAutoText, FeatureTags, FeatureTrackClicks, FeatureTrackOpens, FeatureSendAt} {
		assert.False(t, provider.SupportsFeature(feature), feature)
	}
}

// TestSMTPProviderTLSConfig checks the TLS configuration defaults
func TestSMTPProviderTLSConfig(t *testing.T) {
	t.Parallel()

	t.Run("default", func(t *testing.T) {
		config := NewSMTPProvider(SMTPConfig{Host: "smtp.example.com"}).tlsConfig()
		assert.Equal(t, "smtp.example.com", config.ServerName)
		assert.Equal(t, uint16(tls.VersionTLS12), config.MinVersion)
	})

	t.Run("custom config is cloned", func(t *testing.T) {
		custom := &tls.Config{MinVersion: tls.VersionTLS13}
		config := NewSMTPProvider(SMTPConfig{Host: "smtp.example.com", TLSConfig: custom}).tlsConfig()
		assert.Equal(t, "smtp.example.com", config.ServerName)
		assert.Empty(t, custom.ServerName, "the caller's config must not be modified")
	})

	t.Run("custom server name is kept", func(t *testing.T) {
		custom := &tls.Config{MinVersion: tls.VersionTLS13, ServerName: "other.example.com"}
		config := NewSMTPProvider(SMTPConfig{Host: "10.0.0.1", TLSConfig: custom}).tlsConfig()
		assert.Equal(t, "other.example.com", config.ServerName)
	})
}

// TestSMTPLoginAuth checks the LOGIN authentication mechanism
func TestSMTPLoginAuth(t *testing.T) {
	t.Parallel()

	auth := &smtpLoginAuth{host: "smtp.example.com", password: "pass", username: "user"}

	t.Run("refuses unencrypted connections", func(t *testing.T) {
		_, _, err := auth.Start(&smtp.ServerInfo{Name: "smtp.example.com", TLS: false})
		require.ErrorIs(t, err, ErrSMTPError)
		assert.Contains(t, err.Error(), "unencrypted")
	})

	t.Run("allows unencrypted localhost", func(t *testing.T) {
		local := &smtpLoginAuth{host: "localhost"}
		mechanism, _, err := local.Start(&smtp.ServerInfo{Name: "localhost"})
		require.NoError(t, err)
		assert.Equal(t, "LOGIN", mechanism)
	})

	t.Run("rejects the wrong host", func(t *testing.T) {
		_, _, err := auth.Start(&smtp.ServerInfo{Name: "evil.example.com", TLS: true})
		require.ErrorIs(t, err, ErrSMTPError)
	})

	t.Run("answers challenges", func(t *testing.T) {
		mechanism, initial, err := auth.Start(&smtp.ServerInfo{Name: "smtp.example.com", TLS: true})
		require.NoError(t, err)
		assert.Equal(t, "LOGIN", mechanism)
		assert.Nil(t, initial)

		username, err := auth.Next([]byte("Username:"), true)
		require.NoError(t, err)
		assert.Equal(t, "user", string(username))

		password, err := auth.Next([]byte("Password:"), true)
		require.NoError(t, err)
		assert.Equal(t, "pass", string(password))

		done, err := auth.Next(nil, false)
		require.NoError(t, err)
		assert.Nil(t, done)

		_, err = auth.Next([]byte("Something else"), true)
		require.ErrorIs(t, err, ErrSMTPError)
	})
}

// TestIsLocalhost checks the localhost detection
func TestIsLocalhost(t *testing.T) {
	t.Parallel()

	for _, host := range []string{"localhost", "127.0.0.1", "::1"} {
		assert.True(t, isLocalhost(host), host)
	}
	assert.False(t, isLocalhost("example.com"))
}

// pastDeadlineContext reports a passed deadline before its Err is set
type pastDeadlineContext struct {
	context.Context //nolint:containedctx // wraps a context to control Deadline
}

// Deadline returns a deadline in the past
func (pastDeadlineContext) Deadline() (time.Time, bool) {
	return time.Now().Add(-time.Second), true
}

// TestContextError checks the context error detection
func TestContextError(t *testing.T) {
	t.Parallel()

	require.NoError(t, contextError(context.Background()))

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, contextError(canceled), context.Canceled)

	require.ErrorIs(t, contextError(pastDeadlineContext{context.Background()}), context.DeadlineExceeded)

	future, cancelFuture := context.WithTimeout(context.Background(), time.Hour)
	defer cancelFuture()
	require.NoError(t, contextError(future))
}

// TestSMTPErrorWrapping checks the SMTP error wrapping
func TestSMTPErrorWrapping(t *testing.T) {
	t.Parallel()

	cause := errors.New("boom") //nolint:err113 // test-only error
	err := smtpError(context.Background(), "DATA", cause)
	require.ErrorIs(t, err, ErrSMTPError)
	require.ErrorIs(t, err, cause)
	assert.Equal(t, "error from smtp server: DATA: boom", err.Error())
}

// TestMailServiceSMTPWithoutCredentials checks that an SMTP relay without
// authentication is loaded and used by StartUp
func TestMailServiceSMTPWithoutCredentials(t *testing.T) {
	t.Parallel()

	server := startFakeSMTPServer(t, fakeSMTPConfig{})
	service := &MailService{
		FromDomain:   testDomainEmail,
		FromUsername: testUsernameEmail,
		SMTPHost:     "127.0.0.1",
		SMTPPort:     server.port(),
	}
	require.NoError(t, service.StartUp())
	assert.Equal(t, []ServiceProvider{SMTP}, service.AvailableProviders)

	email := service.NewEmail()
	email.Subject = "Relay"
	email.PlainTextContent = "Hello"
	email.Recipients = []string{testRecipientSuccess}
	require.NoError(t, service.SendEmail(context.Background(), email, SMTP))
	assert.Equal(t, []string{testRecipientSuccess}, server.session(t).rcpts)
}

// TestSMTPProviderConcurrentSends checks that concurrent sends do not share state
func TestSMTPProviderConcurrentSends(t *testing.T) {
	t.Parallel()

	server := startFakeSMTPServer(t, fakeSMTPConfig{})
	provider := newTestSMTPProvider(server, SMTPConfig{})

	const sends = 10
	var wg sync.WaitGroup
	errs := make(chan error, sends)
	for i := range sends {
		wg.Add(1)
		go func() {
			defer wg.Done()
			email := newValidEmail()
			email.Recipients = []string{"user" + strconv.Itoa(i) + "@example.com"}
			_, err := provider.Send(context.Background(), email)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	server.mu.Lock()
	defer server.mu.Unlock()
	require.Len(t, server.sessions, sends)
	for _, session := range server.sessions {
		assert.Len(t, session.rcpts, 1, "every send must only carry its own recipient")
	}
}
