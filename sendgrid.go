package gomail

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/sendgrid/rest"
	sendgrid "github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

// sendGridInterface is an interface for SendGrid/mocking
type sendGridInterface interface {
	SendWithContext(ctx context.Context, email *mail.SGMailV3) (*rest.Response, error)
}

// newSendGridClient creates a new SendGrid client; *sendgrid.Client already
// satisfies sendGridInterface, so no wrapper type is needed
func newSendGridClient(apiKey string) sendGridInterface {
	return sendgrid.NewSendClient(apiKey)
}

// sendViaSendGrid sends an email using the SendGrid service
func sendViaSendGrid(ctx context.Context, client sendGridInterface, email *Email) (err error) {
	// Create the SendGrid message with the sender and subject
	message := mail.NewV3Mail()
	message.SetFrom(mail.NewEmail(email.FromName, email.FromAddress))
	message.Subject = email.Subject

	// Build a single personalization holding all recipients
	personalization := mail.NewPersonalization()
	for _, recipient := range email.Recipients {
		personalization.AddTos(mail.NewEmail("", recipient))
	}
	for _, recipient := range email.RecipientsCc {
		personalization.AddCCs(mail.NewEmail("", recipient))
	}
	for _, recipient := range email.RecipientsBcc {
		personalization.AddBCCs(mail.NewEmail("", recipient))
	}
	message.AddPersonalizations(personalization)

	// Add content (SendGrid requires the plain-text part before the HTML part)
	if len(email.PlainTextContent) > 0 {
		message.AddContent(mail.NewContent("text/plain", email.PlainTextContent))
	}
	if len(email.HTMLContent) > 0 {
		message.AddContent(mail.NewContent("text/html", email.HTMLContent))
	}

	// Add a custom reply to address
	if len(email.ReplyToAddress) > 0 {
		message.SetReplyTo(mail.NewEmail("", email.ReplyToAddress))
	}

	// Add tags as SendGrid categories
	if len(email.Tags) > 0 {
		message.AddCategories(email.Tags...)
	}

	// Add importance headers
	if email.Important {
		message.SetHeader(headerXPriority, headerXPriorityValue)
		message.SetHeader(headerXMSMailPriority, headerHighValue)
		message.SetHeader(headerImportance, headerHighValue)
	}

	// SendGrid supports open/click tracking natively (no "unsupported" warning)
	if email.TrackClicks || email.TrackOpens {
		message.SetTrackingSettings(mail.NewTrackingSettings().
			SetClickTracking(mail.NewClickTrackingSetting().SetEnable(email.TrackClicks).SetEnableText(email.TrackClicks)).
			SetOpenTracking(mail.NewOpenTrackingSetting().SetEnable(email.TrackOpens)),
		)
	}

	// Warn about features that are set but not available
	if email.AutoText {
		log.Printf("warning: auto text is enabled, but SendGrid does not offer this feature")
	}

	// Convert attachments to SendGrid format
	for _, attachment := range email.Attachments {

		// Encode the attachment contents as base64
		var encoded string
		if encoded, err = encodeAttachmentBase64(attachment.FileReader); err != nil {
			return err
		}

		// Add to the message
		message.AddAttachment(mail.NewAttachment().
			SetContent(encoded).
			SetType(attachment.FileType).
			SetFilename(attachment.FileName).
			SetDisposition("attachment"),
		)
	}

	// Send the message and check the response
	var resp *rest.Response
	if resp, err = client.SendWithContext(ctx, message); err != nil {
		return err
	}

	// SendGrid returns a 2xx status code on success
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		err = fmt.Errorf("error from sendgrid: status code %d, body: %s: %w", resp.StatusCode, resp.Body, ErrSendGridError)
	}

	return err
}
