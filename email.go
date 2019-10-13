package gomail

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"path/filepath"

	"github.com/aymerick/douceur/inliner"
)

// Email represents the fields of the email to send
type Email struct {
	Attachments      []Attachment `json:"attachments" mapstructure:"attachments"`
	AutoText         bool         `json:"auto_text" mapstructure:"auto_text"`
	CSS              []byte       `json:"css" mapstructure:"css"`
	FromAddress      string       `json:"from_address" mapstructure:"from_address"`
	FromName         string       `json:"from_name" mapstructure:"from_name"`
	HTMLContent      string       `json:"html_content" mapstructure:"html_content"`
	Important        bool         `json:"important" mapstructure:"important"`
	PlainTextContent string       `json:"plain_text_content" mapstructure:"plain_text_content"`
	Recipients       []string     `json:"recipients" mapstructure:"recipients"`
	RecipientsBcc    []string     `json:"recipients_bcc" mapstructure:"recipients_bcc"`
	RecipientsCc     []string     `json:"recipients_cc" mapstructure:"recipients_cc"`
	ReplyToAddress   string       `json:"reply_to_address" mapstructure:"reply_to_address"`
	Styles           []byte       `json:"styles" mapstructure:"styles"`
	Subject          string       `json:"subject" mapstructure:"subject"`
	Tags             []string     `json:"tags" mapstructure:"tags"`
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

// ApplyTemplates will take the template files and process them with the email dataStruct (can be e or overridden)
func (e *Email) ApplyTemplates(htmlTemplate *template.Template, textTemplate *template.Template, emailData interface{}) (err error) {

	// Start the buffer
	var buffer bytes.Buffer

	// Use the default email if nil is given
	if emailData == nil {
		emailData = e
	}

	// Do we have an html template?
	if htmlTemplate != nil {

		// Read the struct into the HTML buffer
		err = htmlTemplate.ExecuteTemplate(&buffer, htmlTemplate.Name(), emailData)
		if err != nil {
			return
		}

		// Turn the buffer to a string
		e.HTMLContent = buffer.String()

		// Reset the buffer to ""
		buffer.Reset()
	}

	// Do we have a text template?
	if textTemplate != nil {

		// Read the struct into the text buffer
		err = textTemplate.ExecuteTemplate(&buffer, textTemplate.Name(), emailData)
		if err != nil {
			return
		}

		// Turn the buffer to a string
		e.PlainTextContent = buffer.String()
	}

	return
}

// ParseTemplate parse the template, fire error if parse fails
// This method returns the template which should be stored in memory for quick access
func (e *Email) ParseTemplate(filename string) (parsed *template.Template, err error) {
	parsed = template.New(filepath.Base(filename))
	return parsed.ParseFiles(filename)
}

// ParseHTMLTemplate parse the template with inline style injection (html)
// This method returns the template which should be stored in memory for quick access
func (e *Email) ParseHTMLTemplate(htmlLocation string) (htmlTemplate *template.Template, err error) {

	// Read HTML template file
	var tempBytes []byte
	tempBytes, err = ioutil.ReadFile(htmlLocation)
	if err != nil {
		err = fmt.Errorf("")
		return
	}

	// Do we have styles to replace?
	if bytes.Contains(tempBytes, []byte("{{.Styles}}")) && len(e.CSS) > 0 {

		// Inject styles
		tempBytes = bytes.Replace(tempBytes, []byte("{{.Styles}}"), e.CSS, -1)
		var tempString string
		tempString, err = inliner.Inline(string(tempBytes))
		if err != nil {
			return
		}

		// Replace the string with template
		htmlTemplate, err = e.ParseTemplate(htmlLocation)
		if err != nil {
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
			err = fmt.Errorf("email is missing a recipient")
			return
		} else if len(email.Recipients) > m.MaxToRecipients {
			err = fmt.Errorf("max TO recipient limit of %d reached: %d", m.MaxToRecipients, len(email.Recipients))
			return
		} else if len(email.RecipientsCc) > m.MaxCcRecipients {
			err = fmt.Errorf("max CC recipient limit of %d reached: %d", m.MaxCcRecipients, len(email.RecipientsCc))
			return
		} else if len(email.RecipientsBcc) > m.MaxBccRecipients {
			err = fmt.Errorf("max BCC recipient limit of %d reached: %d", m.MaxBccRecipients, len(email.RecipientsBcc))
			return
		}

		// Send using given provider
		if provider == AwsSes {
			err = m.sendWithAwsSes(email)
		} else if provider == Mandrill {
			err = m.sendWithMandrill(email)
		} else if provider == Postmark {
			err = m.sendWithPostmark(email)
		} else if provider == SMTP {
			err = m.sendWithSMTP(email)
		}
	} else {
		err = fmt.Errorf("service provider: %x was not in the list of available service providers: %x, email not sent", provider, m.AvailableProviders)
	}

	return
}
