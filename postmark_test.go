package gomail

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/mrz1836/postmark"
)

// mockPostmarkInterface is a mocking interface for Postmark
type mockPostmarkInterface struct{}

// SendEmail is for mocking
func (m *mockPostmarkInterface) SendEmail(_ context.Context, email postmark.Email) (postmark.EmailResponse, error) {
	// Success
	if email.To == "test@domain.com" {
		return *new(postmark.EmailResponse), nil
	}

	// Invalid domain name
	if email.To == "test@badhostname.com" {
		return *new(postmark.EmailResponse), fmt.Errorf("400 The 'From' address you supplied (No Reply %s) is not a Sender Signature on your account. Please add and confirm this address in order to be able to use it in the 'From' field of your messages: %w", email.To, ErrPostmarkFromError)
	}

	// Invalid token
	if email.To == "test@badtoken.com" {
		return *new(postmark.EmailResponse), fmt.Errorf("10 The Server Token you provided in the X-Postmark-Server-Token request header was invalid. Please verify that you are using a valid token: %w", ErrPostmarkTokenError)
	}

	// Invalid - bad error code
	if email.To == "test@errorcode.com" {
		resp := &postmark.EmailResponse{
			ErrorCode: http.StatusBadGateway,
		}
		return *resp, nil
	}

	// Default is success
	return *new(postmark.EmailResponse), nil
}

// newMockPostmarkClient will create a new mock client for Postmark
func newMockPostmarkClient() postmarkInterface {
	return &mockPostmarkInterface{}
}

// TestSendViaPostmark will test the sendViaPostmark() method
func TestSendViaPostmark(t *testing.T) {
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
	client := newMockPostmarkClient()

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
		{"test@badhostname.com", true},
		{"test@badtoken.com", true},
		{"test@errorcode.com", true},
	}

	// Loop tests
	for _, test := range tests {
		email.Recipients = []string{test.input}
		email.RecipientsCc = []string{test.input}
		email.RecipientsBcc = []string{test.input}
		email.ReplyToAddress = test.input
		if err = sendViaPostmark(context.Background(), client, email); err != nil && !test.expectedError {
			t.Fatalf("%s Failed: expected to NOT throw an error, inputted and [%s], error [%s]", t.Name(), test.input, err.Error())
		} else if err == nil && test.expectedError {
			t.Fatalf("%s Failed: expected to throw an error, inputted and [%s]", t.Name(), test.input)
		}
	}
}
