package gomail

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"github.com/mattbaird/gochimp"
)

// mandrillInterface is an interface for Mandrill/mocking
type mandrillInterface interface {
	MessageSend(message gochimp.Message, async bool) ([]gochimp.SendResponse, error)
}

// sendViaMandrill sends an email using the Mandrill service
// Mandrill uses the word Message for their email
func sendViaMandrill(client mandrillInterface, email *Email, async bool) (err error) {
	// Get the signing domain from the FromAddress
	emailParts := strings.Split(email.FromAddress, "@")
	if len(emailParts) <= 1 || emailParts[1] == "" {
		err = fmt.Errorf("invalid FromAddress, domain not found using: %s", email.FromAddress)
		return
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

	// Convert recipients
	for _, recipient := range email.Recipients {
		emailRecipient := gochimp.Recipient{
			Email: recipient,
			Type:  "to",
		}
		message.To = append(message.To, emailRecipient)
	}

	// Convert any BCC recipients
	for _, recipient := range email.RecipientsBcc {
		emailRecipient := gochimp.Recipient{
			Email: recipient,
			Type:  "bcc",
		}
		message.To = append(message.To, emailRecipient)
	}

	// Convert any CC recipients
	for _, recipient := range email.RecipientsCc {
		emailRecipient := gochimp.Recipient{
			Email: recipient,
			Type:  "cc",
		}
		message.To = append(message.To, emailRecipient)
	}

	// Convert attachments to Mandrill format
	for _, attachment := range email.Attachments {

		// Create the Mandrill attachment
		mandrillAttachment := &gochimp.Attachment{
			Name: attachment.FileName,
			Type: attachment.FileType,
		}

		// Read all content from the attachment
		reader := bufio.NewReader(attachment.FileReader)
		var content []byte
		if content, err = io.ReadAll(reader); err != nil {
			return
		}

		// Encode as base64
		mandrillAttachment.Content = base64.StdEncoding.EncodeToString(content)

		// Add to the email
		message.Attachments = append(message.Attachments, *mandrillAttachment)
	}

	// Send the email
	var sendResponse []gochimp.SendResponse
	if sendResponse, err = client.MessageSend(message, async); err != nil {
		return
	}

	// Check the response of each email that was sent
	if len(sendResponse) > 0 {
		for _, response := range sendResponse {
			if response.Status != "sent" && response.Status != "queued" && response.Status != "scheduled" {
				err = fmt.Errorf("message status was %s and not sent - given reason: %s", response.Status, response.RejectedReason)
			}
		}
	}
	return
}
