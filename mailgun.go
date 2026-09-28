package gomail

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/mailgun/mailgun-go/v5"
	"github.com/mailgun/mailgun-go/v5/mtypes"
)

// MailgunClient is the Mailgun API used by MailgunProvider; *mailgun.Client satisfies it
type MailgunClient interface {
	Send(ctx context.Context, message mailgun.Message) (mtypes.SendMessageResponse, error)
}

// MailgunProvider sends email through Mailgun.
//
// Tags become Mailgun tags (at most 10 per email) and Metadata becomes user
// variables, which Mailgun returns in webhooks and events. Open and click
// tracking are always set explicitly from the email, so the domain defaults do
// not apply.
//
// Mailgun detects an attachment's content type from its file name, and uses
// the file name as the content id of an inline attachment, so inline
// attachments are sent with their ContentID as the file name (use a content id
// with an extension, ie: logo.png, so the image type is detected).
type MailgunProvider struct {
	client MailgunClient
	domain string
}

// NewMailgunProvider creates a Mailgun provider, ie: NewMailgunProvider(mailgun.NewMailgun(key), "mg.example.com").
//
// The domain is the Mailgun sending domain; when empty, the domain of each
// email's FromAddress is used. For the EU region, set the client's API base
// with client.SetAPIBase(mailgun.APIBaseEU).
func NewMailgunProvider(client MailgunClient, domain string) *MailgunProvider {
	return &MailgunProvider{client: client, domain: domain}
}

// Send sends the email through Mailgun; the result MessageID is the Mailgun
// message id (ie: <20260101.1@mg.example.com>) and Response is the mtypes.SendMessageResponse
func (p *MailgunProvider) Send(ctx context.Context, email *Email) (*SendResult, error) {
	env, err := parseEnvelope(email)
	if err != nil {
		return nil, err
	}

	var attachments []attachmentData
	if attachments, err = readAttachments(email); err != nil {
		return nil, err
	}

	var message *mailgun.PlainMessage
	if message, err = p.newMessage(email, env, attachments); err != nil {
		return nil, err
	}

	applyProviderOptions[MailgunOption](email.ProviderOptions, message)

	var resp mtypes.SendMessageResponse
	if resp, err = p.client.Send(ctx, message); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrMailgunError, err)
	}

	// Mailgun returns the id of the queued message on success
	if len(resp.ID) == 0 {
		return nil, fmt.Errorf("%w: empty message id", ErrMailgunError)
	}

	return &SendResult{MessageID: resp.ID, Response: resp}, nil
}

// SupportsFeature reports whether Mailgun supports the feature
func (p *MailgunProvider) SupportsFeature(feature Feature) bool {
	return supportsFeature([]Feature{
		FeatureMetadata, FeatureSendAt, FeatureTags, FeatureTrackClicks, FeatureTrackOpens,
	}, feature)
}

// newMessage maps the email onto a Mailgun message
func (p *MailgunProvider) newMessage(email *Email, env *envelope, attachments []attachmentData) (*mailgun.PlainMessage, error) {
	domain := p.domain
	if len(domain) == 0 {
		domain = addressDomain(env.from.Address)
	}

	message := mailgun.NewMessage(
		domain, formatAddress(&env.from), email.Subject, email.PlainTextContent, formatAddresses(env.to)...,
	)
	message.SetHTML(email.HTMLContent)
	for _, addr := range env.cc {
		message.AddCC(formatAddress(addr))
	}
	for _, addr := range env.bcc {
		message.AddBCC(formatAddress(addr))
	}

	// Mailgun sets Reply-To through the headers
	if env.replyTo != nil {
		message.SetReplyTo(formatAddress(env.replyTo))
	}
	for _, h := range emailHeaders(email) {
		message.AddHeader(h.name, h.value)
	}

	for _, tag := range mailgunTags(email.Tags) {
		if err := message.AddTag(tag); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrMailgunError, err)
		}
	}
	for key, value := range email.Metadata {
		if err := message.AddVariable(key, value); err != nil {
			return nil, fmt.Errorf("%w: metadata %q: %w", ErrMailgunError, key, err)
		}
	}
	if !email.SendAt.IsZero() {
		message.SetDeliveryTime(email.SendAt)
	}

	// Tracking is always explicit so the email flags are authoritative
	message.SetTrackingClicks(email.TrackClicks)
	message.SetTrackingOpens(email.TrackOpens)

	// Regular attachments, and inline images (Mailgun uses the file name as the content id)
	for _, att := range attachments {
		if att.inline() {
			message.AddReaderInline(att.contentID, io.NopCloser(bytes.NewReader(att.content)))
			continue
		}
		message.AddBufferAttachment(att.name, att.content)
	}

	return message, nil
}

// mailgunTags returns the tags without empty or duplicate values (Mailgun
// rejects an empty tag and compares tags case-insensitively)
func mailgunTags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}

	list := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		key := strings.ToLower(tag)
		if _, ok := seen[key]; ok || len(tag) == 0 {
			continue
		}
		seen[key] = struct{}{}
		list = append(list, tag)
	}
	return list
}
