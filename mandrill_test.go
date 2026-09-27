package gomail

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/mattbaird/gochimp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errMandrillInvalidKey is a test-only Mandrill API error
var errMandrillInvalidKey = errors.New("invalid API key")

// fakeMandrillClient records the last message sent
type fakeMandrillClient struct {
	err       error
	block     chan struct{}
	message   gochimp.Message
	options   gochimp.MessageSendOptions
	responses []gochimp.SendResponse
	panics    bool
}

// MessageSendWithOptions records the message and returns the configured outcome
func (f *fakeMandrillClient) MessageSendWithOptions(message gochimp.Message, opts gochimp.MessageSendOptions) ([]gochimp.SendResponse, error) {
	if f.panics {
		panic("mandrill exploded")
	}
	if f.block != nil {
		<-f.block
	}
	f.message = message
	f.options = opts
	return f.responses, f.err
}

// newFakeMandrillClient returns a fake Mandrill client accepting every recipient
func newFakeMandrillClient() *fakeMandrillClient {
	return &fakeMandrillClient{responses: []gochimp.SendResponse{
		{Email: testRecipientSuccess, Status: mandrillStatusSent, Id: testMessageID},
		{Email: "other@example.com", Status: mandrillStatusQueued, Id: "second"},
	}}
}

// TestMandrillProviderSend checks the Mandrill message
func TestMandrillProviderSend(t *testing.T) {
	t.Parallel()

	client := newFakeMandrillClient()
	provider := NewMandrillProvider(client)

	email := newProviderTestEmail(t)
	email.Recipients = []string{"To Person <to@example.com>"}
	email.RecipientsCc = []string{testCcAddress}
	email.RecipientsBcc = []string{testBccAddress}
	email.ReplyToAddress = "Support <support@example.com>"
	email.Tags = []string{"welcome"}
	email.Metadata = map[string]string{"user": "42"}
	email.ViewContentLink = true
	email.SetHeader("X-Campaign", "spring")
	email.AddInlineAttachment("logo.png", "image/png", "logo", []byte("png"))

	result, err := provider.Send(context.Background(), email)
	require.NoError(t, err)
	assert.Equal(t, testMessageID, result.MessageID)
	assert.Equal(t, client.responses, result.Response)

	message := client.message
	assert.Equal(t, "no-reply@example.com", message.FromEmail)
	assert.Equal(t, testFromNameEmail, message.FromName)
	assert.Equal(t, testDomainEmail, message.SigningDomain)
	assert.Equal(t, "Test Subject", message.Subject)
	assert.Equal(t, "<html>Test</html>", message.Html)
	assert.Equal(t, "Test", message.Text)
	assert.True(t, message.AutoText)
	assert.True(t, message.Important)
	assert.True(t, message.TrackClicks)
	assert.True(t, message.TrackOpens)
	assert.True(t, message.ViewContentLink)
	assert.False(t, message.PreserveRecipients)
	assert.Equal(t, []string{"welcome"}, message.Tags)
	assert.Equal(t, map[string]string{"user": "42"}, message.Metadata)
	assert.Equal(t, []gochimp.Recipient{
		{Email: "to@example.com", Name: "To Person", Type: "to"},
		{Email: testCcAddress, Type: "cc"},
		{Email: testBccAddress, Type: "bcc"},
	}, message.To)
	assert.Equal(t, `"Support" <support@example.com>`, message.Headers["Reply-To"])
	assert.Equal(t, "spring", message.Headers["X-Campaign"])
	assert.Equal(t, headerXPriorityValue, message.Headers[headerXPriority])

	require.Len(t, message.Attachments, 1)
	assert.Equal(t, "test-attachment-file.txt", message.Attachments[0].Name)
	assert.Equal(t, "text/plain", message.Attachments[0].Type)
	assert.NotEmpty(t, message.Attachments[0].Content)
	require.Len(t, message.Images, 1)
	assert.Equal(t, gochimp.Attachment{Name: "logo", Type: "image/png", Content: base64.StdEncoding.EncodeToString([]byte("png"))}, message.Images[0])

	assert.False(t, client.options.Async, "sends are synchronous so rejections are reported")
	assert.Nil(t, client.options.SendAt)
}

// TestMandrillProviderSendAt checks scheduled sending
func TestMandrillProviderSendAt(t *testing.T) {
	t.Parallel()

	client := newFakeMandrillClient()
	email := newValidEmail()
	email.SendAt = time.Date(2030, time.January, 1, 0, 0, 0, 0, time.UTC)

	_, err := NewMandrillProvider(client).Send(context.Background(), email)
	require.NoError(t, err)
	require.NotNil(t, client.options.SendAt)
	assert.True(t, client.options.SendAt.Equal(email.SendAt))
}

// TestMandrillProviderOptions checks that MandrillOption customizes the message
func TestMandrillProviderOptions(t *testing.T) {
	t.Parallel()

	client := newFakeMandrillClient()
	email := newValidEmail().With(MandrillOption(func(message *gochimp.Message) {
		message.Subaccount = "tenant-1"
		message.PreserveRecipients = true
	}))

	_, err := NewMandrillProvider(client).Send(context.Background(), email)
	require.NoError(t, err)
	assert.Equal(t, "tenant-1", client.message.Subaccount)
	assert.True(t, client.message.PreserveRecipients)
}

// TestMandrillProviderRejections checks that rejected recipients are errors
func TestMandrillProviderRejections(t *testing.T) {
	t.Parallel()

	client := &fakeMandrillClient{responses: []gochimp.SendResponse{
		{Email: testAddressA, Status: mandrillStatusSent},
		{Email: "b@example.com", Status: "rejected", RejectedReason: "hard-bounce"},
		{Email: "c@example.com", Status: "invalid"},
	}}

	_, err := NewMandrillProvider(client).Send(context.Background(), newValidEmail())
	require.ErrorIs(t, err, ErrMessageNotSent)
	assert.Contains(t, err.Error(), "b@example.com status was rejected and not sent - given reason: hard-bounce")
	assert.Contains(t, err.Error(), "c@example.com status was invalid")
	assert.NotContains(t, err.Error(), testAddressA)
}

// TestMandrillProviderScheduledStatus checks the scheduled status is accepted
func TestMandrillProviderScheduledStatus(t *testing.T) {
	t.Parallel()

	client := &fakeMandrillClient{responses: []gochimp.SendResponse{{Status: mandrillStatusScheduled, Id: "s1"}}}
	result, err := NewMandrillProvider(client).Send(context.Background(), newValidEmail())
	require.NoError(t, err)
	assert.Equal(t, "s1", result.MessageID)
}

// TestMandrillProviderEmptyResponse checks an empty response is a success without an id
func TestMandrillProviderEmptyResponse(t *testing.T) {
	t.Parallel()

	result, err := NewMandrillProvider(&fakeMandrillClient{}).Send(context.Background(), newValidEmail())
	require.NoError(t, err)
	assert.Empty(t, result.MessageID)
}

// TestMandrillProviderErrors checks the Mandrill failure paths
func TestMandrillProviderErrors(t *testing.T) {
	t.Parallel()

	t.Run("api error", func(t *testing.T) {
		_, err := NewMandrillProvider(&fakeMandrillClient{err: errMandrillInvalidKey}).Send(context.Background(), newValidEmail())
		require.ErrorIs(t, err, errMandrillInvalidKey)
	})

	t.Run("client panic is recovered", func(t *testing.T) {
		_, err := NewMandrillProvider(&fakeMandrillClient{panics: true}).Send(context.Background(), newValidEmail())
		require.ErrorIs(t, err, ErrMessageNotSent)
		assert.Contains(t, err.Error(), "mandrill exploded")
	})

	t.Run("invalid from address", func(t *testing.T) {
		email := newValidEmail()
		email.FromAddress = "user@"
		_, err := NewMandrillProvider(newFakeMandrillClient()).Send(context.Background(), email)
		require.ErrorIs(t, err, ErrInvalidFromAddress)
	})

	t.Run("attachment read error", func(t *testing.T) {
		email := newValidEmail()
		email.AddAttachment(testFileName, "text/plain", errReader{})
		_, err := NewMandrillProvider(newFakeMandrillClient()).Send(context.Background(), email)
		require.Error(t, err)
	})
}

// TestMandrillProviderHonorsContext checks that a hung request cannot block a send
func TestMandrillProviderHonorsContext(t *testing.T) {
	t.Parallel()

	t.Run("deadline", func(t *testing.T) {
		client := newFakeMandrillClient()
		client.block = make(chan struct{})
		defer close(client.block)

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		_, err := NewMandrillProvider(client).Send(ctx, newValidEmail())
		require.ErrorIs(t, err, context.DeadlineExceeded)
	})

	t.Run("already canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := NewMandrillProvider(newFakeMandrillClient()).Send(ctx, newValidEmail())
		require.ErrorIs(t, err, context.Canceled)
	})
}

// TestMandrillProviderSupportsFeature checks the Mandrill feature support
func TestMandrillProviderSupportsFeature(t *testing.T) {
	t.Parallel()

	provider := NewMandrillProvider(nil)
	for _, feature := range []Feature{
		FeatureAutoText, FeatureMetadata, FeatureSendAt, FeatureTags,
		FeatureTrackClicks, FeatureTrackOpens, FeatureViewContentLink,
	} {
		assert.True(t, provider.SupportsFeature(feature), feature)
	}
	assert.False(t, provider.SupportsFeature(FeatureIdempotencyKey))
}

// TestNewMandrillProviderWithSDKClient checks that *gochimp.MandrillAPI satisfies MandrillClient
func TestNewMandrillProviderWithSDKClient(t *testing.T) {
	t.Parallel()

	api, err := gochimp.NewMandrill("key")
	require.NoError(t, err)
	require.NotNil(t, NewMandrillProvider(api))
}
