package gomail

import (
	"fmt"
	"os"
	"testing"

	"github.com/mattbaird/gochimp"
)

// mockMandrillInterface is a mocking interface for Mandrill
type mockMandrillInterface struct{}

// MessageSend is for mocking
func (m *mockMandrillInterface) MessageSend(message gochimp.Message, async bool) ([]gochimp.SendResponse, error) {

	// Success
	if message.To[0].Email == "test@domain.com" {
		return []gochimp.SendResponse{}, nil
	}

	// Invalid from domain
	if message.To[0].Email == "test@badhostname.com" {
		return []gochimp.SendResponse{}, fmt.Errorf(`-2: Validation error: {"message":{"from_email":"The domain portion of the email address is invalid (the portion after the @: badhostname.com)"}}`)
	}

	// Invalid token
	if message.To[0].Email == "test@badtoken.com" {
		return []gochimp.SendResponse{}, fmt.Errorf(`-1: Invalid API key`)
	}

	// Invalid status
	if message.To[0].Email == "test@badstatus.com" {
		var responses []gochimp.SendResponse
		resp := gochimp.SendResponse{
			Status: "unknown",
		}
		responses = append(responses, resp)

		return responses, nil
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
	mail.FromDomain = "example.com"
	mail.FromName = "No Reply"
	mail.FromUsername = "no-reply"
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
		t.Errorf("failed to attach file: %s", err.Error())
	} else {
		email.AddAttachment("test-attachment-file.txt", "text/plain", f)
	}

	// Create the list of tests
	var tests = []struct {
		input         string
		expectedError bool
	}{
		{"test@domain.com", false},
		{"test@badhostname.com", true},
		{"test@badtoken.com", true},
		{"test@badstatus.com", true},
	}

	// Loop tests
	for _, test := range tests {
		email.Recipients = []string{test.input}
		email.RecipientsCc = []string{test.input}
		email.RecipientsBcc = []string{test.input}
		email.ReplyToAddress = test.input
		if err = sendViaMandrill(client, email, false); err != nil && !test.expectedError {
			t.Errorf("%s Failed: expected to NOT throw an error, inputted and [%s], error [%s]", t.Name(), test.input, err.Error())
		} else if err == nil && test.expectedError {
			t.Errorf("%s Failed: expected to throw an error, inputted and [%s]", t.Name(), test.input)
		}
	}

	// Test bad from address
	email.FromAddress = "invalid@"
	if err = sendViaMandrill(client, email, false); err == nil {
		t.Errorf("%s Failed: expected to throw an error, inputted and [%s]", t.Name(), email.FromAddress)
	}
}
