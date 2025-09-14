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
	SendRawEmail(raw []byte) (string, error)
}

// awsSesSdkV2Client wraps the AWS SDK v2 SES client to implement awsSesInterface
type awsSesSdkV2Client struct {
	client *ses.Client
}

// SendRawEmail implements the awsSesInterface using AWS SDK v2
func (c *awsSesSdkV2Client) SendRawEmail(raw []byte) (string, error) {
	input := &ses.SendRawEmailInput{
		RawMessage: &types.RawMessage{
			Data: raw,
		},
	}

	result, err := c.client.SendRawEmail(context.TODO(), input)
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

	responseStr := fmt.Sprintf(`<SendRawEmailResponse xmlns="http://ses.amazonaws.com/doc/2010-12-01/">
  <SendRawEmailResult>
    <MessageId>%s</MessageId>
  </SendRawEmailResult>
  <ResponseMetadata>
    <RequestId>%s</RequestId>
  </ResponseMetadata>
</SendRawEmailResponse>`, *result.MessageId, requestID)

	return responseStr, nil
}

// sendViaAwsSes sends an email using the AWS SES service
func sendViaAwsSes(client awsSesInterface, email *Email) (err error) {
	// Create new mail message
	mail := mailyak.New("", nil)

	// Add the "to" recipients
	mail.To(email.Recipients...)

	// Add the "cc" recipients
	if len(email.RecipientsCc) > 0 {
		mail.Cc(email.RecipientsCc...)
	}

	// Add the "bcc" recipients
	if len(email.RecipientsBcc) > 0 {
		mail.WriteBccHeader(true)
		mail.Bcc(email.RecipientsBcc...)
	}

	// Add the basics
	mail.From(email.FromAddress)
	mail.FromName(email.FromName)
	mail.Subject(email.Subject)

	// Add a custom reply to address
	if len(email.ReplyToAddress) > 0 {
		mail.ReplyTo(email.ReplyToAddress)
	}

	// Add plain text
	if len(email.PlainTextContent) > 0 {
		mail.Plain().Set(email.PlainTextContent)
	}

	// Add html
	if len(email.HTMLContent) > 0 {
		mail.HTML().Set(email.HTMLContent)
	}

	// Add any attachments
	if len(email.Attachments) > 0 {
		for _, att := range email.Attachments {
			mail.Attach(att.FileName, att.FileReader)
		}
	}

	// Add importance?
	if email.Important {
		mail.AddHeader("X-Priority", "1 (Highest)")
		mail.AddHeader("X-MSMail-Priority", "High")
		mail.AddHeader("Importance", "High")
	}

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
	awsResponse, err = client.SendRawEmail(buf.Bytes())
	if err != nil {
		return err
	} else if !strings.Contains(awsResponse, "SendRawEmailResult") {
		err = fmt.Errorf("aws ses did not return expected valid response: %s", awsResponse)
	}

	return
}
