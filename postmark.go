package gomail

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"strings"

	"github.com/mrz1836/postmark"
)

// sendWithPostmark sends an email using the postmark service
func (m *MailService) sendWithPostmark(email *Email) (err error) {

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

	// Warn about features that are set but not available //todo: remove once enabled
	if email.AutoText {
		log.Printf("warning: auto text is enabled, but postmark does not offer this feature")
	}

	// Set the from name if given
	if len(email.FromName) > 0 {
		postmarkEmail.From = email.FromName + " " + email.FromAddress
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
	if len(email.Attachments) > 0 {
		for _, attachment := range email.Attachments {

			// Create the postmark attachment
			postmarkAttachment := new(postmark.Attachment)
			postmarkAttachment.Name = attachment.FileName
			postmarkAttachment.ContentType = attachment.FileType

			// Read all content from the attachment
			reader := bufio.NewReader(attachment.FileReader)
			var content []byte
			content, err = ioutil.ReadAll(reader)
			if err != nil {
				return
			}

			// Encode as base64
			encoded := base64.StdEncoding.EncodeToString(content)
			postmarkAttachment.Content = encoded

			// Add to the email
			postmarkEmail.Attachments = append(postmarkEmail.Attachments, *postmarkAttachment)
		}
	}

	// Add importance
	if email.Important {
		postmarkEmail.Headers = append(
			postmarkEmail.Headers,
			postmark.Header{Name: "X-Priority", Value: "1 (Highest)"},
			postmark.Header{Name: "X-MSMail-Priority", Value: "High"},
			postmark.Header{Name: "Importance", Value: "High"},
		)
	}

	// Send the email
	var resp postmark.EmailResponse
	resp, err = m.postmarkService.SendEmail(postmarkEmail)
	if err != nil {
		return
	}

	// Check the response from postmark
	if resp.ErrorCode > 0 {
		err = fmt.Errorf("error from postmark: %s error code: %d to email: %s", resp.Message, resp.ErrorCode, resp.To)
	}

	return
}
