package gomail

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/mrz1836/postmark"
)

// postmarkInterface is an interface for Postmark/mocking
type postmarkInterface interface {
	SendEmail(ctx context.Context, email postmark.Email) (postmark.EmailResponse, error)
}

// sendViaPostmark sends an email using the Postmark service
func sendViaPostmark(ctx context.Context, client postmarkInterface, email *Email) (err error) {
	// Create the email struct
	postmarkEmail := postmark.Email{
		From:       email.FromAddress,
		HTMLBody:   email.HTMLContent,
		ReplyTo:    email.ReplyToAddress,
		Subject:    email.Subject,
		TextBody:   email.PlainTextContent,
		TrackOpens: email.TrackOpens,
		TrackLinks: "None",
	}

	// Set the link tracking
	if email.TrackClicks {
		postmarkEmail.TrackLinks = "HtmlAndText"
	}

	// Warn about features that are set but not available
	if email.AutoText {
		log.Printf("warning: auto text is enabled, but Postmark does not offer this feature")
	}

	// Set the "from" name if given (RFC 5322: "Name <address>")
	if len(email.FromName) > 0 {
		postmarkEmail.From = email.FromName + " <" + email.FromAddress + ">"
	}

	// Convert recipients to comma separated
	postmarkEmail.To = strings.Join(email.Recipients, ",")

	// Convert tags to comma separated
	postmarkEmail.Tag = strings.Join(email.Tags, ",")

	// CC addresses
	if len(email.RecipientsCc) > 0 {
		postmarkEmail.Cc = strings.Join(email.RecipientsCc, ",")
	}

	// BCC addresses
	if len(email.RecipientsBcc) > 0 {
		postmarkEmail.Bcc = strings.Join(email.RecipientsBcc, ",")
	}

	// Convert attachments to Postmark format
	postmarkEmail.Attachments = make([]postmark.Attachment, 0, len(email.Attachments))
	for _, attachment := range email.Attachments {

		// Create the postmark attachment
		postmarkAttachment := postmark.Attachment{
			ContentType: attachment.FileType,
			Name:        attachment.FileName,
		}

		// Encode the attachment contents as base64
		if postmarkAttachment.Content, err = encodeAttachmentBase64(attachment.FileReader); err != nil {
			return err
		}

		// Add to the email
		postmarkEmail.Attachments = append(postmarkEmail.Attachments, postmarkAttachment)
	}

	// Add importance
	if email.Important {
		postmarkEmail.Headers = append(
			postmarkEmail.Headers,
			postmark.Header{Name: headerXPriority, Value: headerXPriorityValue},
			postmark.Header{Name: headerXMSMailPriority, Value: headerHighValue},
			postmark.Header{Name: headerImportance, Value: headerHighValue},
		)
	}

	// Send the email
	var resp postmark.EmailResponse
	if resp, err = client.SendEmail(ctx, postmarkEmail); err != nil {
		return err
	}

	// Check the response from Postmark
	if resp.ErrorCode > 0 {
		err = fmt.Errorf("error from postmark: %s error code: %d: %w", resp.Message, resp.ErrorCode, ErrPostmarkError)
	}

	return err
}
