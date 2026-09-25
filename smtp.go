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

// smtpClientFactory builds an SMTP client for the given connection string and auth
type smtpClientFactory func(host string, auth smtp.Auth) smtpInterface

// newSMTPClient will create a new yak client given the connection string and auth
func newSMTPClient(host string, auth smtp.Auth) smtpInterface {
	return mailyak.New(host, auth)
}

// newSMTPMessageClient returns a fresh SMTP client for a single send.
//
// A mailyak client holds the state of one message (recipients, body parts,
// attachments, headers, and the date), so it must never be reused across
// sends: a reused client leaks that state into the next email and races when
// emails are sent concurrently.
func (m *MailService) newSMTPMessageClient() smtpInterface {
	if m.smtpClientFactory != nil {
		return m.smtpClientFactory(m.smtpAddr, m.smtpAuth)
	}
	return newSMTPClient(m.smtpAddr, m.smtpAuth)
}

// populateMailyakMessage fills a mailyak message (or any smtpInterface) with the
// fields shared by the mailyak-based providers (AWS SES and SMTP): recipients,
// sender, subject, reply-to, body parts, attachments, and importance headers
func populateMailyakMessage(client smtpInterface, email *Email) {
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
	for _, att := range email.Attachments {
		client.Attach(att.FileName, att.FileReader)
	}

	// Add importance?
	if email.Important {
		client.AddHeader(headerXPriority, headerXPriorityValue)
		client.AddHeader(headerXMSMailPriority, headerHighValue)
		client.AddHeader(headerImportance, headerHighValue)
	}
}

// sendViaSMTP sends an email using the smtp service
func sendViaSMTP(client smtpInterface, email *Email) (err error) {
	// Populate the shared mailyak message fields
	populateMailyakMessage(client, email)

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
