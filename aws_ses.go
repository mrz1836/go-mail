package gomail

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
)

// SESClient is the AWS SES API used by SESProvider; *ses.Client satisfies it
type SESClient interface {
	SendRawEmail(ctx context.Context, params *ses.SendRawEmailInput, optFns ...func(*ses.Options)) (*ses.SendRawEmailOutput, error)
}

// SESProvider sends email through AWS SES (SendRawEmail).
//
// Tags and Metadata become SES message tags. Open/click tracking is configured
// on an SES configuration set rather than per email.
type SESProvider struct {
	client           SESClient
	now              func() time.Time
	configurationSet string
}

// NewSESProvider creates an AWS SES provider; configurationSet is optional
// (use it for event publishing, ie: open/click tracking)
func NewSESProvider(client SESClient, configurationSet string) *SESProvider {
	return &SESProvider{client: client, configurationSet: configurationSet, now: time.Now}
}

// Send sends the email through AWS SES; the result MessageID is the SES message id
func (p *SESProvider) Send(ctx context.Context, email *Email) (*SendResult, error) {
	env, err := parseEnvelope(email)
	if err != nil {
		return nil, err
	}

	var attachments []attachmentData
	if attachments, err = readAttachments(email); err != nil {
		return nil, err
	}

	// SES assigns its own Message-ID, so none is generated here
	var raw []byte
	if raw, err = buildMIME(email, env, attachments, "", p.now()); err != nil {
		return nil, err
	}

	// Recipients are passed as Destinations so the Bcc header is never needed
	input := &ses.SendRawEmailInput{
		Destinations: env.allRecipients(),
		RawMessage:   &types.RawMessage{Data: raw},
	}
	if len(p.configurationSet) > 0 {
		input.ConfigurationSetName = aws.String(p.configurationSet)
	}
	for _, tag := range nameValueTags(email.Tags, email.Metadata) {
		input.Tags = append(input.Tags, types.MessageTag{Name: aws.String(tag.name), Value: aws.String(tag.value)})
	}
	applyProviderOptions[SESOption](email.ProviderOptions, input)

	var output *ses.SendRawEmailOutput
	if output, err = p.client.SendRawEmail(ctx, input); err != nil {
		return nil, err
	}
	if output == nil || output.MessageId == nil || len(*output.MessageId) == 0 {
		return nil, ErrInvalidAWSResponse
	}

	return &SendResult{MessageID: *output.MessageId, Response: output}, nil
}

// SupportsFeature reports whether AWS SES supports the feature
func (p *SESProvider) SupportsFeature(feature Feature) bool {
	return supportsFeature([]Feature{FeatureMetadata, FeatureTags}, feature)
}
