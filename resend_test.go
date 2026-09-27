package gomail

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/resend/resend-go/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Recipients the Resend routing mock maps to specific responses
const (
	testResendErrorCode    = "test@errorcode.com"
	testResendBadToken     = "test@badtoken.com"
	testResendRateLimit    = "test@ratelimit.com"
	testResendBadHostname  = "test@badhostname.com"
	testResendEmptyID      = "test@emptyid.com"
	testResendNilResponse  = "test@nilresponse.com"
	testResendAPIKey       = "re_test"
	testResendAttachment   = "examples/test-attachment-file.txt"
	testResendAttachmentFn = "test-attachment-file.txt"
)

// Test-only Resend API errors, shaped like the errors the SDK returns
var (
	errResendServer   = errors.New("[ERROR]: internal server error")
	errResendBadToken = errors.New("[ERROR]: API key is invalid")
)

// mockResendInterface is a mocking interface for Resend; it routes by the
// primary recipient address (mirroring the other providers' mocks)
type mockResendInterface struct{}

// SendWithContext is for mocking
func (m *mockResendInterface) SendWithContext(_ context.Context, params *resend.SendEmailRequest) (*resend.SendEmailResponse, error) {
	// Determine the primary recipient
	var to string
	if len(params.To) > 0 {
		to = params.To[0]
	}

	switch to {
	// Success
	case testRecipientSuccess:
		return &resend.SendEmailResponse{Id: "re_123"}, nil

	// Server error
	case testResendErrorCode:
		return nil, errResendServer

	// Invalid api key
	case testResendBadToken:
		return nil, errResendBadToken

	// Rate limited
	case testResendRateLimit:
		return nil, &resend.RateLimitError{Message: "Too many requests", Limit: "2", Remaining: "0", Reset: "1", RetryAfter: "1"}

	// Transport error
	case testResendBadHostname:
		return nil, ErrBadHostname

	// Successful status but no id returned
	case testResendEmptyID:
		return &resend.SendEmailResponse{Id: ""}, nil

	// No response and no error
	case testResendNilResponse:
		return nil, nil //nolint:nilnil // intentionally exercising the nil-response guard
	}

	// Default is success
	return &resend.SendEmailResponse{Id: "re_123"}, nil
}

// newMockResendClient will create a new mock client for Resend
func newMockResendClient() resendInterface {
	return &mockResendInterface{}
}

// capturingResendInterface records the last request passed to SendWithContext
// so tests can assert on the built *resend.SendEmailRequest payload
type capturingResendInterface struct {
	lastRequest *resend.SendEmailRequest
}

// SendWithContext captures the request and returns a successful response
func (m *capturingResendInterface) SendWithContext(_ context.Context, params *resend.SendEmailRequest) (*resend.SendEmailResponse, error) {
	m.lastRequest = params
	return &resend.SendEmailResponse{Id: "re_123"}, nil
}

// newResendTestServerClient starts an httptest server running handler and
// returns the real Resend SDK emails service pointed at it
func newResendTestServerClient(t *testing.T, handler http.HandlerFunc) resendInterface {
	t.Helper()

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	client := resend.NewCustomClient(srv.Client(), testResendAPIKey)
	baseURL, err := url.Parse(srv.URL + "/")
	require.NoError(t, err)
	client.BaseURL = baseURL

	return client.Emails
}

// writeResendJSON writes a JSON response with the given status code
func writeResendJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

// TestNewResendClient tests creating a new Resend client
func TestNewResendClient(t *testing.T) {
	t.Parallel()

	client := newResendClient("test-api-key")
	require.NotNil(t, client)
}

// TestSendViaResend will test the sendViaResend() method
func TestSendViaResend(t *testing.T) {
	t.Parallel()

	// Setup mock client and a ready-to-send email
	client := newMockResendClient()
	email := newProviderTestEmail(t)

	// Create the list of tests
	cases := []providerSendCase{
		{"successful send", testRecipientSuccess, false},
		{"error code failure", testResendErrorCode, true},
		{"invalid token error", testResendBadToken, true},
		{"rate limit error", testResendRateLimit, true},
		{"bad hostname transport error", testResendBadHostname, true},
		{"empty message id", testResendEmptyID, true},
		{"nil response", testResendNilResponse, true},
	}

	// Loop tests
	runProviderSendCases(t, email, cases, func() error {
		return sendViaResend(context.Background(), client, email)
	})
}

// TestSendViaResend_ErrorWrapping confirms every failure wraps ErrResendError
// while preserving the underlying SDK error for errors.Is checks
func TestSendViaResend_ErrorWrapping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		recipient  string
		underlying error
	}{
		{"server error", testResendErrorCode, errResendServer},
		{"invalid token", testResendBadToken, errResendBadToken},
		{"rate limit", testResendRateLimit, resend.ErrRateLimit},
		{"transport error", testResendBadHostname, ErrBadHostname},
		{"empty message id", testResendEmptyID, nil},
		{"nil response", testResendNilResponse, nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			email := newProviderTestEmail(t)
			email.Recipients = []string{test.recipient}

			err := sendViaResend(context.Background(), newMockResendClient(), email)
			require.ErrorIs(t, err, ErrResendError)
			if test.underlying != nil {
				require.ErrorIs(t, err, test.underlying)
			}
		})
	}
}

// TestSendViaResend_MessageBuild confirms the *resend.SendEmailRequest payload is
// built correctly: sender, subject, recipients, content, reply-to, tags,
// importance headers, and attachments (raw bytes)
func TestSendViaResend_MessageBuild(t *testing.T) {
	t.Parallel()

	email := newProviderTestEmail(t)
	email.Subject = "Test subject"
	email.Recipients = []string{"to@domain.com"}
	email.RecipientsCc = []string{"cc@domain.com"}
	email.RecipientsBcc = []string{"bcc@domain.com"}
	email.ReplyToAddress = "reply@domain.com"
	email.Tags = []string{"tag1", "tag2"}

	capture := &capturingResendInterface{}
	err := sendViaResend(context.Background(), capture, email)
	require.NoError(t, err)
	require.NotNil(t, capture.lastRequest)

	req := capture.lastRequest

	// From (RFC 5322 "Name <address>")
	assert.Equal(t, testFromNameEmail+" <"+testUsernameEmail+"@"+testDomainEmail+">", req.From)

	// Subject and content
	assert.Equal(t, "Test subject", req.Subject)
	assert.Equal(t, "<html>Test</html>", req.Html)
	assert.Equal(t, "Test", req.Text)

	// Recipients
	assert.Equal(t, []string{"to@domain.com"}, req.To)
	assert.Equal(t, []string{"cc@domain.com"}, req.Cc)
	assert.Equal(t, []string{"bcc@domain.com"}, req.Bcc)

	// Reply-to
	assert.Equal(t, "reply@domain.com", req.ReplyTo)

	// Tags
	assert.Equal(t, []resend.Tag{{Name: "tag1", Value: resendTagValue}, {Name: "tag2", Value: resendTagValue}}, req.Tags)

	// Importance headers
	assert.Equal(t, map[string]string{
		headerXPriority:       headerXPriorityValue,
		headerXMSMailPriority: headerHighValue,
		headerImportance:      headerHighValue,
	}, req.Headers)

	// Attachment (raw bytes, not base64)
	expected, err := os.ReadFile(testResendAttachment)
	require.NoError(t, err)
	require.Len(t, req.Attachments, 1)
	assert.Equal(t, testResendAttachmentFn, req.Attachments[0].Filename)
	assert.Equal(t, "text/plain", req.Attachments[0].ContentType)
	assert.Equal(t, expected, req.Attachments[0].Content)
}

// TestSendViaResend_NoFromName confirms the bare address is used when no from name is set
func TestSendViaResend_NoFromName(t *testing.T) {
	t.Parallel()

	email := newProviderTestEmail(t)
	email.Recipients = []string{testRecipientSuccess}
	email.FromName = ""

	capture := &capturingResendInterface{}
	err := sendViaResend(context.Background(), capture, email)
	require.NoError(t, err)
	require.NotNil(t, capture.lastRequest)
	assert.Equal(t, testUsernameEmail+"@"+testDomainEmail, capture.lastRequest.From)
}

// TestSendViaResend_NotImportant confirms no headers or tags are set when the
// email is not important and has no tags
func TestSendViaResend_NotImportant(t *testing.T) {
	t.Parallel()

	email := newProviderTestEmail(t)
	email.Recipients = []string{testRecipientSuccess}
	email.Important = false
	email.TrackClicks = false
	email.TrackOpens = false

	capture := &capturingResendInterface{}
	err := sendViaResend(context.Background(), capture, email)
	require.NoError(t, err)
	require.NotNil(t, capture.lastRequest)
	assert.Nil(t, capture.lastRequest.Headers)
	assert.Nil(t, capture.lastRequest.Tags)
}

// TestSendViaResend_AttachmentError confirms an attachment whose reader fails
// surfaces the read error before sending
func TestSendViaResend_AttachmentError(t *testing.T) {
	t.Parallel()

	email := newProviderTestEmail(t)
	email.Recipients = []string{testRecipientSuccess}
	email.Attachments = []Attachment{
		{FileName: "bad.txt", FileType: "text/plain", FileReader: errReader{}},
	}

	capture := &capturingResendInterface{}
	err := sendViaResend(context.Background(), capture, email)
	require.ErrorIs(t, err, ErrBadHostname)
	require.Nil(t, capture.lastRequest, "send should not be attempted when an attachment fails to read")
}

// TestResendTags tests the resendTags() helper
func TestResendTags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    []string
		expected []resend.Tag
	}{
		{"nil tags", nil, nil},
		{"empty tags", []string{}, nil},
		{"valid tags", []string{"admin_alert", "Signup-2"}, []resend.Tag{
			{Name: "admin_alert", Value: resendTagValue},
			{Name: "Signup-2", Value: resendTagValue},
		}},
		{"invalid characters replaced", []string{"admin alert!"}, []resend.Tag{
			{Name: "admin_alert_", Value: resendTagValue},
		}},
		{"multi-byte runes replaced", []string{"café"}, []resend.Tag{
			{Name: "caf_", Value: resendTagValue},
		}},
		{"truncated to max length", []string{strings.Repeat("a", resendTagMaxLength+10)}, []resend.Tag{
			{Name: strings.Repeat("a", resendTagMaxLength), Value: resendTagValue},
		}},
		{"empty tag dropped", []string{"", "tag"}, []resend.Tag{
			{Name: "tag", Value: resendTagValue},
		}},
		{"duplicates dropped", []string{"tag", "tag", "ta g", "ta_g"}, []resend.Tag{
			{Name: "tag", Value: resendTagValue},
			{Name: "ta_g", Value: resendTagValue},
		}},
		{"only empty tags", []string{""}, []resend.Tag{}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.expected, resendTags(test.input))
		})
	}
}

// resendWireRequest is the JSON body the Resend SDK sends to POST /emails
type resendWireRequest struct {
	From        string            `json:"from"`
	To          []string          `json:"to"`
	Cc          []string          `json:"cc"`
	Bcc         []string          `json:"bcc"`
	ReplyTo     string            `json:"reply_to"`
	Subject     string            `json:"subject"`
	HTML        string            `json:"html"`
	Text        string            `json:"text"`
	Tags        []resend.Tag      `json:"tags"`
	Headers     map[string]string `json:"headers"`
	Attachments []struct {
		Filename    string `json:"filename"`
		ContentType string `json:"content_type"`
		Content     []int  `json:"content"`
	} `json:"attachments"`
}

// TestSendViaResend_RealSDKWire drives the real Resend SDK against a local
// httptest server (no API key required) and asserts on the HTTP request it sends
func TestSendViaResend_RealSDKWire(t *testing.T) {
	t.Parallel()

	var (
		gotMethod, gotPath, gotAuth, gotContentType string
		gotBody                                     resendWireRequest
		decodeErr                                   error
	)

	client := newResendTestServerClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")
		decodeErr = json.NewDecoder(r.Body).Decode(&gotBody)
		writeResendJSON(w, http.StatusOK, `{"id":"re_abc"}`)
	})

	email := newProviderTestEmail(t)
	email.Subject = "Wire subject"
	email.Recipients = []string{"to@domain.com"}
	email.RecipientsCc = []string{"cc@domain.com"}
	email.RecipientsBcc = []string{"bcc@domain.com"}
	email.ReplyToAddress = "reply@domain.com"
	email.Tags = []string{"admin alert"}

	err := sendViaResend(context.Background(), client, email)
	require.NoError(t, err)
	require.NoError(t, decodeErr)

	// Request line and headers
	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Equal(t, "/emails", gotPath)
	assert.Equal(t, "Bearer "+testResendAPIKey, gotAuth)
	assert.Equal(t, "application/json", gotContentType)

	// JSON body
	assert.Equal(t, testFromNameEmail+" <"+testUsernameEmail+"@"+testDomainEmail+">", gotBody.From)
	assert.Equal(t, []string{"to@domain.com"}, gotBody.To)
	assert.Equal(t, []string{"cc@domain.com"}, gotBody.Cc)
	assert.Equal(t, []string{"bcc@domain.com"}, gotBody.Bcc)
	assert.Equal(t, "reply@domain.com", gotBody.ReplyTo)
	assert.Equal(t, "Wire subject", gotBody.Subject)
	assert.Equal(t, "<html>Test</html>", gotBody.HTML)
	assert.Equal(t, "Test", gotBody.Text)
	assert.Equal(t, []resend.Tag{{Name: "admin_alert", Value: resendTagValue}}, gotBody.Tags)
	assert.Equal(t, headerXPriorityValue, gotBody.Headers[headerXPriority])
	assert.Equal(t, headerHighValue, gotBody.Headers[headerXMSMailPriority])
	assert.Equal(t, headerHighValue, gotBody.Headers[headerImportance])

	// Attachment content is sent as an array of byte values
	expected, err := os.ReadFile(testResendAttachment)
	require.NoError(t, err)
	require.Len(t, gotBody.Attachments, 1)
	assert.Equal(t, testResendAttachmentFn, gotBody.Attachments[0].Filename)
	assert.Equal(t, "text/plain", gotBody.Attachments[0].ContentType)
	expectedContent := make([]int, len(expected))
	for i, b := range expected {
		expectedContent[i] = int(b)
	}
	assert.Equal(t, expectedContent, gotBody.Attachments[0].Content)
}

// TestSendViaResend_RealSDKErrors drives the real Resend SDK against a local
// httptest server returning API errors, and confirms each is wrapped in ErrResendError
func TestSendViaResend_RealSDKErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		handler     http.HandlerFunc
		contains    string
		isRateLimit bool
	}{
		{
			name: "422 validation error",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				writeResendJSON(w, http.StatusUnprocessableEntity, `{"statusCode":422,"name":"validation_error","message":"Invalid from"}`)
			},
			contains: "Invalid from",
		},
		{
			name: "401 invalid api key",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				writeResendJSON(w, http.StatusUnauthorized, `{"statusCode":401,"name":"missing_api_key","message":"Missing API key"}`)
			},
			contains: "Missing API key",
		},
		{
			name: "500 server error",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				writeResendJSON(w, http.StatusInternalServerError, `{"statusCode":500,"name":"internal_server_error","message":"Something went wrong"}`)
			},
			contains: "Something went wrong",
		},
		{
			name: "429 rate limit",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("ratelimit-limit", "2")
				w.Header().Set("ratelimit-remaining", "0")
				w.Header().Set("ratelimit-reset", "1")
				w.Header().Set("retry-after", "1")
				writeResendJSON(w, http.StatusTooManyRequests, `{"statusCode":429,"name":"rate_limit_exceeded","message":"Too many requests"}`)
			},
			contains:    "Too many requests",
			isRateLimit: true,
		},
		{
			name: "200 with empty id",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				writeResendJSON(w, http.StatusOK, `{"id":""}`)
			},
			contains: "empty message id",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			client := newResendTestServerClient(t, test.handler)
			email := newProviderTestEmail(t)
			email.Recipients = []string{testRecipientSuccess}

			err := sendViaResend(context.Background(), client, email)
			require.ErrorIs(t, err, ErrResendError)
			require.ErrorContains(t, err, test.contains)
			if test.isRateLimit {
				require.ErrorIs(t, err, resend.ErrRateLimit)
			}
		})
	}
}

// TestSendViaResend_RealSDKContextCanceled confirms a canceled context aborts
// the send through the real SDK and is surfaced as a wrapped error
func TestSendViaResend_RealSDKContextCanceled(t *testing.T) {
	t.Parallel()

	client := newResendTestServerClient(t, func(w http.ResponseWriter, _ *http.Request) {
		writeResendJSON(w, http.StatusOK, `{"id":"re_abc"}`)
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	email := newProviderTestEmail(t)
	email.Recipients = []string{testRecipientSuccess}

	err := sendViaResend(ctx, client, email)
	require.ErrorIs(t, err, ErrResendError)
	require.ErrorIs(t, err, context.Canceled)
}
