package gomail

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// Shared test constants for the from address used across the provider tests
const (
	testDomainEmail   = "example.com"
	testFromNameEmail = "No Reply"
	testUsernameEmail = "no-reply"

	// testRecipientSuccess is the recipient every provider mock routes to a
	// successful send response
	testRecipientSuccess = "test@domain.com"

	// testMessageID is the message id returned by the provider fakes
	testMessageID = "message-id-123"

	// Shared test addresses and file names
	testAddressA   = "a@example.com"
	testBccAddress = "bcc@example.com"
	testCcAddress  = "cc@example.com"
	testFileName   = "a.txt"
)

// newProviderTestEmail builds a ready-to-send Email from a MailService configured
// with the shared provider test defaults (all warning-triggering flags on, with a
// file attachment) used by the provider tests
func newProviderTestEmail(t *testing.T) *Email {
	t.Helper()

	// Start the service with all the defaults, toggling every warning path
	service := new(MailService)
	service.AutoText = true
	service.FromDomain = testDomainEmail
	service.FromName = testFromNameEmail
	service.FromUsername = testUsernameEmail
	service.Important = true
	service.TrackClicks = true
	service.TrackOpens = true

	// New email with both content types
	email := service.NewEmail()
	email.Subject = "Test Subject"
	email.HTMLContent = "<html>Test</html>"
	email.PlainTextContent = "Test"
	email.Recipients = []string{testRecipientSuccess}

	// Add an attachment
	f, err := os.Open("examples/test-attachment-file.txt")
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	email.AddAttachment("test-attachment-file.txt", "text/plain", f)

	return email
}

// newTestService returns a started MailService with the given fake providers registered
func newTestService(t *testing.T, providers map[ServiceProvider]Provider) *MailService {
	t.Helper()

	service := &MailService{FromDomain: testDomainEmail, FromUsername: testUsernameEmail}
	for _, id := range []ServiceProvider{AwsSes, Mandrill, Postmark, SMTP, SendGrid, Resend, Mailgun, ServiceProvider(100)} {
		if provider, ok := providers[id]; ok {
			require.NoError(t, service.RegisterProvider(id, provider))
		}
	}
	require.NoError(t, service.StartUp())
	return service
}

// newValidEmail returns a minimal valid email
func newValidEmail() *Email {
	return &Email{
		FromAddress:      testUsernameEmail + "@" + testDomainEmail,
		PlainTextContent: "Hello",
		Recipients:       []string{testRecipientSuccess},
		Subject:          "Hello",
	}
}

// fakeProvider is a configurable Provider used to test MailService
type fakeProvider struct {
	err       error
	result    *SendResult
	supported []Feature
	sendFn    func(ctx context.Context, email *Email) (*SendResult, error)
	mu        sync.Mutex
	calls     int
	deadline  bool
}

// Send records the call and returns the configured outcome
func (f *fakeProvider) Send(ctx context.Context, email *Email) (*SendResult, error) {
	f.mu.Lock()
	f.calls++
	_, f.deadline = ctx.Deadline()
	f.mu.Unlock()

	if f.sendFn != nil {
		return f.sendFn(ctx, email)
	}
	return f.result, f.err
}

// callCount returns how many times Send was called
func (f *fakeProvider) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// hadDeadline reports whether the last Send context had a deadline
func (f *fakeProvider) hadDeadline() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.deadline
}

// featureFakeProvider is a fakeProvider that reports the features it supports
type featureFakeProvider struct {
	fakeProvider
}

// SupportsFeature reports whether the feature is in the supported list
func (f *featureFakeProvider) SupportsFeature(feature Feature) bool {
	return supportsFeature(f.supported, feature)
}

// errReader is an io.Reader that always fails
type errReader struct{}

// Read always returns an error
func (errReader) Read(_ []byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}

// parsedMIME is a MIME message parsed back for assertions
type parsedMIME struct {
	header mail.Header
	parts  []parsedPart // leaf parts, depth first
	root   string       // media type of the top-level body
}

// parsedPart is a decoded leaf MIME part
type parsedPart struct {
	header      map[string][]string
	mediaType   string
	disposition string
	filename    string
	content     string
}

// parseMIME parses a raw message, decoding every leaf part
func parseMIME(t *testing.T, raw []byte) *parsedMIME {
	t.Helper()

	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	require.NoError(t, err)

	mediaType, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	require.NoError(t, err)

	parsed := &parsedMIME{header: msg.Header, root: mediaType}
	body, err := io.ReadAll(msg.Body)
	require.NoError(t, err)
	collectParts(t, parsed, msg.Header.Get("Content-Type"), msg.Header.Get("Content-Transfer-Encoding"), params, body)
	return parsed
}

// collectParts recursively decodes a MIME entity into its leaf parts
func collectParts(t *testing.T, parsed *parsedMIME, contentType, encoding string, params map[string]string, body []byte) {
	t.Helper()

	mediaType, _, err := mime.ParseMediaType(contentType)
	require.NoError(t, err)

	if strings.HasPrefix(mediaType, "multipart/") {
		reader := multipart.NewReader(bytes.NewReader(body), params["boundary"])
		for {
			part, partErr := reader.NextRawPart()
			if partErr == io.EOF {
				return
			}
			require.NoError(t, partErr)

			partBody, readErr := io.ReadAll(part)
			require.NoError(t, readErr)

			_, partParams, parseErr := mime.ParseMediaType(part.Header.Get("Content-Type"))
			require.NoError(t, parseErr)
			collectParts(t, parsed, part.Header.Get("Content-Type"), part.Header.Get("Content-Transfer-Encoding"), partParams, partBody)
			parsed.parts[len(parsed.parts)-1].header = mergeHeader(parsed.parts[len(parsed.parts)-1].header, part.Header)
		}
	}

	parsed.parts = append(parsed.parts, parsedPart{
		content:   decodeBody(t, encoding, body),
		mediaType: mediaType,
	})
}

// mergeHeader keeps the innermost part header
func mergeHeader(existing, header map[string][]string) map[string][]string {
	if existing != nil {
		return existing
	}
	return header
}

// decodeBody decodes a part body with its transfer encoding
func decodeBody(t *testing.T, encoding string, body []byte) string {
	t.Helper()

	switch strings.ToLower(encoding) {
	case "base64":
		decoded, err := io.ReadAll(base64Decoder(body))
		require.NoError(t, err)
		return string(decoded)
	case "quoted-printable":
		decoded, err := io.ReadAll(quotedPrintableDecoder(body))
		require.NoError(t, err)
		return string(decoded)
	default:
		return string(body)
	}
}

// part returns the first leaf part with the media type
func (p *parsedMIME) part(t *testing.T, mediaType string) parsedPart {
	t.Helper()

	for _, part := range p.parts {
		if part.mediaType == mediaType {
			part.disposition, part.filename = partDisposition(part.header)
			return part
		}
	}
	require.Failf(t, "part not found", "no %s part in %v", mediaType, p.mediaTypes())
	return parsedPart{}
}

// mediaTypes lists the leaf part media types
func (p *parsedMIME) mediaTypes() []string {
	types := make([]string, 0, len(p.parts))
	for _, part := range p.parts {
		types = append(types, part.mediaType)
	}
	return types
}

// partDisposition returns the part disposition and filename
func partDisposition(header map[string][]string) (string, string) {
	values := header["Content-Disposition"]
	if len(values) == 0 {
		return "", ""
	}
	disposition, params, err := mime.ParseMediaType(values[0])
	if err != nil {
		return "", ""
	}
	return disposition, params["filename"]
}

// base64Decoder returns a reader decoding a base64 body (line breaks are ignored)
func base64Decoder(body []byte) io.Reader {
	return base64.NewDecoder(base64.StdEncoding, bytes.NewReader(body))
}

// quotedPrintableDecoder returns a reader decoding a quoted-printable body
func quotedPrintableDecoder(body []byte) io.Reader {
	return quotedprintable.NewReader(bytes.NewReader(body))
}
