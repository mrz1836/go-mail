package gomail

import (
	"testing"

	"github.com/mattbaird/gochimp"
	"github.com/stretchr/testify/require"
)

// mockMandrillInterface is a mocking interface for Mandrill
type mockMandrillInterface struct{}

// MessageSend is for mocking
func (m *mockMandrillInterface) MessageSend(message gochimp.Message, _ bool) ([]gochimp.SendResponse, error) {
	// todo: is async (bool) needed?

	// Success
	if message.To[0].Email == testRecipientSuccess {
		return []gochimp.SendResponse{}, nil
	}

	// Invalid from domain
	if message.To[0].Email == "test@badhostname.com" {
		return []gochimp.SendResponse{}, ErrValidationError
	}

	// Invalid token
	if message.To[0].Email == "test@badtoken.com" {
		return []gochimp.SendResponse{}, ErrInvalidAPIKey
	}

	// Invalid status
	if message.To[0].Email == "test@badstatus.com" {
		return []gochimp.SendResponse{{Status: "unknown"}}, nil
	}

	// Default is success
	return []gochimp.SendResponse{}, nil
}

// newMockMandrillClient will create a new mock client for Mandrill
func newMockMandrillClient() mandrillInterface {
	return &mockMandrillInterface{}
}

// TestSendViaMandrill will test the sendViaMandrill() method
func TestSendViaMandrill(t *testing.T) {
	t.Parallel()

	// Setup mock client and a ready-to-send email
	client := newMockMandrillClient()
	email := newProviderTestEmail(t)

	// Create the list of tests
	cases := []providerSendCase{
		{"successful send", testRecipientSuccess, false},
		{"invalid domain error", "test@badhostname.com", true},
		{"invalid token error", "test@badtoken.com", true},
		{"bad status error", "test@badstatus.com", true},
	}

	// Loop tests
	runProviderSendCases(t, email, cases, func() error {
		return sendViaMandrill(client, email, false)
	})

	// Test bad from address (kept inline; mutates FromAddress after the table cases)
	t.Run("invalid from address error", func(t *testing.T) {
		email.FromAddress = "invalid@"
		err := sendViaMandrill(client, email, false)
		require.Error(t, err)
	})
}

// TestSendViaMandrill_AttachmentError confirms an attachment whose reader fails
// surfaces the read error before sending
func TestSendViaMandrill_AttachmentError(t *testing.T) {
	t.Parallel()

	client := newMockMandrillClient()
	email := newProviderTestEmail(t)
	email.Recipients = []string{testRecipientSuccess}
	email.Attachments = []Attachment{
		{FileName: "bad.txt", FileType: "text/plain", FileReader: errReader{}},
	}

	err := sendViaMandrill(client, email, false)
	require.ErrorIs(t, err, ErrBadHostname)
}
