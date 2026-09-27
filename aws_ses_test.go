package gomail

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errSESThrottled is a test-only SES API error
var errSESThrottled = errors.New("throttling: rate exceeded")

// fakeSESClient records the last SendRawEmail input
type fakeSESClient struct {
	err    error
	input  *ses.SendRawEmailInput
	output *ses.SendRawEmailOutput
}

// SendRawEmail records the input and returns the configured outcome
func (f *fakeSESClient) SendRawEmail(_ context.Context, params *ses.SendRawEmailInput, _ ...func(*ses.Options)) (*ses.SendRawEmailOutput, error) {
	f.input = params
	return f.output, f.err
}

// newFakeSESClient returns a fake SES client that succeeds
func newFakeSESClient() *fakeSESClient {
	return &fakeSESClient{output: &ses.SendRawEmailOutput{MessageId: aws.String(testMessageID)}}
}

// newTestSESProvider returns an SES provider with a fixed clock
func newTestSESProvider(client SESClient, configurationSet string) *SESProvider {
	provider := NewSESProvider(client, configurationSet)
	provider.now = testMIMEDate
	return provider
}

// TestSESProviderSend checks the SendRawEmail request
func TestSESProviderSend(t *testing.T) {
	t.Parallel()

	client := newFakeSESClient()
	provider := newTestSESProvider(client, "tracking")

	email := newProviderTestEmail(t)
	email.Recipients = []string{"To <to@example.com>", "dup@example.com"}
	email.RecipientsCc = []string{testCcAddress, "DUP@example.com"}
	email.RecipientsBcc = []string{testBccAddress}
	email.Tags = []string{"welcome"}
	email.Metadata = map[string]string{"user": "42"}

	result, err := provider.Send(context.Background(), email)
	require.NoError(t, err)
	assert.Equal(t, testMessageID, result.MessageID)
	assert.Equal(t, client.output, result.Response)

	input := client.input
	assert.Equal(t, []string{"to@example.com", "dup@example.com", testCcAddress, testBccAddress}, input.Destinations)
	assert.Equal(t, "tracking", aws.ToString(input.ConfigurationSetName))
	assert.Equal(t, []types.MessageTag{
		{Name: aws.String("welcome"), Value: aws.String(tagValueMarker)},
		{Name: aws.String("user"), Value: aws.String("42")},
	}, input.Tags)

	// The raw message never discloses Bcc recipients
	parsed := parseMIME(t, input.RawMessage.Data)
	assert.Empty(t, parsed.header.Get("Bcc"))
	assert.NotContains(t, string(input.RawMessage.Data), testBccAddress)
	assert.Empty(t, parsed.header.Get("Message-Id"), "SES assigns the Message-ID")
	assert.Equal(t, "Test", parsed.part(t, mimeTypePlain).content)
	require.Len(t, parsed.parts, 3)
	disposition, filename := partDisposition(parsed.parts[2].header)
	assert.Equal(t, "attachment", disposition)
	assert.Equal(t, "test-attachment-file.txt", filename)
}

// TestSESProviderSendMinimal checks optional fields are omitted
func TestSESProviderSendMinimal(t *testing.T) {
	t.Parallel()

	client := newFakeSESClient()
	_, err := newTestSESProvider(client, "").Send(context.Background(), newValidEmail())
	require.NoError(t, err)
	assert.Nil(t, client.input.ConfigurationSetName)
	assert.Nil(t, client.input.Tags)
}

// TestSESProviderOptions checks that SESOption customizes the request
func TestSESProviderOptions(t *testing.T) {
	t.Parallel()

	client := newFakeSESClient()
	email := newValidEmail().With(SESOption(func(input *ses.SendRawEmailInput) {
		input.FromArn = aws.String("arn:aws:ses:us-east-1:123:identity/example.com")
	}))

	_, err := newTestSESProvider(client, "").Send(context.Background(), email)
	require.NoError(t, err)
	assert.Equal(t, "arn:aws:ses:us-east-1:123:identity/example.com", aws.ToString(client.input.FromArn))
}

// TestSESProviderErrors checks the SES failure paths
func TestSESProviderErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		client   *fakeSESClient
		expected error
	}{
		{"api error", &fakeSESClient{err: errSESThrottled}, errSESThrottled},
		{"nil output", &fakeSESClient{}, ErrInvalidAWSResponse},
		{"nil message id", &fakeSESClient{output: &ses.SendRawEmailOutput{}}, ErrInvalidAWSResponse},
		{"empty message id", &fakeSESClient{output: &ses.SendRawEmailOutput{MessageId: aws.String("")}}, ErrInvalidAWSResponse},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := newTestSESProvider(test.client, "").Send(context.Background(), newValidEmail())
			require.ErrorIs(t, err, test.expected)
		})
	}
}

// TestSESProviderInvalidEmail checks envelope and attachment errors
func TestSESProviderInvalidEmail(t *testing.T) {
	t.Parallel()

	provider := newTestSESProvider(newFakeSESClient(), "")

	badReplyTo := newValidEmail()
	badReplyTo.ReplyToAddress = "not valid"
	_, err := provider.Send(context.Background(), badReplyTo)
	require.ErrorIs(t, err, ErrInvalidReplyToAddress)

	badAttachment := newValidEmail()
	badAttachment.AddAttachment(testFileName, "text/plain", errReader{})
	_, err = provider.Send(context.Background(), badAttachment)
	require.Error(t, err)
}

// TestSESProviderSupportsFeature checks the SES feature support
func TestSESProviderSupportsFeature(t *testing.T) {
	t.Parallel()

	provider := NewSESProvider(nil, "")
	assert.True(t, provider.SupportsFeature(FeatureTags))
	assert.True(t, provider.SupportsFeature(FeatureMetadata))
	for _, feature := range []Feature{FeatureAutoText, FeatureSendAt, FeatureTrackClicks, FeatureTrackOpens, FeatureIdempotencyKey} {
		assert.False(t, provider.SupportsFeature(feature), feature)
	}
}

// TestNewSESProviderWithSDKClient checks that *ses.Client satisfies SESClient
func TestNewSESProviderWithSDKClient(t *testing.T) {
	t.Parallel()

	provider := NewSESProvider(ses.New(ses.Options{Region: awsSesDefaultRegion}), "")
	require.NotNil(t, provider)
}
