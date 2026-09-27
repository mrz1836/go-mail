package gomail

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/sendgrid/rest"
	sendgrid "github.com/sendgrid/sendgrid-go"
	sgmail "github.com/sendgrid/sendgrid-go/helpers/mail"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errSendGridTransport is a test-only transport error
var errSendGridTransport = errors.New("dial tcp: no such host")

// fakeSendGridClient records the last message sent
type fakeSendGridClient struct {
	err      error
	message  *sgmail.SGMailV3
	response *rest.Response
}

// SendWithContext records the message and returns the configured outcome
func (f *fakeSendGridClient) SendWithContext(_ context.Context, email *sgmail.SGMailV3) (*rest.Response, error) {
	f.message = email
	return f.response, f.err
}

// newFakeSendGridClient returns a fake SendGrid client that succeeds
func newFakeSendGridClient() *fakeSendGridClient {
	return &fakeSendGridClient{response: &rest.Response{
		StatusCode: http.StatusAccepted,
		Headers:    map[string][]string{"X-Message-Id": {testMessageID}},
	}}
}

// TestSendGridProviderSend checks the SendGrid message
func TestSendGridProviderSend(t *testing.T) {
	t.Parallel()

	client := newFakeSendGridClient()
	email := newProviderTestEmail(t)
	email.Recipients = []string{"To Person <to@example.com>", "dup@example.com"}
	email.RecipientsCc = []string{testCcAddress, "dup@example.com"}
	email.RecipientsBcc = []string{testBccAddress, "to@example.com"}
	email.ReplyToAddress = "Support <support@example.com>"
	email.Tags = []string{"welcome"}
	email.Metadata = map[string]string{"user": "42"}
	email.SendAt = time.Unix(1893456000, 0)
	email.SetHeader("X-Campaign", "spring")
	email.AddInlineAttachment("logo.png", "image/png", "logo", []byte("png"))

	result, err := NewSendGridProvider(client).Send(context.Background(), email)
	require.NoError(t, err)
	assert.Equal(t, testMessageID, result.MessageID)
	assert.Equal(t, client.response, result.Response)

	message := client.message
	assert.Equal(t, &sgmail.Email{Name: testFromNameEmail, Address: "no-reply@example.com"}, message.From)
	assert.Equal(t, &sgmail.Email{Name: "Support", Address: "support@example.com"}, message.ReplyTo)
	assert.Equal(t, "Test Subject", message.Subject)

	// Duplicate recipients are removed (SendGrid rejects them)
	require.Len(t, message.Personalizations, 1)
	personalization := message.Personalizations[0]
	assert.Equal(t, []*sgmail.Email{{Name: "To Person", Address: "to@example.com"}, {Address: "dup@example.com"}}, personalization.To)
	assert.Equal(t, []*sgmail.Email{{Address: testCcAddress}}, personalization.CC)
	assert.Equal(t, []*sgmail.Email{{Address: testBccAddress}}, personalization.BCC)

	require.Len(t, message.Content, 2)
	assert.Equal(t, mimeTypePlain, message.Content[0].Type)
	assert.Equal(t, mimeTypeHTML, message.Content[1].Type)
	assert.Equal(t, []string{"welcome"}, message.Categories)
	assert.Equal(t, map[string]string{"user": "42"}, message.CustomArgs)
	assert.Equal(t, 1893456000, message.SendAt)
	assert.Equal(t, "spring", message.Headers["X-Campaign"])
	assert.Equal(t, headerXPriorityValue, message.Headers[headerXPriority])
	assert.True(t, *message.TrackingSettings.ClickTracking.Enable)
	assert.True(t, *message.TrackingSettings.OpenTracking.Enable)

	require.Len(t, message.Attachments, 2)
	assert.Equal(t, "attachment", message.Attachments[0].Disposition)
	assert.Equal(t, "test-attachment-file.txt", message.Attachments[0].Filename)
	assert.Equal(t, "inline", message.Attachments[1].Disposition)
	assert.Equal(t, "logo", message.Attachments[1].ContentID)
}

// TestSendGridProviderTrackingDisabled checks tracking is explicitly disabled
func TestSendGridProviderTrackingDisabled(t *testing.T) {
	t.Parallel()

	client := newFakeSendGridClient()
	_, err := NewSendGridProvider(client).Send(context.Background(), newValidEmail())
	require.NoError(t, err)

	tracking := client.message.TrackingSettings
	require.NotNil(t, tracking)
	assert.False(t, *tracking.ClickTracking.Enable)
	assert.False(t, *tracking.ClickTracking.EnableText)
	assert.False(t, *tracking.OpenTracking.Enable)
	assert.Nil(t, client.message.ReplyTo)
	assert.Zero(t, client.message.SendAt)
	assert.Empty(t, client.message.Categories)
}

// TestSendGridProviderOptions checks that SendGridOption customizes the message
func TestSendGridProviderOptions(t *testing.T) {
	t.Parallel()

	client := newFakeSendGridClient()
	email := newValidEmail().With(SendGridOption(func(message *sgmail.SGMailV3) { message.SetTemplateID("d-123") }))

	_, err := NewSendGridProvider(client).Send(context.Background(), email)
	require.NoError(t, err)
	assert.Equal(t, "d-123", client.message.TemplateID)
}

// TestSendGridProviderErrors checks the SendGrid failure paths
func TestSendGridProviderErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		client   *fakeSendGridClient
		contains string
	}{
		{"transport error", &fakeSendGridClient{err: errSendGridTransport}, "no such host"},
		{"nil response", &fakeSendGridClient{}, "empty response"},
		{"bad request", &fakeSendGridClient{response: &rest.Response{StatusCode: http.StatusBadRequest, Body: `{"errors":[]}`}}, "status code 400"},
		{"server error", &fakeSendGridClient{response: &rest.Response{StatusCode: http.StatusInternalServerError}}, "status code 500"},
		{"informational status", &fakeSendGridClient{response: &rest.Response{StatusCode: http.StatusContinue}}, "status code 100"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewSendGridProvider(test.client).Send(context.Background(), newValidEmail())
			require.ErrorIs(t, err, ErrSendGridError)
			assert.Contains(t, err.Error(), test.contains)
		})
	}

	t.Run("invalid reply-to", func(t *testing.T) {
		email := newValidEmail()
		email.ReplyToAddress = "nope"
		_, err := NewSendGridProvider(newFakeSendGridClient()).Send(context.Background(), email)
		require.ErrorIs(t, err, ErrInvalidReplyToAddress)
	})

	t.Run("attachment read error", func(t *testing.T) {
		email := newValidEmail()
		email.AddAttachment(testFileName, "text/plain", errReader{})
		_, err := NewSendGridProvider(newFakeSendGridClient()).Send(context.Background(), email)
		require.Error(t, err)
	})
}

// TestSendGridProviderSupportsFeature checks the SendGrid feature support
func TestSendGridProviderSupportsFeature(t *testing.T) {
	t.Parallel()

	provider := NewSendGridProvider(nil)
	for _, feature := range []Feature{FeatureMetadata, FeatureSendAt, FeatureTags, FeatureTrackClicks, FeatureTrackOpens} {
		assert.True(t, provider.SupportsFeature(feature), feature)
	}
	for _, feature := range []Feature{FeatureAutoText, FeatureIdempotencyKey, FeatureViewContentLink} {
		assert.False(t, provider.SupportsFeature(feature), feature)
	}
}

// TestNewSendGridProviderWrapsSDKClient checks the SDK client is wrapped for concurrent use
func TestNewSendGridProviderWrapsSDKClient(t *testing.T) {
	t.Parallel()

	provider := NewSendGridProvider(sendgrid.NewSendClient("key"))
	wrapped, ok := provider.client.(*sendGridRequestClient)
	require.True(t, ok)
	assert.Equal(t, "Bearer key", wrapped.request.Headers["Authorization"])

	custom := newFakeSendGridClient()
	assert.Same(t, custom, NewSendGridProvider(custom).client)
}

// TestSendGridProviderConcurrentSends sends concurrently through the real SDK
// and checks every request carries its own body (the SDK client stores the
// body on itself, so a shared client would mix up concurrent emails)
func TestSendGridProviderConcurrentSends(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Subject string `json:"subject"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		// Echo the subject back as the message id
		w.Header().Set("X-Message-Id", body.Subject)
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(server.Close)

	client := sendgrid.NewSendClient("key")
	client.BaseURL = server.URL + "/v3/mail/send"
	provider := NewSendGridProvider(client)

	const sends = 20
	var wg sync.WaitGroup
	errs := make(chan error, sends)
	for i := range sends {
		wg.Add(1)
		go func() {
			defer wg.Done()
			email := newValidEmail()
			email.Subject = "subject-" + strconv.Itoa(i)
			result, err := provider.Send(context.Background(), email)
			if err == nil && result.MessageID != email.Subject {
				err = errors.New("got the body of another email: " + result.MessageID) //nolint:err113 // test-only error
			}
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
}
