package gomail

import (
	"os"
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
)

// newProviderTestEmail builds a ready-to-send Email from a MailService configured
// with the shared provider test defaults (all warning-triggering flags on, with a
// file attachment) used by the TestSendVia* provider tests
func newProviderTestEmail(t *testing.T) *Email {
	t.Helper()

	// Start the service with all the defaults, toggling every warning path
	mail := new(MailService)
	mail.AutoText = true
	mail.FromDomain = testDomainEmail
	mail.FromName = testFromNameEmail
	mail.FromUsername = testUsernameEmail
	mail.Important = true
	mail.TrackClicks = true
	mail.TrackOpens = true

	// New email with both content types
	email := mail.NewEmail()
	email.HTMLContent = "<html>Test</html>"
	email.PlainTextContent = "Test"

	// Add an attachment
	f, err := os.Open("examples/test-attachment-file.txt")
	require.NoError(t, err)
	email.AddAttachment("test-attachment-file.txt", "text/plain", f)

	return email
}

// providerSendCase is a single table-driven case for the provider send tests
type providerSendCase struct {
	name          string
	input         string
	expectedError bool
}

// runProviderSendCases runs the shared provider send cases; for each case it sets
// the recipient fields on the shared email and invokes send, asserting on the
// expected error outcome
func runProviderSendCases(t *testing.T, email *Email, cases []providerSendCase, send func() error) {
	t.Helper()

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			email.Recipients = []string{test.input}
			email.RecipientsCc = []string{test.input}
			email.RecipientsBcc = []string{test.input}
			email.ReplyToAddress = test.input
			err := send()
			if test.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
