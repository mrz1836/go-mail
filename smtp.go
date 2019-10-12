package gomail

import (
	"fmt"
	"log"

	"github.com/domodwyer/mailyak"
)

// sendWithSMTP sends an email using the smtp service
func (m *MailService) sendWithSMTP(email *Email) (err error) {

	// Create new mail message
	mail := mailyak.New(fmt.Sprintf("%s:%d", m.SMTPHost, m.SMTPPort), m.smtpAuth)

	// Add the to recipients
	mail.To(email.Recipients...)

	// Add the cc recipients
	if len(email.RecipientsCc) > 0 {
		mail.Cc(email.RecipientsCc...)
	}

	// Add the bcc recipients
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
		log.Printf("warning: track clicks is enabled, but aws ses does not offer this feature")
	}
	if email.TrackOpens {
		log.Printf("warning: track opens is enabled, but aws ses does not offer this feature")
	}
	if email.AutoText {
		log.Printf("warning: auto text is enabled, but aws ses does not offer this feature")
	}

	// Send via smtp
	return mail.Send()
}
