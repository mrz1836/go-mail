package gomail

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/mailgun/mailgun-go/v5"
	"github.com/mattbaird/gochimp"
	"github.com/mrz1836/postmark"
	"github.com/resend/resend-go/v4"
	sgmail "github.com/sendgrid/sendgrid-go/helpers/mail"
	"github.com/stretchr/testify/assert"
)

// TestRequestedFeatures checks which features an email requests
func TestRequestedFeatures(t *testing.T) {
	t.Parallel()

	assert.Empty(t, requestedFeatures(&Email{}))

	email := &Email{
		AutoText:        true,
		IdempotencyKey:  "key",
		Metadata:        map[string]string{"k": "v"},
		SendAt:          time.Now(),
		Tags:            []string{"t"},
		TrackClicks:     true,
		TrackOpens:      true,
		ViewContentLink: true,
	}
	assert.Equal(t, []Feature{
		FeatureAutoText, FeatureIdempotencyKey, FeatureMetadata, FeatureSendAt,
		FeatureTags, FeatureTrackClicks, FeatureTrackOpens, FeatureViewContentLink,
	}, requestedFeatures(email))
}

// TestUnsupportedFeatures checks the features a provider cannot honor
func TestUnsupportedFeatures(t *testing.T) {
	t.Parallel()

	email := &Email{TrackOpens: true, Tags: []string{"t"}}

	t.Run("provider without feature support assumes all", func(t *testing.T) {
		assert.Empty(t, unsupportedFeatures(&fakeProvider{}, email))
	})

	t.Run("provider reports support", func(t *testing.T) {
		provider := &featureFakeProvider{fakeProvider{supported: []Feature{FeatureTags}}}
		assert.Equal(t, []Feature{FeatureTrackOpens}, unsupportedFeatures(provider, email))
	})

	t.Run("built-in provider", func(t *testing.T) {
		assert.Equal(t, []Feature{FeatureTags, FeatureTrackOpens}, unsupportedFeatures(NewSMTPProvider(SMTPConfig{}), email))
	})
}

// TestProviderOptionServiceProvider checks every option reports its provider
func TestProviderOptionServiceProvider(t *testing.T) {
	t.Parallel()

	assert.Equal(t, AwsSes, SESOption(nil).ServiceProvider())
	assert.Equal(t, Mandrill, MandrillOption(nil).ServiceProvider())
	assert.Equal(t, Postmark, PostmarkOption(nil).ServiceProvider())
	assert.Equal(t, SendGrid, SendGridOption(nil).ServiceProvider())
	assert.Equal(t, Resend, ResendOption(nil).ServiceProvider())
	assert.Equal(t, Mailgun, MailgunOption(nil).ServiceProvider())
}

// customOption is a ProviderOption for a custom provider
type customOption struct{}

// ServiceProvider returns a custom provider id
func (customOption) ServiceProvider() ServiceProvider { return ServiceProvider(100) }

// TestApplyProviderOptions checks options are applied in order and filtered by type
func TestApplyProviderOptions(t *testing.T) {
	t.Parallel()

	opts := []ProviderOption{
		PostmarkOption(func(e *postmark.Email) { e.Subject += "a" }),
		SendGridOption(func(m *sgmail.SGMailV3) { m.Subject = "wrong" }),
		customOption{},
		PostmarkOption(nil),
		PostmarkOption(func(e *postmark.Email) { e.Subject += "b" }),
	}

	target := &postmark.Email{}
	applyProviderOptions[PostmarkOption](opts, target)
	assert.Equal(t, "ab", target.Subject)

	// Every option type compiles against its request type
	applyProviderOptions[SESOption](opts, &ses.SendRawEmailInput{})
	applyProviderOptions[MailgunOption](opts, mailgun.NewMessage("", "", "", ""))
	applyProviderOptions[MandrillOption](opts, &gochimp.Message{})
	applyProviderOptions[ResendOption](opts, &resend.SendEmailRequest{})
	applyProviderOptions[SendGridOption](opts, sgmail.NewV3Mail())
}
