package gomail

import (
	"bytes"
	"io"
	"log"
	"net/smtp"

	"github.com/domodwyer/mailyak"
)

// smtpInterface is an interface for mailyak/mocking
type smtpInterface interface {
	AddHeader(name, value string)
	Attach(name string, r io.Reader)
	AttachInline(name string, r io.Reader)
	AttachInlineWithMimeType(name string, r io.Reader, mimeType string)
	AttachWithMimeType(name string, r io.Reader, mimeType string)
	Bcc(addrs ...string)
	Cc(addrs ...string)
	ClearAttachments()
	From(addr string)
	FromName(name string)
	HTML() *mailyak.BodyPart
	MimeBuf() (*bytes.Buffer, error)
	Plain() *mailyak.BodyPart
	ReplyTo(addr string)
	Send() error
	String() string
	Subject(sub string)
	To(addrs ...string)
	WriteBccHeader(shouldWrite bool)
}

// newSMTPClient will create a new yak client given the connection string and auth
func newSMTPClient(host string, auth smtp.Auth) smtpInterface {
	return mailyak.New(host, auth)
}

// sendViaSMTP sends an email using the smtp service
func sendViaSMTP(client smtpInterface, email *Email) (err error) {

	// Add the "to" recipients
	client.To(email.Recipients...)

	// Add the "cc" recipients
	if len(email.RecipientsCc) > 0 {
		client.Cc(email.RecipientsCc...)
	}

	// Add the "bcc" recipients
	if len(email.RecipientsBcc) > 0 {
		client.WriteBccHeader(true)
		client.Bcc(email.RecipientsBcc...)
	}

	// Add the basics
	client.From(email.FromAddress)
	client.FromName(email.FromName)
	client.Subject(email.Subject)

	// Add a custom reply to address
	if len(email.ReplyToAddress) > 0 {
		client.ReplyTo(email.ReplyToAddress)
	}

	// Add plain text
	if len(email.PlainTextContent) > 0 {
		client.Plain().Set(email.PlainTextContent)
	}

	// Add html
	if len(email.HTMLContent) > 0 {
		client.HTML().Set(email.HTMLContent)
	}

	// Add any attachments
	if len(email.Attachments) > 0 {
		for _, att := range email.Attachments {
			client.Attach(att.FileName, att.FileReader)
		}
	}

	// Add importance?
	if email.Important {
		client.AddHeader("X-Priority", "1 (Highest)")
		client.AddHeader("X-MSMail-Priority", "High")
		client.AddHeader("Importance", "High")
	}

	// Warn about features that are set but not available
	if email.TrackClicks {
		log.Printf("warning: track clicks is enabled, SMTP does not have this feature")
	}
	if email.TrackOpens {
		log.Printf("warning: track opens is enabled, SMTP does not have this feature")
	}
	if email.AutoText {
		log.Printf("warning: auto text is enabled, SMTP does not have this feature")
	}

	// Send via smtp
	return client.Send()
}
