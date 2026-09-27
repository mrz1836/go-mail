package gomail

import (
	"context"
	"encoding/base64"
	"fmt"
	"maps"
	"strings"

	"github.com/mrz1836/postmark"
)

// Postmark link tracking settings
const (
	postmarkTrackLinksAll  = "HtmlAndText"
	postmarkTrackLinksNone = "None"
)

// PostmarkClient is the Postmark API used by PostmarkProvider; *postmark.Client satisfies it
type PostmarkClient interface {
	SendEmail(ctx context.Context, email postmark.Email) (postmark.EmailResponse, error)
}

// PostmarkProvider sends email through Postmark.
//
// Postmark supports a single tag per email, so multiple Tags are joined with a
// comma into one tag. Use a PostmarkOption to set the MessageStream.
type PostmarkProvider struct {
	client PostmarkClient
}

// NewPostmarkProvider creates a Postmark provider
func NewPostmarkProvider(client PostmarkClient) *PostmarkProvider {
	return &PostmarkProvider{client: client}
}

// Send sends the email through Postmark; the result MessageID is the Postmark message id
func (p *PostmarkProvider) Send(ctx context.Context, email *Email) (*SendResult, error) {
	env, err := parseEnvelope(email)
	if err != nil {
		return nil, err
	}

	var attachments []attachmentData
	if attachments, err = readAttachments(email); err != nil {
		return nil, err
	}

	postmarkEmail := postmark.Email{
		Bcc:        strings.Join(formatAddresses(env.bcc), ","),
		Cc:         strings.Join(formatAddresses(env.cc), ","),
		From:       formatAddress(&env.from),
		HTMLBody:   email.HTMLContent,
		Metadata:   maps.Clone(email.Metadata),
		Subject:    email.Subject,
		Tag:        strings.Join(email.Tags, ","),
		TextBody:   email.PlainTextContent,
		To:         strings.Join(formatAddresses(env.to), ","),
		TrackLinks: postmarkTrackLinksNone,
		TrackOpens: email.TrackOpens,
	}
	if email.TrackClicks {
		postmarkEmail.TrackLinks = postmarkTrackLinksAll
	}
	if env.replyTo != nil {
		postmarkEmail.ReplyTo = formatAddress(env.replyTo)
	}
	for _, h := range emailHeaders(email) {
		postmarkEmail.Headers = append(postmarkEmail.Headers, postmark.Header{Name: h.name, Value: h.value})
	}

	// Convert attachments to Postmark format (inline attachments use a cid: content id)
	postmarkEmail.Attachments = make([]postmark.Attachment, 0, len(attachments))
	for _, att := range attachments {
		postmarkAttachment := postmark.Attachment{
			Content:     base64.StdEncoding.EncodeToString(att.content),
			ContentType: att.contentType,
			Name:        att.name,
		}
		if att.inline() {
			postmarkAttachment.ContentID = "cid:" + att.contentID
		}
		postmarkEmail.Attachments = append(postmarkEmail.Attachments, postmarkAttachment)
	}

	applyProviderOptions[PostmarkOption](email.ProviderOptions, &postmarkEmail)

	var resp postmark.EmailResponse
	if resp, err = p.client.SendEmail(ctx, postmarkEmail); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrPostmarkError, err)
	}
	if resp.ErrorCode > 0 {
		return nil, fmt.Errorf("error from postmark: %s error code: %d: %w", resp.Message, resp.ErrorCode, ErrPostmarkError)
	}

	return &SendResult{MessageID: resp.MessageID, Response: resp}, nil
}

// SupportsFeature reports whether Postmark supports the feature
func (p *PostmarkProvider) SupportsFeature(feature Feature) bool {
	return supportsFeature([]Feature{FeatureMetadata, FeatureTags, FeatureTrackClicks, FeatureTrackOpens}, feature)
}
