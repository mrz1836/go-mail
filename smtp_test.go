package gomail

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/smtp"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/domodwyer/mailyak"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSMTPInterface is a mocking interface for SMTP
type mockSMTPInterface struct {
	html      mailyak.BodyPart
	plain     mailyak.BodyPart
	toAddrs   []string
	trimRegex *regexp.Regexp
}

// Send will mock sending the email
func (m *mockSMTPInterface) Send() error {
	if len(m.toAddrs) > 0 {

		// Valid email
		if m.toAddrs[0] == testRecipientSuccess {
			return nil
		}

		// Bad username - Auth
		if m.toAddrs[0] == "test@badusername.com" {
			return ErrSMTPAuth
		}

		// Bad hostname
		if m.toAddrs[0] == "test@badhostname.com" {
			return ErrDNSLookup
		}

	}

	// Return success anyway
	return nil
}

// MimeBuf will mock the mime type
func (m *mockSMTPInterface) MimeBuf() (*bytes.Buffer, error) {
	return &bytes.Buffer{}, nil
}

// String is a mock method
func (m *mockSMTPInterface) String() string {
	return ""
}

// HTML is a mock method
func (m *mockSMTPInterface) HTML() *mailyak.BodyPart {
	return &m.html
}

// Plain is a mock method
func (m *mockSMTPInterface) Plain() *mailyak.BodyPart {
	return &m.plain
}

// To is a mock method
func (m *mockSMTPInterface) To(addrs ...string) {
	m.toAddrs = []string{}

	for _, addr := range addrs {
		trimmed := m.trimRegex.ReplaceAllString(addr, "")
		if trimmed == "" {
			continue
		}

		m.toAddrs = append(m.toAddrs, trimmed)
	}
}

// Bcc is a mock method
func (m *mockSMTPInterface) Bcc(_ ...string) {}

// WriteBccHeader is a mock method
func (m *mockSMTPInterface) WriteBccHeader(_ bool) {}

// Cc is a mock method
func (m *mockSMTPInterface) Cc(_ ...string) {}

// From is a mock method
func (m *mockSMTPInterface) From(_ string) {}

// FromName is a mock method
func (m *mockSMTPInterface) FromName(_ string) {}

// ReplyTo is a mock method
func (m *mockSMTPInterface) ReplyTo(_ string) {}

// Subject is a mock method
func (m *mockSMTPInterface) Subject(_ string) {}

// AddHeader is a mock method
func (m *mockSMTPInterface) AddHeader(_, _ string) {}

// Attach is a mock method
func (m *mockSMTPInterface) Attach(_ string, _ io.Reader) {}

// AttachWithMimeType is a mock method
func (m *mockSMTPInterface) AttachWithMimeType(_ string, _ io.Reader, _ string) {}

// AttachInline is a mock method
func (m *mockSMTPInterface) AttachInline(_ string, _ io.Reader) {}

// AttachInlineWithMimeType is a mock method
func (m *mockSMTPInterface) AttachInlineWithMimeType(_ string, _ io.Reader, _ string) {}

// ClearAttachments is a mock method
func (m *mockSMTPInterface) ClearAttachments() {}

// TestNewSMTPClient is a basic test for creating a client
func TestNewSMTPClient(t *testing.T) {
	auth := smtp.PlainAuth("", "user", "password", "host")

	t.Run("empty host error", func(t *testing.T) {
		client := newSMTPClient("", auth)
		err := client.Send()
		assert.Error(t, err)
	})

	t.Run("example.com host error", func(t *testing.T) {
		client := newSMTPClient("example.com", auth)
		err := client.Send()
		assert.Error(t, err)
	})
}

// newMockSMTPClient will create a new mock client for SMTP
func newMockSMTPClient() smtpInterface {
	return &mockSMTPInterface{
		trimRegex: regexp.MustCompile("\r?\n"),
	}
}

// newMockSMTPClientFactory is an smtpClientFactory that returns a new mock client
func newMockSMTPClientFactory(_ string, _ smtp.Auth) smtpInterface {
	return newMockSMTPClient()
}

// recordingSMTPClient is a real mailyak client whose Send renders the message
// instead of delivering it, so tests can inspect exactly what would be sent
type recordingSMTPClient struct {
	*mailyak.MailYak

	mime  string // the rendered MIME message
	state string // mailyak's redacted state, including the Bcc envelope recipients
}

// Send records the rendered message instead of delivering it
func (c *recordingSMTPClient) Send() error {
	buf, err := c.MimeBuf()
	if err != nil {
		return err
	}
	c.mime = buf.String()
	c.state = c.String()
	return nil
}

// recordingSMTPFactory is an smtpClientFactory that records every client it builds
type recordingSMTPFactory struct {
	mu      sync.Mutex
	hosts   []string
	auths   []smtp.Auth
	clients []*recordingSMTPClient
}

// build creates a new recording client and records it with its connection details
func (f *recordingSMTPFactory) build(host string, auth smtp.Auth) smtpInterface {
	client := &recordingSMTPClient{MailYak: mailyak.New(host, auth)}

	f.mu.Lock()
	defer f.mu.Unlock()
	f.hosts = append(f.hosts, host)
	f.auths = append(f.auths, auth)
	f.clients = append(f.clients, client)

	return client
}

// newSMTPTestService starts a MailService with only the SMTP provider loaded and
// installs a recording client factory
func newSMTPTestService(t *testing.T) (*MailService, *recordingSMTPFactory) {
	t.Helper()

	mail := &MailService{
		FromDomain:   testDomainEmail,
		FromName:     testFromNameEmail,
		FromUsername: testUsernameEmail,
		SMTPHost:     "localhost",
		SMTPPassword: "fake-password",
		SMTPPort:     1025,
		SMTPUsername: "fake-username",
	}
	require.NoError(t, mail.StartUp())

	factory := &recordingSMTPFactory{}
	mail.smtpClientFactory = factory.build

	return mail, factory
}

// TestNewSMTPMessageClient will test the newSMTPMessageClient() method
func TestNewSMTPMessageClient(t *testing.T) {
	t.Run("without a factory builds a new mailyak client per call", func(t *testing.T) {
		mail := &MailService{smtpAddr: "localhost:1025"}

		first, ok := mail.newSMTPMessageClient().(*mailyak.MailYak)
		require.True(t, ok)
		second, ok := mail.newSMTPMessageClient().(*mailyak.MailYak)
		require.True(t, ok)

		assert.NotSame(t, first, second)
	})

	t.Run("passes the address and auth from StartUp to the factory", func(t *testing.T) {
		mail, factory := newSMTPTestService(t)

		mail.newSMTPMessageClient()

		require.Len(t, factory.hosts, 1)
		assert.Equal(t, "localhost:1025", factory.hosts[0])
		assert.NotNil(t, factory.auths[0])
	})
}

// TestSendEmailSMTPUsesFreshClientPerSend verifies that nothing from one SMTP
// email (recipients, reply-to, body parts, attachments, headers) leaks into the next
func TestSendEmailSMTPUsesFreshClientPerSend(t *testing.T) {
	mail, factory := newSMTPTestService(t)

	// A fully populated first email
	first := mail.NewEmail()
	first.Subject = "first"
	first.PlainTextContent = "first message"
	first.HTMLContent = "<p>first message</p>"
	first.Recipients = []string{"alice@example.com"}
	first.RecipientsCc = []string{"carol@example.com"}
	first.RecipientsBcc = []string{"dave@example.com"}
	first.ReplyToAddress = "support@example.com"
	first.Important = true
	first.AddAttachment("first.txt", "text/plain", strings.NewReader("for alice"))
	require.NoError(t, mail.SendEmail(context.Background(), first, SMTP))

	// A minimal second email
	second := mail.NewEmail()
	second.Subject = "second"
	second.PlainTextContent = "second message"
	second.Recipients = []string{"bob@example.com"}
	require.NoError(t, mail.SendEmail(context.Background(), second, SMTP))

	// Each send got its own client
	require.Len(t, factory.clients, 2)
	assert.NotSame(t, factory.clients[0], factory.clients[1])

	// The first email carried everything it was given
	firstMime := factory.clients[0].mime
	assert.Contains(t, firstMime, "carol@example.com")
	assert.Contains(t, firstMime, "support@example.com")
	assert.Contains(t, firstMime, "text/html")
	assert.Contains(t, firstMime, "first.txt")
	assert.Contains(t, firstMime, headerXPriority)
	assert.Contains(t, factory.clients[0].state, "dave@example.com")

	// The second email carries none of it
	secondMime := factory.clients[1].mime
	assert.Contains(t, secondMime, "bob@example.com")
	assert.Contains(t, secondMime, "second message")
	for _, leaked := range []string{
		"alice@example.com",
		"carol@example.com",
		"dave@example.com",
		"support@example.com",
		"text/html",
		"first.txt",
		headerXPriority,
	} {
		assert.NotContains(t, secondMime, leaked)
	}
	assert.Contains(t, factory.clients[1].state, "bccAddrs: []")
}

// TestSendEmailSMTPConcurrentSends verifies that concurrent SMTP sends through one
// MailService each use their own client (run with -race to detect shared state)
func TestSendEmailSMTPConcurrentSends(t *testing.T) {
	mail, factory := newSMTPTestService(t)

	const sends = 8
	var wg sync.WaitGroup
	errs := make(chan error, sends)
	for i := range sends {
		wg.Add(1)
		go func() {
			defer wg.Done()
			email := mail.NewEmail()
			email.Subject = fmt.Sprintf("concurrent %d", i)
			email.PlainTextContent = "hello"
			email.Recipients = []string{fmt.Sprintf("user%d@example.com", i)}
			errs <- mail.SendEmail(context.Background(), email, SMTP)
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}

	// One client per send, and each rendered only its own recipient
	require.Len(t, factory.clients, sends)
	for _, client := range factory.clients {
		matches := regexp.MustCompile(`user\d+@example\.com`).FindAllString(client.mime, -1)
		require.NotEmpty(t, matches)
		for _, match := range matches {
			assert.Equal(t, matches[0], match)
		}
	}
}

// TestSendViaSMTP will test the sendViaSMTP() method
func TestSendViaSMTP(t *testing.T) {
	t.Parallel()

	// Setup mock client and a ready-to-send email
	client := newMockSMTPClient()
	email := newProviderTestEmail(t)

	// Create the list of tests
	cases := []providerSendCase{
		{"successful send", testRecipientSuccess, false},
		{"bad username auth error", "test@badusername.com", true},
		{"bad hostname DNS error", "test@badhostname.com", true},
	}

	// Loop tests
	runProviderSendCases(t, email, cases, func() error {
		return sendViaSMTP(client, email)
	})
}
