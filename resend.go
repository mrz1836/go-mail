package gomail

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/resend/resend-go/v4"
)

const (
	resendTagMaxLength = 256    // maximum length Resend allows for a tag name or value
	resendTagValue     = "true" // value applied to every tag (go-mail tags have no value)
)

// resendInterface is an interface for Resend/mocking
type resendInterface interface {
	SendWithContext(ctx context.Context, params *resend.SendEmailRequest) (*resend.SendEmailResponse, error)
}

// newResendClient creates a new Resend client; *resend.EmailsSvcImpl already
// satisfies resendInterface, so no wrapper type is needed
func newResendClient(apiKey string) resendInterface {
	return resend.NewClient(apiKey).Emails
}

// sendViaResend sends an email using the Resend service
func sendViaResend(ctx context.Context, client resendInterface, email *Email) (err error) {
	// Create the Resend request
	request := &resend.SendEmailRequest{
		Bcc:     email.RecipientsBcc,
		Cc:      email.RecipientsCc,
		From:    email.FromAddress,
		Html:    email.HTMLContent,
		ReplyTo: email.ReplyToAddress,
		Subject: email.Subject,
		Tags:    resendTags(email.Tags),
		Text:    email.PlainTextContent,
		To:      email.Recipients,
	}

	// Set the "from" name if given (RFC 5322: "Name <address>")
	if len(email.FromName) > 0 {
		request.From = email.FromName + " <" + email.FromAddress + ">"
	}

	// Add importance headers
	if email.Important {
		request.Headers = map[string]string{
			headerXPriority:       headerXPriorityValue,
			headerXMSMailPriority: headerHighValue,
			headerImportance:      headerHighValue,
		}
	}

	// Warn about features that are set but not available per email
	// (Resend auto-generates the plain-text part from HTML natively, so no auto text warning)
	if email.TrackClicks || email.TrackOpens {
		log.Printf("warning: open/click tracking is enabled, but Resend configures tracking per domain (not per email)")
	}

	// Convert attachments to Resend format (the SDK encodes the raw bytes itself)
	for _, attachment := range email.Attachments {

		// Read the attachment contents
		var content []byte
		if content, err = io.ReadAll(attachment.FileReader); err != nil {
			return err
		}

		// Add to the request
		request.Attachments = append(request.Attachments, &resend.Attachment{
			Content:     content,
			ContentType: attachment.FileType,
			Filename:    attachment.FileName,
		})
	}

	// Send the email
	var resp *resend.SendEmailResponse
	if resp, err = client.SendWithContext(ctx, request); err != nil {
		return fmt.Errorf("error from resend: %w: %w", ErrResendError, err)
	}

	// Resend returns the id of the created email on success
	if resp == nil || len(resp.Id) == 0 {
		err = fmt.Errorf("error from resend: empty message id: %w", ErrResendError)
	}

	return err
}

// resendTags converts go-mail tags into Resend tags; Resend only allows ASCII
// letters, numbers, underscores, and dashes (max 256 chars), so any other
// characters are replaced with an underscore. Empty and duplicate tags are dropped.
func resendTags(tags []string) []resend.Tag {
	if len(tags) == 0 {
		return nil
	}

	resendTagList := make([]resend.Tag, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, tag := range tags {

		// Sanitize the tag into a valid Resend tag name
		name := strings.Map(func(r rune) rune {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
				return r
			}
			return '_'
		}, tag)
		if len(name) > resendTagMaxLength {
			name = name[:resendTagMaxLength]
		}

		// Skip empty and duplicate tags
		if _, ok := seen[name]; ok || len(name) == 0 {
			continue
		}
		seen[name] = struct{}{}

		resendTagList = append(resendTagList, resend.Tag{Name: name, Value: resendTagValue})
	}

	return resendTagList
}
