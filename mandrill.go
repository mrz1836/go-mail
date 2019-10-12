package gomail

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/mattbaird/gochimp"
)

// sendWithMandrill sends an email using the Mandrill service
func (m *MailService) sendWithMandrill(email *Email) (err error) {

	// Get the signing domain from the from address
	sign := strings.Split(email.FromAddress, "@")
	var signDomain string
	if len(sign) <= 1 {
		signDomain = m.FromDomain
	} else {
		signDomain = sign[1]
	}

	// Create the Mandrill Email
	message := gochimp.Message{
		FromEmail:          email.FromAddress,
		FromName:           email.FromName,
		Html:               email.HTMLContent,
		PreserveRecipients: false,
		SigningDomain:      signDomain,
		Subject:            email.Subject,
		Tags:               email.Tags,
		Text:               email.PlainTextContent,
		Important:          email.Important,
		TrackOpens:         email.TrackOpens,
		TrackClicks:        email.TrackClicks,
		ViewContentLink:    email.ViewContentLink,
		AutoText:           email.AutoText,
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
	if len(email.RecipientsBcc) > 0 {
		for _, recipient := range email.RecipientsBcc {
			emailRecipient := gochimp.Recipient{
				Email: recipient,
				Type:  "bcc",
			}
			message.To = append(message.To, emailRecipient)
		}
	}

	// Convert any BCC recipients
	if len(email.RecipientsCc) > 0 {
		for _, recipient := range email.RecipientsCc {
			emailRecipient := gochimp.Recipient{
				Email: recipient,
				Type:  "cc",
			}
			message.To = append(message.To, emailRecipient)
		}
	}

	// Convert attachments to Mandrill format
	if len(email.Attachments) > 0 {
		for _, attachment := range email.Attachments {

			// Create the mandrill attachment
			mandrillAttachment := new(gochimp.Attachment)
			mandrillAttachment.Name = attachment.FileName
			mandrillAttachment.Type = attachment.FileType

			// Read all content from the attachment
			reader := bufio.NewReader(attachment.FileReader)
			var content []byte
			content, err = ioutil.ReadAll(reader)
			if err != nil {
				return
			}

			// Encode as base64
			encoded := base64.StdEncoding.EncodeToString(content)
			mandrillAttachment.Content = encoded

			// Add to the email
			message.Attachments = append(message.Attachments, *mandrillAttachment)
		}
	}

	// Execute the send
	var sendResponse []gochimp.SendResponse
	sendResponse, err = m.mandrillService.MessageSend(message, false)
	if err != nil {
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
