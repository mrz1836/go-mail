package gomail

import (
	"fmt"
	"strings"

	"github.com/mattbaird/gochimp"
)

// mandrillInterface is an interface for Mandrill/mocking
type mandrillInterface interface {
	MessageSend(message gochimp.Message, async bool) ([]gochimp.SendResponse, error)
}

// appendRecipients converts addresses into Mandrill recipients of the given kind
// (to/cc/bcc) and appends them to dst
func appendRecipients(dst []gochimp.Recipient, addrs []string, kind string) []gochimp.Recipient {
	for _, addr := range addrs {
		dst = append(dst, gochimp.Recipient{
			Email: addr,
			Type:  kind,
		})
	}
	return dst
}

// sendViaMandrill sends an email using the Mandrill service
// Mandrill uses the word Message for their email
func sendViaMandrill(client mandrillInterface, email *Email, async bool) (err error) {
	// Get the signing domain from the FromAddress
	emailParts := strings.Split(email.FromAddress, "@")
	if len(emailParts) <= 1 || emailParts[1] == "" {
		err = fmt.Errorf("invalid FromAddress, domain not found using: %s: %w", email.FromAddress, ErrInvalidFromAddress)
		return err
	}

	// Create the Mandrill email
	message := gochimp.Message{
		AutoText:           email.AutoText,
		FromEmail:          email.FromAddress,
		FromName:           email.FromName,
		Html:               email.HTMLContent,
		Important:          email.Important,
		PreserveRecipients: false,
		SigningDomain:      emailParts[1],
		Subject:            email.Subject,
		Tags:               email.Tags,
		Text:               email.PlainTextContent,
		TrackClicks:        email.TrackClicks,
		TrackOpens:         email.TrackOpens,
		ViewContentLink:    email.ViewContentLink,
	}

	// Convert recipients (to, bcc, cc) into a single preallocated list
	message.To = make([]gochimp.Recipient, 0, len(email.Recipients)+len(email.RecipientsBcc)+len(email.RecipientsCc))
	message.To = appendRecipients(message.To, email.Recipients, "to")
	message.To = appendRecipients(message.To, email.RecipientsBcc, "bcc")
	message.To = appendRecipients(message.To, email.RecipientsCc, "cc")

	// Convert attachments to Mandrill format
	message.Attachments = make([]gochimp.Attachment, 0, len(email.Attachments))
	for _, attachment := range email.Attachments {

		// Create the Mandrill attachment
		mandrillAttachment := gochimp.Attachment{
			Name: attachment.FileName,
			Type: attachment.FileType,
		}

		// Encode the attachment contents as base64
		if mandrillAttachment.Content, err = encodeAttachmentBase64(attachment.FileReader); err != nil {
			return err
		}

		// Add to the email
		message.Attachments = append(message.Attachments, mandrillAttachment)
	}

	// Send the email
	var sendResponse []gochimp.SendResponse
	if sendResponse, err = client.MessageSend(message, async); err != nil {
		return err
	}

	// Check the response of each email that was sent
	if len(sendResponse) > 0 {
		for _, response := range sendResponse {
			if response.Status != "sent" && response.Status != "queued" && response.Status != "scheduled" {
				err = fmt.Errorf("message status was %s and not sent - given reason: %s: %w", response.Status, response.RejectedReason, ErrMessageNotSent)
			}
		}
	}
	return err
}
