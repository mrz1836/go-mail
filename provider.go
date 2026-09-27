package gomail

import (
	"context"
	"slices"

	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/mattbaird/gochimp"
	"github.com/mrz1836/postmark"
	"github.com/resend/resend-go/v4"
	sgmail "github.com/sendgrid/sendgrid-go/helpers/mail"
)

// Provider sends an email through a single email service.
//
// Every built-in service (AWS SES, Mandrill, Postmark, SendGrid, Resend and
// SMTP) implements Provider, and custom providers (or test fakes) can be added
// to a MailService with RegisterProvider. The email passed to Send has already
// been validated by MailService.
type Provider interface {
	Send(ctx context.Context, email *Email) (*SendResult, error)
}

// SendResult describes an email that was accepted by a provider
//
// DO NOT CHANGE ORDER - Optimized for memory (maligned)
type SendResult struct {
	Response  any             // native provider response (ie: postmark.EmailResponse, *rest.Response)
	MessageID string          // provider message id (the Message-ID header for SMTP)
	Provider  ServiceProvider // the provider that accepted the email
}

// Feature is an optional email feature that not every provider supports
type Feature string

// Optional email features, checked against the provider before every send
const (
	FeatureAutoText        Feature = "auto_text"         // Email.AutoText
	FeatureIdempotencyKey  Feature = "idempotency_key"   // Email.IdempotencyKey
	FeatureMetadata        Feature = "metadata"          // Email.Metadata
	FeatureSendAt          Feature = "send_at"           // Email.SendAt (never silently ignored)
	FeatureTags            Feature = "tags"              // Email.Tags
	FeatureTrackClicks     Feature = "track_clicks"      // Email.TrackClicks
	FeatureTrackOpens      Feature = "track_opens"       // Email.TrackOpens
	FeatureViewContentLink Feature = "view_content_link" // Email.ViewContentLink
)

// FeatureSupporter is optionally implemented by a Provider to report which
// optional features it supports. MailService logs a warning (or, with
// StrictFeatures, returns ErrUnsupportedFeature) when an email requests a
// feature the provider does not support. Providers that do not implement
// FeatureSupporter are assumed to support every feature.
type FeatureSupporter interface {
	SupportsFeature(feature Feature) bool
}

// requestedFeatures returns the optional features the email asks for
func requestedFeatures(email *Email) []Feature {
	var features []Feature
	if email.AutoText {
		features = append(features, FeatureAutoText)
	}
	if len(email.IdempotencyKey) > 0 {
		features = append(features, FeatureIdempotencyKey)
	}
	if len(email.Metadata) > 0 {
		features = append(features, FeatureMetadata)
	}
	if !email.SendAt.IsZero() {
		features = append(features, FeatureSendAt)
	}
	if len(email.Tags) > 0 {
		features = append(features, FeatureTags)
	}
	if email.TrackClicks {
		features = append(features, FeatureTrackClicks)
	}
	if email.TrackOpens {
		features = append(features, FeatureTrackOpens)
	}
	if email.ViewContentLink {
		features = append(features, FeatureViewContentLink)
	}
	return features
}

// unsupportedFeatures returns the requested features the provider does not support
func unsupportedFeatures(provider Provider, email *Email) []Feature {
	supporter, ok := provider.(FeatureSupporter)
	if !ok {
		return nil
	}

	var unsupported []Feature
	for _, feature := range requestedFeatures(email) {
		if !supporter.SupportsFeature(feature) {
			unsupported = append(unsupported, feature)
		}
	}
	return unsupported
}

// ProviderOption customizes the native request a single provider builds for an
// email. Options are applied after go-mail has mapped the Email onto the
// provider's request, so they can set anything the provider SDK exposes.
// Options for other providers are ignored, so one Email can carry options for
// several providers (useful with failover).
//
// Custom providers may define their own ProviderOption types and read them
// from Email.ProviderOptions.
type ProviderOption interface {
	ServiceProvider() ServiceProvider
}

// SESOption customizes the AWS SES SendRawEmail request (ie: FromArn, Tags)
type SESOption func(input *ses.SendRawEmailInput)

// ServiceProvider returns AwsSes
func (SESOption) ServiceProvider() ServiceProvider { return AwsSes }

// MandrillOption customizes the Mandrill message (ie: merge vars, subaccount)
type MandrillOption func(message *gochimp.Message)

// ServiceProvider returns Mandrill
func (MandrillOption) ServiceProvider() ServiceProvider { return Mandrill }

// PostmarkOption customizes the Postmark email (ie: MessageStream)
type PostmarkOption func(email *postmark.Email)

// ServiceProvider returns Postmark
func (PostmarkOption) ServiceProvider() ServiceProvider { return Postmark }

// SendGridOption customizes the SendGrid message (ie: template id, ASM group)
type SendGridOption func(message *sgmail.SGMailV3)

// ServiceProvider returns SendGrid
func (SendGridOption) ServiceProvider() ServiceProvider { return SendGrid }

// ResendOption customizes the Resend send request (ie: template, topic)
type ResendOption func(request *resend.SendEmailRequest)

// ServiceProvider returns Resend
func (ResendOption) ServiceProvider() ServiceProvider { return Resend }

// applyProviderOptions runs every option of type O in opts against target
func applyProviderOptions[O ~func(*T), T any](opts []ProviderOption, target *T) {
	for _, opt := range opts {
		if fn, ok := opt.(O); ok && fn != nil {
			fn(target)
		}
	}
}

// supportsFeature reports whether feature is in the supported list
func supportsFeature(supported []Feature, feature Feature) bool {
	return slices.Contains(supported, feature)
}
