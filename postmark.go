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

// postmarkInterface is an interface for Postmark/mocking
type postmarkInterface interface {
	SendEmail(email postmark.Email) (postmark.EmailResponse, error)
}

// sendViaPostmark sends an email using the Postmark service
func sendViaPostmark(client postmarkInterface, email *Email) (err error) {

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

	// Set the "from" name if given
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
	for _, attachment := range email.Attachments {

		// Create the postmark attachment
		postmarkAttachment := &postmark.Attachment{
			ContentType: attachment.FileType,
			Name:        attachment.FileName,
		}

		// Read all content from the attachment
		reader := bufio.NewReader(attachment.FileReader)
		var content []byte
		if content, err = ioutil.ReadAll(reader); err != nil {
			return
		}

		// Encode as base64
		postmarkAttachment.Content = base64.StdEncoding.EncodeToString(content)

		// Add to the email
		postmarkEmail.Attachments = append(postmarkEmail.Attachments, *postmarkAttachment)
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
	if resp, err = client.SendEmail(postmarkEmail); err != nil {
		return
	}

	// Check the response from Postmark
	if resp.ErrorCode > 0 {
		err = fmt.Errorf("error from postmark: %s error code: %d", resp.Message, resp.ErrorCode)
	}

	return
}
