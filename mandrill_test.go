package gomail

import (
	"os"
	"testing"

	"github.com/mattbaird/gochimp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testDomain   = "example.com"
	testFromName = "No Reply"
	testUsername = "no-reply"
)

// mockMandrillInterface is a mocking interface for Mandrill
type mockMandrillInterface struct{}

// MessageSend is for mocking
func (m *mockMandrillInterface) MessageSend(message gochimp.Message, _ bool) ([]gochimp.SendResponse, error) {
	// todo: is async (bool) needed?

	// Success
	if message.To[0].Email == "test@domain.com" {
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

	// Start the service
	mail := new(MailService)

	// Set all the defaults, toggle all warnings
	mail.AutoText = true
	mail.FromDomain = testDomain
	mail.FromName = testFromName
	mail.FromUsername = testUsername
	mail.Important = true
	mail.TrackClicks = true
	mail.TrackOpens = true

	// Setup mock client
	client := newMockMandrillClient()

	// New email
	email := mail.NewEmail()
	email.HTMLContent = "<html>Test</html>"
	email.PlainTextContent = "Test"

	// Add an attachment
	f, err := os.Open("examples/test-attachment-file.txt")
	if err != nil {
		require.NoError(t, err, "failed to attach file")
	} else {
		email.AddAttachment("test-attachment-file.txt", "text/plain", f)
	}

	// Create the list of tests
	tests := []struct {
		name          string
		input         string
		expectedError bool
	}{
		{"successful send", "test@domain.com", false},
		{"invalid domain error", "test@badhostname.com", true},
		{"invalid token error", "test@badtoken.com", true},
		{"bad status error", "test@badstatus.com", true},
	}

	// Loop tests
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			email.Recipients = []string{test.input}
			email.RecipientsCc = []string{test.input}
			email.RecipientsBcc = []string{test.input}
			email.ReplyToAddress = test.input
			err := sendViaMandrill(client, email, false)
			if test.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}

	// Test bad from address
	t.Run("invalid from address error", func(t *testing.T) {
		email.FromAddress = "invalid@"
		err := sendViaMandrill(client, email, false)
		assert.Error(t, err)
	})
}
