package gomail

import (
	"context"
	"net/http"
	"testing"

	"github.com/sendgrid/rest"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSendGridInterface is a mocking interface for SendGrid; it routes by the
// primary recipient address (mirroring the other providers' mocks)
type mockSendGridInterface struct{}

// SendWithContext is for mocking
func (m *mockSendGridInterface) SendWithContext(_ context.Context, email *mail.SGMailV3) (*rest.Response, error) {
	// Determine the primary recipient
	var to string
	if len(email.Personalizations) > 0 && len(email.Personalizations[0].To) > 0 {
		to = email.Personalizations[0].To[0].Address
	}

	switch to {
	// Success (accepted)
	case testRecipientSuccess:
		return &rest.Response{StatusCode: http.StatusAccepted}, nil

	// Server error status code
	case "test@errorcode.com":
		return &rest.Response{StatusCode: http.StatusInternalServerError}, nil

	// Invalid token status code
	case "test@badtoken.com":
		return &rest.Response{StatusCode: http.StatusUnauthorized}, nil

	// Transport error
	case "test@badhostname.com":
		return nil, ErrBadHostname
	}

	// Default is success
	return &rest.Response{StatusCode: http.StatusAccepted}, nil
}

// newMockSendGridClient will create a new mock client for SendGrid
func newMockSendGridClient() sendGridInterface {
	return &mockSendGridInterface{}
}

// capturingSendGridInterface records the last message passed to SendWithContext
// so tests can assert on the built *mail.SGMailV3 payload
type capturingSendGridInterface struct {
	lastMessage *mail.SGMailV3
}

// SendWithContext captures the message and returns a successful response
func (m *capturingSendGridInterface) SendWithContext(_ context.Context, email *mail.SGMailV3) (*rest.Response, error) {
	m.lastMessage = email
	return &rest.Response{StatusCode: http.StatusAccepted}, nil
}

// TestNewSendGridClient tests creating a new SendGrid client
func TestNewSendGridClient(t *testing.T) {
	t.Parallel()

	client := newSendGridClient("test-api-key")
	require.NotNil(t, client)
}

// TestSendViaSendGrid will test the sendViaSendGrid() method
func TestSendViaSendGrid(t *testing.T) {
	t.Parallel()

	// Setup mock client and a ready-to-send email
	client := newMockSendGridClient()
	email := newProviderTestEmail(t)

	// Create the list of tests
	cases := []providerSendCase{
		{"successful send", testRecipientSuccess, false},
		{"error code failure", "test@errorcode.com", true},
		{"invalid token error", "test@badtoken.com", true},
		{"bad hostname transport error", "test@badhostname.com", true},
	}

	// Loop tests
	runProviderSendCases(t, email, cases, func() error {
		return sendViaSendGrid(context.Background(), client, email)
	})
}

// TestSendViaSendGrid_MessageBuild confirms the *mail.SGMailV3 payload is built
// correctly: sender, subject, recipients, plain-before-html content ordering,
// reply-to, categories, importance headers, native tracking, and attachments
func TestSendViaSendGrid_MessageBuild(t *testing.T) {
	t.Parallel()

	email := newProviderTestEmail(t)
	email.Subject = "Test subject"
	email.Recipients = []string{"to@domain.com"}
	email.RecipientsCc = []string{"cc@domain.com"}
	email.RecipientsBcc = []string{"bcc@domain.com"}
	email.ReplyToAddress = "reply@domain.com"
	email.Tags = []string{"tag1", "tag2"}

	capture := &capturingSendGridInterface{}
	err := sendViaSendGrid(context.Background(), capture, email)
	require.NoError(t, err)
	require.NotNil(t, capture.lastMessage)

	msg := capture.lastMessage

	// From
	require.NotNil(t, msg.From)
	assert.Equal(t, testFromNameEmail, msg.From.Name)
	assert.Equal(t, testUsernameEmail+"@"+testDomainEmail, msg.From.Address)

	// Subject
	assert.Equal(t, "Test subject", msg.Subject)

	// Recipients (single personalization with to/cc/bcc)
	require.Len(t, msg.Personalizations, 1)
	p := msg.Personalizations[0]
	require.Len(t, p.To, 1)
	assert.Equal(t, "to@domain.com", p.To[0].Address)
	require.Len(t, p.CC, 1)
	assert.Equal(t, "cc@domain.com", p.CC[0].Address)
	require.Len(t, p.BCC, 1)
	assert.Equal(t, "bcc@domain.com", p.BCC[0].Address)

	// Content ordering: plain text must come before html
	require.Len(t, msg.Content, 2)
	assert.Equal(t, "text/plain", msg.Content[0].Type)
	assert.Equal(t, "text/html", msg.Content[1].Type)

	// Reply-to
	require.NotNil(t, msg.ReplyTo)
	assert.Equal(t, "reply@domain.com", msg.ReplyTo.Address)

	// Categories from tags
	assert.Equal(t, []string{"tag1", "tag2"}, msg.Categories)

	// Native tracking populated (TrackClicks + TrackOpens both on via defaults)
	require.NotNil(t, msg.TrackingSettings)
	require.NotNil(t, msg.TrackingSettings.ClickTracking)
	require.NotNil(t, msg.TrackingSettings.ClickTracking.Enable)
	assert.True(t, *msg.TrackingSettings.ClickTracking.Enable)
	require.NotNil(t, msg.TrackingSettings.OpenTracking)
	require.NotNil(t, msg.TrackingSettings.OpenTracking.Enable)
	assert.True(t, *msg.TrackingSettings.OpenTracking.Enable)

	// Importance headers
	assert.Equal(t, headerXPriorityValue, msg.Headers[headerXPriority])
	assert.Equal(t, headerHighValue, msg.Headers[headerXMSMailPriority])
	assert.Equal(t, headerHighValue, msg.Headers[headerImportance])

	// Attachment (base64 encoded)
	require.Len(t, msg.Attachments, 1)
	assert.Equal(t, "test-attachment-file.txt", msg.Attachments[0].Filename)
	assert.Equal(t, "text/plain", msg.Attachments[0].Type)
	assert.Equal(t, "attachment", msg.Attachments[0].Disposition)
	assert.NotEmpty(t, msg.Attachments[0].Content)
}

// TestSendViaSendGrid_AttachmentError confirms an attachment whose reader fails
// surfaces the read error before sending
func TestSendViaSendGrid_AttachmentError(t *testing.T) {
	t.Parallel()

	email := newProviderTestEmail(t)
	email.Recipients = []string{testRecipientSuccess}
	email.Attachments = []Attachment{
		{FileName: "bad.txt", FileType: "text/plain", FileReader: errReader{}},
	}

	capture := &capturingSendGridInterface{}
	err := sendViaSendGrid(context.Background(), capture, email)
	require.ErrorIs(t, err, ErrBadHostname)
	require.Nil(t, capture.lastMessage, "send should not be attempted when an attachment fails to encode")
}

// TestSendViaSendGrid_NoTracking confirms tracking settings are omitted when
// neither open nor click tracking is enabled
func TestSendViaSendGrid_NoTracking(t *testing.T) {
	t.Parallel()

	email := newProviderTestEmail(t)
	email.Recipients = []string{"to@domain.com"}
	email.TrackClicks = false
	email.TrackOpens = false

	capture := &capturingSendGridInterface{}
	err := sendViaSendGrid(context.Background(), capture, email)
	require.NoError(t, err)
	require.NotNil(t, capture.lastMessage)
	assert.Nil(t, capture.lastMessage.TrackingSettings)
}
