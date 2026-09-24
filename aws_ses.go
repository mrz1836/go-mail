package gomail

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
	"github.com/domodwyer/mailyak"
)

// awsSesInterface is an interface for ses/mocking
type awsSesInterface interface {
	SendRawEmail(ctx context.Context, raw []byte) (string, error)
}

// sesSendRawEmailAPI is the narrow slice of the AWS SDK v2 SES client used by
// awsSesSdkV2Client; *ses.Client satisfies it and it enables mocking in tests
type sesSendRawEmailAPI interface {
	SendRawEmail(ctx context.Context, params *ses.SendRawEmailInput, optFns ...func(*ses.Options)) (*ses.SendRawEmailOutput, error)
}

// awsSesSdkV2Client wraps the AWS SDK v2 SES client to implement awsSesInterface
type awsSesSdkV2Client struct {
	client sesSendRawEmailAPI
}

// SendRawEmail implements the awsSesInterface using AWS SDK v2
func (c *awsSesSdkV2Client) SendRawEmail(ctx context.Context, raw []byte) (string, error) {
	input := &ses.SendRawEmailInput{
		RawMessage: &types.RawMessage{
			Data: raw,
		},
	}

	result, err := c.client.SendRawEmail(ctx, input)
	if err != nil {
		return "", err
	}

	// Format response similar to what was expected from v1 SDK
	requestID := "unknown"
	if result.ResultMetadata.Get("RequestId") != nil {
		if id, ok := result.ResultMetadata.Get("RequestId").(string); ok {
			requestID = id
		}
	}

	// Guard against a nil MessageId on an otherwise successful response
	messageID := ""
	if result.MessageId != nil {
		messageID = *result.MessageId
	}

	responseStr := fmt.Sprintf(`<SendRawEmailResponse xmlns="http://ses.amazonaws.com/doc/2010-12-01/">
  <SendRawEmailResult>
    <MessageId>%s</MessageId>
  </SendRawEmailResult>
  <ResponseMetadata>
    <RequestId>%s</RequestId>
  </ResponseMetadata>
</SendRawEmailResponse>`, messageID, requestID)

	return responseStr, nil
}

// sendViaAwsSes sends an email using the AWS SES service
func sendViaAwsSes(ctx context.Context, client awsSesInterface, email *Email) (err error) {
	// Create new mail message
	mail := mailyak.New("", nil)

	// Populate the shared mailyak message fields
	populateMailyakMessage(mail, email)

	// Warn about features that are set but not available
	if email.TrackClicks {
		log.Printf("warning: track clicks is enabled, but AWS SES does not offer this feature")
	}
	if email.TrackOpens {
		log.Printf("warning: track opens is enabled, but AWS SES does not offer this feature")
	}
	if email.AutoText {
		log.Printf("warning: auto text is enabled, but AWS SES does not offer this feature")
	}

	// Create the email buffer and pass to the ses service
	var buf *bytes.Buffer
	if buf, err = mail.MimeBuf(); err != nil {
		return err
	}

	// Send the message post and check the response
	var awsResponse string
	awsResponse, err = client.SendRawEmail(ctx, buf.Bytes())
	if err != nil {
		return err
	} else if !strings.Contains(awsResponse, "SendRawEmailResult") {
		err = fmt.Errorf("aws ses did not return expected valid response: %s: %w", awsResponse, ErrInvalidAWSResponse)
	}

	return err
}
