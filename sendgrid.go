package gomail

import (
	"context"
	"encoding/base64"
	"fmt"
	"maps"
	"net/http"
	"net/mail"
	"slices"

	"github.com/sendgrid/rest"
	sendgrid "github.com/sendgrid/sendgrid-go"
	sgmail "github.com/sendgrid/sendgrid-go/helpers/mail"
)

// headerSendGridMessageID is the response header holding the SendGrid message id
const headerSendGridMessageID = "X-Message-Id"

// SendGridClient is the SendGrid API used by SendGridProvider; *sendgrid.Client satisfies it
type SendGridClient interface {
	SendWithContext(ctx context.Context, email *sgmail.SGMailV3) (*rest.Response, error)
}

// SendGridProvider sends email through SendGrid.
//
// Tags become categories and Metadata becomes custom args. Open and click
// tracking are always set explicitly from the email, so the account defaults
// do not apply.
type SendGridProvider struct {
	client SendGridClient
}

// NewSendGridProvider creates a SendGrid provider.
//
// A *sendgrid.Client stores each request body on itself while sending, so it
// is not safe for concurrent use; it is wrapped so every send uses its own copy
// of the request (keeping any custom host, subuser or data residency setting).
func NewSendGridProvider(client SendGridClient) *SendGridProvider {
	if sgClient, ok := client.(*sendgrid.Client); ok && sgClient != nil {
		client = &sendGridRequestClient{request: sgClient.Request}
	}
	return &SendGridProvider{client: client}
}

// Send sends the email through SendGrid; the result MessageID is the X-Message-Id
// response header and Response is the *rest.Response
func (p *SendGridProvider) Send(ctx context.Context, email *Email) (*SendResult, error) {
	env, err := parseEnvelope(email)
	if err != nil {
		return nil, err
	}

	var attachments []attachmentData
	if attachments, err = readAttachments(email); err != nil {
		return nil, err
	}

	message := sgmail.NewV3Mail()
	message.SetFrom(sgmail.NewEmail(env.from.Name, env.from.Address))
	message.Subject = email.Subject

	// Build a single personalization holding all recipients
	personalization := sgmail.NewPersonalization()
	personalization.AddTos(sendGridEmails(env.to)...)
	personalization.AddCCs(sendGridEmails(env.cc)...)
	personalization.AddBCCs(sendGridEmails(env.bcc)...)
	message.AddPersonalizations(personalization)

	// Add content (SendGrid requires the plain-text part before the HTML part)
	if len(email.PlainTextContent) > 0 {
		message.AddContent(sgmail.NewContent(mimeTypePlain, email.PlainTextContent))
	}
	if len(email.HTMLContent) > 0 {
		message.AddContent(sgmail.NewContent(mimeTypeHTML, email.HTMLContent))
	}

	if env.replyTo != nil {
		message.SetReplyTo(sgmail.NewEmail(env.replyTo.Name, env.replyTo.Address))
	}
	if len(email.Tags) > 0 {
		message.AddCategories(email.Tags...)
	}
	for _, key := range slices.Sorted(maps.Keys(email.Metadata)) {
		message.SetCustomArg(key, email.Metadata[key])
	}
	for _, h := range emailHeaders(email) {
		message.SetHeader(h.name, h.value)
	}
	if !email.SendAt.IsZero() {
		message.SetSendAt(int(email.SendAt.Unix()))
	}

	// Tracking is always explicit so the email flags are authoritative
	message.SetTrackingSettings(sgmail.NewTrackingSettings().
		SetClickTracking(sgmail.NewClickTrackingSetting().SetEnable(email.TrackClicks).SetEnableText(email.TrackClicks)).
		SetOpenTracking(sgmail.NewOpenTrackingSetting().SetEnable(email.TrackOpens)),
	)

	// Convert attachments to SendGrid format (inline attachments use a content id)
	for _, att := range attachments {
		sgAttachment := sgmail.NewAttachment().
			SetContent(base64.StdEncoding.EncodeToString(att.content)).
			SetType(att.contentType).
			SetFilename(att.name).
			SetDisposition("attachment")
		if att.inline() {
			sgAttachment.SetDisposition("inline").SetContentID(att.contentID)
		}
		message.AddAttachment(sgAttachment)
	}

	applyProviderOptions[SendGridOption](email.ProviderOptions, message)

	var resp *rest.Response
	if resp, err = p.client.SendWithContext(ctx, message); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSendGridError, err)
	}
	if resp == nil {
		return nil, fmt.Errorf("error from sendgrid: empty response: %w", ErrSendGridError)
	}

	// SendGrid returns a 2xx status code on success
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("error from sendgrid: status code %d, body: %s: %w", resp.StatusCode, resp.Body, ErrSendGridError)
	}

	return &SendResult{MessageID: http.Header(resp.Headers).Get(headerSendGridMessageID), Response: resp}, nil
}

// SupportsFeature reports whether SendGrid supports the feature
func (p *SendGridProvider) SupportsFeature(feature Feature) bool {
	return supportsFeature([]Feature{
		FeatureMetadata, FeatureSendAt, FeatureTags, FeatureTrackClicks, FeatureTrackOpens,
	}, feature)
}

// sendGridEmails converts addresses into SendGrid emails
func sendGridEmails(addrs []*mail.Address) []*sgmail.Email {
	emails := make([]*sgmail.Email, 0, len(addrs))
	for _, addr := range addrs {
		emails = append(emails, sgmail.NewEmail(addr.Name, addr.Address))
	}
	return emails
}

// sendGridRequestClient sends through SendGrid using a private copy of the
// request for every send (see NewSendGridProvider)
type sendGridRequestClient struct {
	request rest.Request
}

// SendWithContext sends the email using a copy of the configured request
func (c *sendGridRequestClient) SendWithContext(ctx context.Context, email *sgmail.SGMailV3) (*rest.Response, error) {
	request := c.request
	request.Headers = maps.Clone(c.request.Headers)
	request.QueryParams = maps.Clone(c.request.QueryParams)

	client := &sendgrid.Client{Request: request}
	return client.SendWithContext(ctx, email)
}
