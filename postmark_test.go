package gomail

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/mrz1836/postmark"
	"github.com/stretchr/testify/require"
)

// capturingPostmarkInterface records the last email passed to SendEmail so tests
// can assert on the built postmark.Email payload
type capturingPostmarkInterface struct {
	lastEmail postmark.Email
}

// SendEmail captures the email and returns a successful response
func (m *capturingPostmarkInterface) SendEmail(_ context.Context, email postmark.Email) (postmark.EmailResponse, error) {
	m.lastEmail = email
	return postmark.EmailResponse{}, nil
}

// mockPostmarkInterface is a mocking interface for Postmark
type mockPostmarkInterface struct{}

// SendEmail is for mocking
func (m *mockPostmarkInterface) SendEmail(_ context.Context, email postmark.Email) (postmark.EmailResponse, error) {
	// Success
	if email.To == testRecipientSuccess {
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

	// Setup mock client and a ready-to-send email
	client := newMockPostmarkClient()
	email := newProviderTestEmail(t)

	// Create the list of tests
	cases := []providerSendCase{
		{"successful send", testRecipientSuccess, false},
		{"invalid domain name error", "test@badhostname.com", true},
		{"invalid token error", "test@badtoken.com", true},
		{"error code failure", "test@errorcode.com", true},
	}

	// Loop tests
	runProviderSendCases(t, email, cases, func() error {
		return sendViaPostmark(context.Background(), client, email)
	})
}

// TestSendViaPostmark_FromHeader confirms the From header is built as a valid
// RFC 5322 "Name <address>" value when a FromName is present
func TestSendViaPostmark_FromHeader(t *testing.T) {
	t.Parallel()

	email := newProviderTestEmail(t)
	email.Recipients = []string{testRecipientSuccess}

	t.Run("from name and address", func(t *testing.T) {
		capture := &capturingPostmarkInterface{}
		err := sendViaPostmark(context.Background(), capture, email)
		require.NoError(t, err)
		require.Equal(t, "No Reply <no-reply@example.com>", capture.lastEmail.From)
	})

	t.Run("address only when from name is empty", func(t *testing.T) {
		email.FromName = ""
		capture := &capturingPostmarkInterface{}
		err := sendViaPostmark(context.Background(), capture, email)
		require.NoError(t, err)
		require.Equal(t, "no-reply@example.com", capture.lastEmail.From)
	})
}

// TestSendViaPostmark_AttachmentError confirms an attachment whose reader fails
// surfaces the read error before sending
func TestSendViaPostmark_AttachmentError(t *testing.T) {
	t.Parallel()

	email := newProviderTestEmail(t)
	email.Recipients = []string{testRecipientSuccess}
	email.Attachments = []Attachment{
		{FileName: "bad.txt", FileType: "text/plain", FileReader: errReader{}},
	}

	capture := &capturingPostmarkInterface{}
	err := sendViaPostmark(context.Background(), capture, email)
	require.ErrorIs(t, err, ErrBadHostname)
	require.Empty(t, capture.lastEmail.From, "send should not be attempted when an attachment fails to encode")
}
