// Package gomail is a lightweight email package with multi-provider support
package gomail

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"

	"github.com/aymerick/douceur/inliner"
)

// Email represents the fields of the email to send
//
// DO NOT CHANGE ORDER - Optimized for memory (maligned)
type Email struct {
	Attachments      []Attachment `json:"attachments" mapstructure:"attachments"`
	CSS              []byte       `json:"css" mapstructure:"css"`
	Recipients       []string     `json:"recipients" mapstructure:"recipients"`
	RecipientsBcc    []string     `json:"recipients_bcc" mapstructure:"recipients_bcc"`
	RecipientsCc     []string     `json:"recipients_cc" mapstructure:"recipients_cc"`
	Styles           []byte       `json:"styles" mapstructure:"styles"`
	Tags             []string     `json:"tags" mapstructure:"tags"`
	FromAddress      string       `json:"from_address" mapstructure:"from_address"`
	FromName         string       `json:"from_name" mapstructure:"from_name"`
	HTMLContent      string       `json:"html_content" mapstructure:"html_content"`
	PlainTextContent string       `json:"plain_text_content" mapstructure:"plain_text_content"`
	ReplyToAddress   string       `json:"reply_to_address" mapstructure:"reply_to_address"`
	Subject          string       `json:"subject" mapstructure:"subject"`
	AutoText         bool         `json:"auto_text" mapstructure:"auto_text"`
	Important        bool         `json:"important" mapstructure:"important"`
	TrackClicks      bool         `json:"track_clicks" mapstructure:"track_clicks"`
	TrackOpens       bool         `json:"track_opens" mapstructure:"track_opens"`
	ViewContentLink  bool         `json:"view_content_link" mapstructure:"view_content_link"`
}

// Attachment is the email file attachment
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

// ApplyTemplates will take the template files and process them with the email data (can be e or overridden)
func (e *Email) ApplyTemplates(htmlTemplate, textTemplate *template.Template, emailData interface{}) (err error) {
	// Start the buffer
	var buffer bytes.Buffer

	// Use the default email if nil is given
	if emailData == nil {
		emailData = e
	}

	// Do we have an HTML template?
	if htmlTemplate != nil {

		// Read the struct into the HTML buffer
		if err = htmlTemplate.ExecuteTemplate(&buffer, htmlTemplate.Name(), emailData); err != nil {
			return err
		}

		// Turn the buffer to a string
		e.HTMLContent = buffer.String()

		// Reset the buffer to ""
		buffer.Reset()
	}

	// Do we have a text template?
	if textTemplate != nil {

		// Read the struct into the text buffer
		if err = textTemplate.ExecuteTemplate(&buffer, textTemplate.Name(), emailData); err != nil {
			return err
		}

		// Turn the buffer to a string
		e.PlainTextContent = buffer.String()
	}

	return nil
}

// ParseTemplate parse the template, fire error if parse fails
// This method returns the template which should be stored in memory for quick access
func (e *Email) ParseTemplate(filename string) (parsed *template.Template, err error) {
	return template.New(filepath.Base(filename)).ParseFiles(filename)
}

// ParseHTMLTemplate parse the template with inline style injection (html)
// This method returns the template which should be stored in memory for quick access
func (e *Email) ParseHTMLTemplate(htmlLocation string) (htmlTemplate *template.Template, err error) {
	// Read HTML template file
	var tempBytes []byte
	if tempBytes, err = os.ReadFile(htmlLocation); err != nil { //nolint:gosec // No security issue here
		return
	}

	// Do we have styles to replace?
	if bytes.Contains(tempBytes, []byte("{{.Styles}}")) && len(e.CSS) > 0 {

		// Inject styles
		tempBytes = bytes.ReplaceAll(tempBytes, []byte("{{.Styles}}"), e.CSS)
		var tempString string
		if tempString, err = inliner.Inline(string(tempBytes)); err != nil {
			return
		}

		// Replace the string with template
		if htmlTemplate, err = e.ParseTemplate(htmlLocation); err != nil {
			return
		}
		_, err = htmlTemplate.Parse(tempString)

	} else {
		// Either no style tag or no CSS set on email
		htmlTemplate, err = e.ParseTemplate(htmlLocation)
	}

	return
}

// NewEmail creates a new email using defaults from the service configuration
func (m *MailService) NewEmail() (email *Email) {
	// Create new email using defaults
	email = new(Email)
	email.AutoText = m.AutoText
	email.FromAddress = m.FromUsername + "@" + m.FromDomain
	email.CSS = m.EmailCSS
	email.FromName = m.FromName
	email.Important = m.Important
	email.ReplyToAddress = email.FromAddress
	email.TrackClicks = m.TrackClicks
	email.TrackOpens = m.TrackOpens

	return
}

// validateEmail performs standard email validation checks
func (m *MailService) validateEmail(email *Email) error {
	if len(email.Subject) == 0 {
		return ErrMissingSubject
	}
	if len(email.PlainTextContent) == 0 && len(email.HTMLContent) == 0 {
		return ErrMissingContent
	}
	if len(email.Recipients) == 0 {
		return ErrMissingRecipient
	}
	if len(email.Recipients) > m.MaxToRecipients {
		return fmt.Errorf("max TO recipient limit of %d reached: %d: %w", m.MaxToRecipients, len(email.Recipients), ErrMaxToRecipientsReached)
	}
	if len(email.RecipientsCc) > m.MaxCcRecipients {
		return fmt.Errorf("max CC recipient limit of %d reached: %d: %w", m.MaxCcRecipients, len(email.RecipientsCc), ErrMaxCcRecipientsReached)
	}
	if len(email.RecipientsBcc) > m.MaxBccRecipients {
		return fmt.Errorf("max BCC recipient limit of %d reached: %d: %w", m.MaxBccRecipients, len(email.RecipientsBcc), ErrMaxBccRecipientsReached)
	}
	return nil
}

// SendEmail will send an email using the given provider
func (m *MailService) SendEmail(ctx context.Context, email *Email, provider ServiceProvider) (err error) {
	// Check if provider is available
	if !containsServiceProvider(m.AvailableProviders, provider) {
		return fmt.Errorf("service provider: %x was not in the list of available service providers: %x, email not sent: %w", provider, m.AvailableProviders, ErrProviderNotFound)
	}

	// Validate email configuration
	if err = m.validateEmail(email); err != nil {
		return err
	}

	// Send it via the given provider
	switch provider {
	case AwsSes:
		err = sendViaAwsSes(m.awsSesService, email)
	case Mandrill:
		err = sendViaMandrill(m.mandrillService, email, true)
	case Postmark:
		err = sendViaPostmark(ctx, m.postmarkService, email)
	case SMTP:
		err = sendViaSMTP(m.smtpClient, email)
	default:
		err = fmt.Errorf("service provider: %x was not in the list of available service providers: %x, email not sent: %w", provider, m.AvailableProviders, ErrProviderNotFound)
	}

	return err
}
