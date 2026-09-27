package gomail

import (
	"context"
	"errors"
	"testing"

	"github.com/mrz1836/postmark"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errPostmarkUnauthorized is a test-only Postmark API error
var errPostmarkUnauthorized = errors.New("401 unauthorized")

// fakePostmarkClient records the last email sent
type fakePostmarkClient struct {
	err      error
	email    postmark.Email
	response postmark.EmailResponse
}

// SendEmail records the email and returns the configured outcome
func (f *fakePostmarkClient) SendEmail(_ context.Context, email postmark.Email) (postmark.EmailResponse, error) {
	f.email = email
	return f.response, f.err
}

// newFakePostmarkClient returns a fake Postmark client that succeeds
func newFakePostmarkClient() *fakePostmarkClient {
	return &fakePostmarkClient{response: postmark.EmailResponse{MessageID: testMessageID}}
}

// TestPostmarkProviderSend checks the Postmark email
func TestPostmarkProviderSend(t *testing.T) {
	t.Parallel()

	client := newFakePostmarkClient()
	email := newProviderTestEmail(t)
	email.FromName = "Acme, Inc."
	email.Recipients = []string{"one@example.com", "Two <two@example.com>"}
	email.RecipientsCc = []string{testCcAddress}
	email.RecipientsBcc = []string{testBccAddress}
	email.Tags = []string{"a", "b"}
	email.Metadata = map[string]string{"user": "42"}
	email.SetHeader("X-Campaign", "spring")
	email.AddInlineAttachment("logo.png", "image/png", "logo", []byte("png"))

	result, err := NewPostmarkProvider(client).Send(context.Background(), email)
	require.NoError(t, err)
	assert.Equal(t, testMessageID, result.MessageID)
	assert.Equal(t, client.response, result.Response)

	sent := client.email
	assert.Equal(t, `"Acme, Inc." <no-reply@example.com>`, sent.From)
	assert.Equal(t, `one@example.com,"Two" <two@example.com>`, sent.To)
	assert.Equal(t, testCcAddress, sent.Cc)
	assert.Equal(t, testBccAddress, sent.Bcc)
	assert.Equal(t, "no-reply@example.com", sent.ReplyTo)
	assert.Equal(t, "Test Subject", sent.Subject)
	assert.Equal(t, "<html>Test</html>", sent.HTMLBody)
	assert.Equal(t, "Test", sent.TextBody)
	assert.Equal(t, "a,b", sent.Tag)
	assert.Equal(t, map[string]string{"user": "42"}, sent.Metadata)
	assert.True(t, sent.TrackOpens)
	assert.Equal(t, postmarkTrackLinksAll, sent.TrackLinks)
	assert.Equal(t, []postmark.Header{
		{Name: headerImportance, Value: headerHighValue},
		{Name: "X-Campaign", Value: "spring"},
		{Name: headerXMSMailPriority, Value: headerHighValue},
		{Name: headerXPriority, Value: headerXPriorityValue},
	}, sent.Headers)

	require.Len(t, sent.Attachments, 2)
	assert.Equal(t, "test-attachment-file.txt", sent.Attachments[0].Name)
	assert.Empty(t, sent.Attachments[0].ContentID)
	assert.Equal(t, "cid:logo", sent.Attachments[1].ContentID)
}

// TestPostmarkProviderSendMinimal checks tracking is off and optional fields are empty
func TestPostmarkProviderSendMinimal(t *testing.T) {
	t.Parallel()

	client := newFakePostmarkClient()
	_, err := NewPostmarkProvider(client).Send(context.Background(), newValidEmail())
	require.NoError(t, err)
	assert.Equal(t, postmarkTrackLinksNone, client.email.TrackLinks)
	assert.False(t, client.email.TrackOpens)
	assert.Empty(t, client.email.ReplyTo)
	assert.Empty(t, client.email.Cc)
	assert.Nil(t, client.email.Headers)
	assert.Empty(t, client.email.Attachments)
}

// TestPostmarkProviderOptions checks that PostmarkOption customizes the email
func TestPostmarkProviderOptions(t *testing.T) {
	t.Parallel()

	client := newFakePostmarkClient()
	email := newValidEmail().With(PostmarkOption(func(e *postmark.Email) { e.MessageStream = "broadcast" }))

	_, err := NewPostmarkProvider(client).Send(context.Background(), email)
	require.NoError(t, err)
	assert.Equal(t, "broadcast", client.email.MessageStream)
}

// TestPostmarkProviderErrors checks the Postmark failure paths
func TestPostmarkProviderErrors(t *testing.T) {
	t.Parallel()

	t.Run("api error", func(t *testing.T) {
		_, err := NewPostmarkProvider(&fakePostmarkClient{err: errPostmarkUnauthorized}).Send(context.Background(), newValidEmail())
		require.ErrorIs(t, err, ErrPostmarkError)
		require.ErrorIs(t, err, errPostmarkUnauthorized)
	})

	t.Run("error code", func(t *testing.T) {
		client := &fakePostmarkClient{response: postmark.EmailResponse{ErrorCode: 406, Message: "inactive recipient"}}
		_, err := NewPostmarkProvider(client).Send(context.Background(), newValidEmail())
		require.ErrorIs(t, err, ErrPostmarkError)
		assert.Contains(t, err.Error(), "inactive recipient error code: 406")
	})

	t.Run("invalid recipient", func(t *testing.T) {
		email := newValidEmail()
		email.RecipientsBcc = []string{"@"}
		_, err := NewPostmarkProvider(newFakePostmarkClient()).Send(context.Background(), email)
		require.ErrorIs(t, err, ErrInvalidRecipient)
	})

	t.Run("attachment read error", func(t *testing.T) {
		email := newValidEmail()
		email.AddAttachment(testFileName, "text/plain", errReader{})
		_, err := NewPostmarkProvider(newFakePostmarkClient()).Send(context.Background(), email)
		require.Error(t, err)
	})
}

// TestPostmarkProviderSupportsFeature checks the Postmark feature support
func TestPostmarkProviderSupportsFeature(t *testing.T) {
	t.Parallel()

	provider := NewPostmarkProvider(nil)
	for _, feature := range []Feature{FeatureMetadata, FeatureTags, FeatureTrackClicks, FeatureTrackOpens} {
		assert.True(t, provider.SupportsFeature(feature), feature)
	}
	for _, feature := range []Feature{FeatureAutoText, FeatureSendAt, FeatureIdempotencyKey, FeatureViewContentLink} {
		assert.False(t, provider.SupportsFeature(feature), feature)
	}
}

// TestNewPostmarkProviderWithSDKClient checks that *postmark.Client satisfies PostmarkClient
func TestNewPostmarkProviderWithSDKClient(t *testing.T) {
	t.Parallel()

	require.NotNil(t, NewPostmarkProvider(postmark.NewClient("token", "")))
}
