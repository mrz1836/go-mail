package gomail

import (
	"fmt"
	"io"
)

// Email represents the fields of the email to send
type Email struct {
	Attachments      []Attachment `json:"attachments" mapstructure:"attachments"`
	FromAddress      string       `json:"from_address" mapstructure:"from_address"`
	FromName         string       `json:"from_name" mapstructure:"from_name"`
	HTMLContent      string       `json:"html_content" mapstructure:"html_content"`
	Important        bool         `json:"important" mapstructure:"important"`
	PlainTextContent string       `json:"plain_text_content" mapstructure:"plain_text_content"`
	Recipients       []string     `json:"recipients" mapstructure:"recipients"`
	RecipientsBcc    []string     `json:"recipients_bcc" mapstructure:"recipients_bcc"`
	RecipientsCc     []string     `json:"recipients_cc" mapstructure:"recipients_cc"`
	ReplyToAddress   string       `json:"reply_to_address" mapstructure:"reply_to_address"`
	Subject          string       `json:"subject" mapstructure:"subject"`
	Tags             []string     `json:"tags" mapstructure:"tags"`
	AutoText         bool         `json:"auto_text" mapstructure:"auto_text"`
	TrackClicks      bool         `json:"track_clicks" mapstructure:"track_clicks"`
	TrackOpens       bool         `json:"track_opens" mapstructure:"track_opens"`
	ViewContentLink  bool         `json:"view_content_link" mapstructure:"view_content_link"`
}

// Attachment is the attachment
type Attachment struct {
	FileName   string    `json:"file_name" mapstructure:"file_name"`
	FileReader io.Reader `json:"-" mapstructure:"-"`
	FileType   string    `json:"file_type" mapstructure:"file_type"`
}

// AddAttachment adds a new attachment
func (e *Email) AddAttachment(name, fileType string, reader io.Reader) {
	e.Attachments = append(e.Attachments, Attachment{
		FileType:   fileType,
		FileName:   name,
		FileReader: reader,
	})
}

// NewEmail creates a new email using defaults from the service configuration
func (m *MailService) NewEmail() (email *Email) {

	// Create new email using defaults
	email = new(Email)
	email.AutoText = m.AutoText
	email.FromAddress = m.FromUsername + "@" + m.FromDomain
	email.FromName = m.FromName
	email.Important = m.Important
	email.TrackClicks = m.TrackClicks
	email.TrackOpens = m.TrackOpens
	email.ReplyToAddress = email.FromAddress

	return
}

// SendEmail will send an email using the given provider
func (m *MailService) SendEmail(email *Email, provider ServiceProvider) (err error) {

	// Do we have that provider?
	if containsServiceProvider(m.AvailableProviders, provider) {

		// Safe guard the user sending mis-configured emails
		if len(email.Subject) == 0 {
			err = fmt.Errorf("email is missing a subject")
			return
		} else if len(email.PlainTextContent) == 0 && len(email.HTMLContent) == 0 {
			err = fmt.Errorf("email is missing content (plain & html)")
			return
		} else if len(email.Recipients) == 0 {
			err = fmt.Errorf("email is a recipient")
			return
		} else if len(email.Recipients) > maxToRecipients {
			err = fmt.Errorf("max TO recipient limit of %d reached: %d", maxToRecipients, len(email.Recipients))
			return
		} else if len(email.RecipientsCc) > maxCcRecipients {
			err = fmt.Errorf("max CC recipient limit of %d reached: %d", maxCcRecipients, len(email.RecipientsCc))
			return
		} else if len(email.RecipientsBcc) > maxBccRecipients {
			err = fmt.Errorf("max BCC recipient limit of %d reached: %d", maxBccRecipients, len(email.RecipientsBcc))
			return
		}

		// Send using given provider
		if provider == Mandrill {
			err = m.sendWithMandrill(email)
		} else if provider == AwsSes {
			err = m.sendWithAwsSes(email)
		} else if provider == Postmark {
			err = m.sendWithPostmark(email)
		} else if provider == Smtp {
			err = m.sendWithSmtp(email)
		}
	} else {
		err = fmt.Errorf("service provider: %x was not in the list of available service providers: %x, email not sent", provider, m.AvailableProviders)
	}

	return
}
