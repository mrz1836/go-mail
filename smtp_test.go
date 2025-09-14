package gomail

import (
	"bytes"
	"io"
	"net/smtp"
	"os"
	"regexp"
	"testing"

	"github.com/domodwyer/mailyak"
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
		if m.toAddrs[0] == "test@domain.com" {
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

	client := newSMTPClient("", auth)

	err := client.Send()
	if err == nil {
		t.Fatalf("error should have occurred, host was empty")
	}

	client = newSMTPClient("example.com", auth)
	err = client.Send()
	if err == nil {
		t.Fatalf("error should have occurred, host example.com")
	}
}

// newMockSMTPClient will create a new mock client for SMTP
func newMockSMTPClient() smtpInterface {
	return &mockSMTPInterface{
		trimRegex: regexp.MustCompile("\r?\n"),
	}
}

// TestSendViaSMTP will test the sendViaSMTP() method
func TestSendViaSMTP(t *testing.T) {
	t.Parallel()

	// Start the service
	mail := new(MailService)

	// Set all the defaults, toggle all warnings
	mail.AutoText = true
	mail.FromDomain = "example.com"
	mail.FromName = "No Reply"
	mail.FromUsername = "no-reply"
	mail.Important = true
	mail.TrackClicks = true
	mail.TrackOpens = true

	// Setup mock client
	client := newMockSMTPClient()

	// New email
	email := mail.NewEmail()
	email.HTMLContent = "<html>Test</html>"
	email.PlainTextContent = "Test"

	// Add an attachment
	f, err := os.Open("examples/test-attachment-file.txt")
	if err != nil {
		t.Fatalf("failed to attach file: %s", err.Error())
	} else {
		email.AddAttachment("test-attachment-file.txt", "text/plain", f)
	}

	// Create the list of tests
	tests := []struct {
		input         string
		expectedError bool
	}{
		{"test@domain.com", false},
		{"test@badusername.com", true},
		{"test@badhostname.com", true},
	}

	// Loop tests
	for _, test := range tests {
		email.Recipients = []string{test.input}
		email.RecipientsCc = []string{test.input}
		email.RecipientsBcc = []string{test.input}
		email.ReplyToAddress = test.input
		if err = sendViaSMTP(client, email); err != nil && !test.expectedError {
			t.Fatalf("%s Failed: expected to NOT throw an error, inputted and [%s], error [%s]", t.Name(), test.input, err.Error())
		} else if err == nil && test.expectedError {
			t.Fatalf("%s Failed: expected to throw an error, inputted and [%s]", t.Name(), test.input)
		}
	}
}
