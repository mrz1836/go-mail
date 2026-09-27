package gomail

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"maps"
	"net/mail"

	"github.com/mattbaird/gochimp"
)

// Mandrill send statuses that mean the message was accepted
const (
	mandrillStatusQueued    = "queued"
	mandrillStatusScheduled = "scheduled"
	mandrillStatusSent      = "sent"
)

// MandrillClient is the Mandrill API used by MandrillProvider; *gochimp.MandrillAPI satisfies it
type MandrillClient interface {
	MessageSendWithOptions(message gochimp.Message, opts gochimp.MessageSendOptions) ([]gochimp.SendResponse, error)
}

// MandrillProvider sends email through Mandrill (Mailchimp Transactional).
//
// Messages are sent synchronously so rejected recipients are reported as
// errors. Recipients are not preserved: each To recipient only sees their own
// address (set PreserveRecipients with a MandrillOption to change this).
type MandrillProvider struct {
	client MandrillClient
}

// NewMandrillProvider creates a Mandrill provider
func NewMandrillProvider(client MandrillClient) *MandrillProvider {
	return &MandrillProvider{client: client}
}

// Send sends the email through Mandrill; the result MessageID is the id of the
// first recipient's message and Response holds every []gochimp.SendResponse
func (p *MandrillProvider) Send(ctx context.Context, email *Email) (*SendResult, error) {
	env, err := parseEnvelope(email)
	if err != nil {
		return nil, err
	}

	var attachments []attachmentData
	if attachments, err = readAttachments(email); err != nil {
		return nil, err
	}

	message := gochimp.Message{
		AutoText:           email.AutoText,
		FromEmail:          env.from.Address,
		FromName:           env.from.Name,
		Headers:            headerMap(email),
		Html:               email.HTMLContent,
		Important:          email.Important,
		Metadata:           maps.Clone(email.Metadata),
		PreserveRecipients: false,
		SigningDomain:      addressDomain(env.from.Address),
		Subject:            email.Subject,
		Tags:               email.Tags,
		Text:               email.PlainTextContent,
		TrackClicks:        email.TrackClicks,
		TrackOpens:         email.TrackOpens,
		ViewContentLink:    email.ViewContentLink,
	}

	// Mandrill sets Reply-To through the headers
	if env.replyTo != nil {
		message.AddHeader("Reply-To", formatAddress(env.replyTo))
	}

	// Convert recipients (to, cc, bcc) into a single list
	message.To = make([]gochimp.Recipient, 0, len(env.to)+len(env.cc)+len(env.bcc))
	message.To = appendMandrillRecipients(message.To, env.to, "to")
	message.To = appendMandrillRecipients(message.To, env.cc, "cc")
	message.To = appendMandrillRecipients(message.To, env.bcc, "bcc")

	// Regular attachments, and inline images (referenced by content id)
	for _, att := range attachments {
		mandrillAttachment := gochimp.Attachment{
			Content: base64.StdEncoding.EncodeToString(att.content),
			Name:    att.name,
			Type:    att.contentType,
		}
		if att.inline() {
			mandrillAttachment.Name = att.contentID
			message.Images = append(message.Images, mandrillAttachment)
			continue
		}
		message.Attachments = append(message.Attachments, mandrillAttachment)
	}

	applyProviderOptions[MandrillOption](email.ProviderOptions, &message)

	options := gochimp.MessageSendOptions{Async: false}
	if !email.SendAt.IsZero() {
		sendAt := email.SendAt
		options.SendAt = &sendAt
	}

	var responses []gochimp.SendResponse
	if responses, err = p.send(ctx, message, options); err != nil {
		return nil, err
	}

	// Every recipient must have been accepted
	var errs []error
	for _, response := range responses {
		switch response.Status {
		case mandrillStatusSent, mandrillStatusQueued, mandrillStatusScheduled:
		default:
			errs = append(errs, fmt.Errorf(
				"message to %s status was %s and not sent - given reason: %s: %w",
				response.Email, response.Status, response.RejectedReason, ErrMessageNotSent,
			))
		}
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	result := &SendResult{Response: responses}
	if len(responses) > 0 {
		result.MessageID = responses[0].Id
	}
	return result, nil
}

// SupportsFeature reports whether Mandrill supports the feature
func (p *MandrillProvider) SupportsFeature(feature Feature) bool {
	return supportsFeature([]Feature{
		FeatureAutoText, FeatureMetadata, FeatureSendAt, FeatureTags,
		FeatureTrackClicks, FeatureTrackOpens, FeatureViewContentLink,
	}, feature)
}

// send calls Mandrill, returning early when the context ends (the Mandrill
// client has no context support; its own timeout bounds the request)
func (p *MandrillProvider) send(ctx context.Context, message gochimp.Message, options gochimp.MessageSendOptions) ([]gochimp.SendResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	type sendOutcome struct {
		err       error
		responses []gochimp.SendResponse
	}

	// Buffered so the goroutine never blocks, even after the caller has returned
	done := make(chan sendOutcome, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- sendOutcome{err: fmt.Errorf("%w: mandrill client panicked: %v", ErrMessageNotSent, r)}
			}
		}()
		responses, err := p.client.MessageSendWithOptions(message, options)
		done <- sendOutcome{err: err, responses: responses}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case outcome := <-done:
		return outcome.responses, outcome.err
	}
}

// appendMandrillRecipients converts addresses into Mandrill recipients of the
// given kind (to/cc/bcc) and appends them to dst
func appendMandrillRecipients(dst []gochimp.Recipient, addrs []*mail.Address, kind string) []gochimp.Recipient {
	for _, addr := range addrs {
		dst = append(dst, gochimp.Recipient{
			Email: addr.Address,
			Name:  addr.Name,
			Type:  kind,
		})
	}
	return dst
}
