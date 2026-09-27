package gomail

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/resend/resend-go/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Resend test values
const (
	testResendAPIKey       = "re_test"
	testResendAttachment   = "examples/test-attachment-file.txt"
	testResendAttachmentFn = "test-attachment-file.txt"
)

// resendWireRequest is the JSON body the Resend SDK sends to POST /emails
type resendWireRequest struct {
	Headers     map[string]string `json:"headers"`
	From        string            `json:"from"`
	ReplyTo     string            `json:"reply_to"`
	Subject     string            `json:"subject"`
	HTML        string            `json:"html"`
	Text        string            `json:"text"`
	ScheduledAt string            `json:"scheduled_at"`
	To          []string          `json:"to"`
	Cc          []string          `json:"cc"`
	Bcc         []string          `json:"bcc"`
	Tags        []resend.Tag      `json:"tags"`
	Attachments []struct {
		Filename    string `json:"filename"`
		ContentType string `json:"content_type"`
		ContentID   string `json:"content_id"`
		Content     []int  `json:"content"`
	} `json:"attachments"`
}

// resendCapture records the HTTP request the Resend SDK sends
type resendCapture struct {
	header http.Header
	method string
	path   string
	body   resendWireRequest
	err    error
	mu     sync.Mutex
}

// newResendTestServer starts an httptest server and returns a Resend provider
// using the real SDK pointed at it
func newResendTestServer(t *testing.T, handler http.HandlerFunc) *ResendProvider {
	t.Helper()

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	client := resend.NewCustomClient(srv.Client(), testResendAPIKey)
	baseURL, err := url.Parse(srv.URL + "/")
	require.NoError(t, err)
	client.BaseURL = baseURL

	return NewResendProvider(client.Emails)
}

// newCapturingResendServer returns a provider whose server records each request
// and responds with the given status and body
func newCapturingResendServer(t *testing.T, status int, body string) (*ResendProvider, *resendCapture) {
	t.Helper()

	capture := &resendCapture{}
	provider := newResendTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		capture.mu.Lock()
		capture.method = r.Method
		capture.path = r.URL.Path
		capture.header = r.Header.Clone()
		capture.err = json.NewDecoder(r.Body).Decode(&capture.body)
		capture.mu.Unlock()
		writeResendJSON(w, status, body)
	})
	return provider, capture
}

// writeResendJSON writes a JSON response with the given status code
func writeResendJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

// TestResendProviderSendWire checks the HTTP request the real SDK sends
func TestResendProviderSendWire(t *testing.T) {
	t.Parallel()

	provider, capture := newCapturingResendServer(t, http.StatusOK, `{"id":"re_abc"}`)

	email := newProviderTestEmail(t)
	email.Subject = "Wire subject"
	email.Recipients = []string{"To Person <to@domain.com>"}
	email.RecipientsCc = []string{"cc@domain.com"}
	email.RecipientsBcc = []string{"bcc@domain.com"}
	email.ReplyToAddress = "reply@domain.com"
	email.Tags = []string{"admin alert"}
	email.Metadata = map[string]string{"user id": "42"}
	email.IdempotencyKey = "welcome/42"
	email.SendAt = time.Date(2030, time.January, 2, 3, 4, 5, 0, time.FixedZone("EST", -5*3600))
	email.SetHeader("X-Campaign", "spring")
	email.AddInlineAttachment("logo.png", "image/png", "logo", []byte{1, 2})

	result, err := provider.Send(context.Background(), email)
	require.NoError(t, err)
	assert.Equal(t, "re_abc", result.MessageID)
	assert.Equal(t, &resend.SendEmailResponse{Id: "re_abc"}, result.Response)
	require.NoError(t, capture.err)

	// Request line and headers
	assert.Equal(t, http.MethodPost, capture.method)
	assert.Equal(t, "/emails", capture.path)
	assert.Equal(t, "Bearer "+testResendAPIKey, capture.header.Get("Authorization"))
	assert.Equal(t, "welcome/42", capture.header.Get("Idempotency-Key"))

	// JSON body
	body := capture.body
	assert.Equal(t, `"No Reply" <no-reply@example.com>`, body.From)
	assert.Equal(t, []string{`"To Person" <to@domain.com>`}, body.To)
	assert.Equal(t, []string{"cc@domain.com"}, body.Cc)
	assert.Equal(t, []string{"bcc@domain.com"}, body.Bcc)
	assert.Equal(t, "reply@domain.com", body.ReplyTo)
	assert.Equal(t, "Wire subject", body.Subject)
	assert.Equal(t, "<html>Test</html>", body.HTML)
	assert.Equal(t, "Test", body.Text)
	assert.Equal(t, "2030-01-02T08:04:05Z", body.ScheduledAt)
	assert.Equal(t, []resend.Tag{
		{Name: "admin_alert", Value: tagValueMarker},
		{Name: "user_id", Value: "42"},
	}, body.Tags)
	assert.Equal(t, map[string]string{
		headerXPriority:       headerXPriorityValue,
		headerXMSMailPriority: headerHighValue,
		headerImportance:      headerHighValue,
		"X-Campaign":          "spring",
	}, body.Headers)

	// Attachment content is sent as an array of byte values
	expected, err := os.ReadFile(testResendAttachment)
	require.NoError(t, err)
	require.Len(t, body.Attachments, 2)
	assert.Equal(t, testResendAttachmentFn, body.Attachments[0].Filename)
	assert.Equal(t, "text/plain", body.Attachments[0].ContentType)
	assert.Len(t, body.Attachments[0].Content, len(expected))
	assert.Equal(t, "logo", body.Attachments[1].ContentID)
	assert.Equal(t, []int{1, 2}, body.Attachments[1].Content)
}

// TestResendProviderSendMinimal checks optional fields are omitted
func TestResendProviderSendMinimal(t *testing.T) {
	t.Parallel()

	provider, capture := newCapturingResendServer(t, http.StatusOK, `{"id":"re_abc"}`)

	_, err := provider.Send(context.Background(), newValidEmail())
	require.NoError(t, err)

	assert.Empty(t, capture.header.Get("Idempotency-Key"))
	assert.Equal(t, "no-reply@example.com", capture.body.From)
	assert.Empty(t, capture.body.ReplyTo)
	assert.Empty(t, capture.body.ScheduledAt)
	assert.Nil(t, capture.body.Tags)
	assert.Nil(t, capture.body.Headers)
}

// TestResendProviderOptions checks that ResendOption customizes the request
func TestResendProviderOptions(t *testing.T) {
	t.Parallel()

	provider, capture := newCapturingResendServer(t, http.StatusOK, `{"id":"re_abc"}`)

	email := newValidEmail().With(
		ResendOption(func(r *resend.SendEmailRequest) { r.Subject = "Overridden" }),
		PostmarkOption(nil), // other providers' options are ignored
	)
	_, err := provider.Send(context.Background(), email)
	require.NoError(t, err)
	assert.Equal(t, "Overridden", capture.body.Subject)
}

// TestResendProviderErrors checks that API errors are wrapped in ErrResendError
func TestResendProviderErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		status      int
		body        string
		contains    string
		isRateLimit bool
	}{
		{"422 validation error", http.StatusUnprocessableEntity, `{"statusCode":422,"name":"validation_error","message":"Invalid from"}`, "Invalid from", false},
		{"401 invalid api key", http.StatusUnauthorized, `{"statusCode":401,"name":"missing_api_key","message":"Missing API key"}`, "Missing API key", false},
		{"500 server error", http.StatusInternalServerError, `{"statusCode":500,"name":"internal_server_error","message":"Something went wrong"}`, "Something went wrong", false},
		{"429 rate limit", http.StatusTooManyRequests, `{"statusCode":429,"name":"rate_limit_exceeded","message":"Too many requests"}`, "Too many requests", true},
		{"200 with empty id", http.StatusOK, `{"id":""}`, "empty message id", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			provider, _ := newCapturingResendServer(t, test.status, test.body)
			_, err := provider.Send(context.Background(), newValidEmail())
			require.ErrorIs(t, err, ErrResendError)
			require.ErrorContains(t, err, test.contains)
			if test.isRateLimit {
				require.ErrorIs(t, err, resend.ErrRateLimit)
			}
		})
	}
}

// TestResendProviderNilResponse checks a nil response without an error
func TestResendProviderNilResponse(t *testing.T) {
	t.Parallel()

	provider := NewResendProvider(resendClientFunc(func(context.Context, *resend.SendEmailRequest, *resend.SendEmailOptions) (*resend.SendEmailResponse, error) {
		return nil, nil //nolint:nilnil // intentionally exercising the nil-response guard
	}))
	_, err := provider.Send(context.Background(), newValidEmail())
	require.ErrorIs(t, err, ErrResendError)
}

// TestResendProviderContextCanceled checks a canceled context aborts the send
func TestResendProviderContextCanceled(t *testing.T) {
	t.Parallel()

	provider, _ := newCapturingResendServer(t, http.StatusOK, `{"id":"re_abc"}`)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := provider.Send(ctx, newValidEmail())
	require.ErrorIs(t, err, ErrResendError)
	require.ErrorIs(t, err, context.Canceled)
}

// TestResendProviderInvalidEmail checks envelope and attachment errors
func TestResendProviderInvalidEmail(t *testing.T) {
	t.Parallel()

	provider := NewResendProvider(nil)

	badRecipient := newValidEmail()
	badRecipient.Recipients = []string{"not an address"}
	_, err := provider.Send(context.Background(), badRecipient)
	require.ErrorIs(t, err, ErrInvalidRecipient)

	badAttachment := newValidEmail()
	badAttachment.AddAttachment(testFileName, "text/plain", errReader{})
	_, err = provider.Send(context.Background(), badAttachment)
	require.Error(t, err)
}

// TestResendProviderSupportsFeature checks the Resend feature support
func TestResendProviderSupportsFeature(t *testing.T) {
	t.Parallel()

	provider := NewResendProvider(nil)
	for _, feature := range []Feature{FeatureAutoText, FeatureIdempotencyKey, FeatureMetadata, FeatureSendAt, FeatureTags} {
		assert.True(t, provider.SupportsFeature(feature), feature)
	}
	for _, feature := range []Feature{FeatureTrackClicks, FeatureTrackOpens, FeatureViewContentLink} {
		assert.False(t, provider.SupportsFeature(feature), feature)
	}
}

// TestResendTags checks the Resend tag conversion
func TestResendTags(t *testing.T) {
	t.Parallel()

	assert.Nil(t, resendTags(nil, nil))
	assert.Nil(t, resendTags([]string{""}, nil))
	assert.Equal(t, []resend.Tag{{Name: "a", Value: tagValueMarker}, {Name: "k", Value: "v"}},
		resendTags([]string{"a"}, map[string]string{"k": "v"}))
}

// TestNewResendProviderWithSDKClient checks the SDK emails service satisfies ResendClient
func TestNewResendProviderWithSDKClient(t *testing.T) {
	t.Parallel()

	provider := NewResendProvider(resend.NewClient("test-api-key").Emails)
	require.NotNil(t, provider)
}

// resendClientFunc adapts a function to ResendClient
type resendClientFunc func(ctx context.Context, params *resend.SendEmailRequest, options *resend.SendEmailOptions) (*resend.SendEmailResponse, error)

// SendWithOptions calls the function
func (f resendClientFunc) SendWithOptions(ctx context.Context, params *resend.SendEmailRequest, options *resend.SendEmailOptions) (*resend.SendEmailResponse, error) {
	return f(ctx, params, options)
}
