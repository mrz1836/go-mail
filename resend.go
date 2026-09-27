package gomail

import (
	"context"
	"fmt"
	"time"

	"github.com/resend/resend-go/v4"
)

// ResendClient is the Resend API used by ResendProvider; the Emails service of
// a *resend.Client (*resend.EmailsSvcImpl) satisfies it
type ResendClient interface {
	SendWithOptions(ctx context.Context, params *resend.SendEmailRequest, options *resend.SendEmailOptions) (*resend.SendEmailResponse, error)
}

// ResendProvider sends email through Resend.
//
// Tags and Metadata become Resend tags (sanitized to the characters Resend
// allows). Open/click tracking is configured per domain in Resend, not per email.
type ResendProvider struct {
	client ResendClient
}

// NewResendProvider creates a Resend provider, ie: NewResendProvider(resend.NewClient(key).Emails)
func NewResendProvider(client ResendClient) *ResendProvider {
	return &ResendProvider{client: client}
}

// Send sends the email through Resend; the result MessageID is the Resend email id
func (p *ResendProvider) Send(ctx context.Context, email *Email) (*SendResult, error) {
	env, err := parseEnvelope(email)
	if err != nil {
		return nil, err
	}

	var attachments []attachmentData
	if attachments, err = readAttachments(email); err != nil {
		return nil, err
	}

	request := &resend.SendEmailRequest{
		Bcc:     formatAddresses(env.bcc),
		Cc:      formatAddresses(env.cc),
		From:    formatAddress(&env.from),
		Headers: headerMap(email),
		Html:    email.HTMLContent,
		Subject: email.Subject,
		Tags:    resendTags(email.Tags, email.Metadata),
		Text:    email.PlainTextContent,
		To:      formatAddresses(env.to),
	}
	if env.replyTo != nil {
		request.ReplyTo = formatAddress(env.replyTo)
	}
	if !email.SendAt.IsZero() {
		request.ScheduledAt = email.SendAt.UTC().Format(time.RFC3339)
	}

	// Convert attachments to Resend format (the SDK encodes the raw bytes itself)
	for _, att := range attachments {
		request.Attachments = append(request.Attachments, &resend.Attachment{
			Content:     att.content,
			ContentId:   att.contentID,
			ContentType: att.contentType,
			Filename:    att.name,
		})
	}

	applyProviderOptions[ResendOption](email.ProviderOptions, request)

	var resp *resend.SendEmailResponse
	if resp, err = p.client.SendWithOptions(ctx, request, &resend.SendEmailOptions{IdempotencyKey: email.IdempotencyKey}); err != nil {
		return nil, fmt.Errorf("error from resend: %w: %w", ErrResendError, err)
	}

	// Resend returns the id of the created email on success
	if resp == nil || len(resp.Id) == 0 {
		return nil, fmt.Errorf("error from resend: empty message id: %w", ErrResendError)
	}

	return &SendResult{MessageID: resp.Id, Response: resp}, nil
}

// SupportsFeature reports whether Resend supports the feature (Resend builds
// the plain-text part from the HTML natively, so auto text is supported)
func (p *ResendProvider) SupportsFeature(feature Feature) bool {
	return supportsFeature([]Feature{
		FeatureAutoText, FeatureIdempotencyKey, FeatureMetadata, FeatureSendAt, FeatureTags,
	}, feature)
}

// resendTags converts go-mail tags and metadata into Resend tags
func resendTags(tags []string, metadata map[string]string) []resend.Tag {
	pairs := nameValueTags(tags, metadata)
	if len(pairs) == 0 {
		return nil
	}

	list := make([]resend.Tag, 0, len(pairs))
	for _, pair := range pairs {
		list = append(list, resend.Tag{Name: pair.name, Value: pair.value})
	}
	return list
}
