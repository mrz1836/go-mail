// Package gomail is a lightweight email package with multi-provider support
package gomail

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html"
	"html/template"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	texttemplate "text/template"
	"time"

	"github.com/aymerick/douceur/inliner"
	"github.com/aymerick/douceur/parser"
)

const (
	headerListUnsubscribe      = "List-Unsubscribe"
	headerListUnsubscribePost  = "List-Unsubscribe-Post"
	listUnsubscribeOneClick    = "List-Unsubscribe=One-Click"
	inlineCSSTemplateName      = "gomail:inline-css" // associated template marking an HTML template for CSS inlining
	stylesTemplatePlaceholder  = "{{.Styles}}"       // replaced with Email.CSS by ParseHTMLTemplate
	listUnsubscribeInvalidChar = "<>,\r\n\t "        // characters that cannot appear in a List-Unsubscribe target
)

// Email represents the fields of the email to send
//
// DO NOT CHANGE ORDER - Optimized for memory (maligned)
type Email struct {
	Attachments     []Attachment     `json:"attachments" mapstructure:"attachments"`
	CSS             []byte           `json:"css" mapstructure:"css"`
	ProviderOptions []ProviderOption `json:"-" mapstructure:"-"` // per-provider request customizations (see With)
	Recipients      []string         `json:"recipients" mapstructure:"recipients"`
	RecipientsBcc   []string         `json:"recipients_bcc" mapstructure:"recipients_bcc"`
	RecipientsCc    []string         `json:"recipients_cc" mapstructure:"recipients_cc"`
	// Deprecated: Styles is not used; set CSS and use the {{.Styles}} placeholder with ParseHTMLTemplate.
	Styles           []byte            `json:"styles" mapstructure:"styles"`
	Tags             []string          `json:"tags" mapstructure:"tags"`
	Headers          map[string]string `json:"headers" mapstructure:"headers"`   // custom headers (ie: List-Unsubscribe); see SetHeader
	Metadata         map[string]string `json:"metadata" mapstructure:"metadata"` // key/value data returned in provider webhooks
	SendAt           time.Time         `json:"send_at,omitzero" mapstructure:"send_at"`
	FromAddress      string            `json:"from_address" mapstructure:"from_address"`
	FromName         string            `json:"from_name" mapstructure:"from_name"`
	HTMLContent      string            `json:"html_content" mapstructure:"html_content"`
	IdempotencyKey   string            `json:"idempotency_key" mapstructure:"idempotency_key"` // prevents duplicate sends on retry (provider dependent)
	PlainTextContent string            `json:"plain_text_content" mapstructure:"plain_text_content"`
	ReplyToAddress   string            `json:"reply_to_address" mapstructure:"reply_to_address"`
	Subject          string            `json:"subject" mapstructure:"subject"`
	AutoText         bool              `json:"auto_text" mapstructure:"auto_text"`
	Important        bool              `json:"important" mapstructure:"important"`
	TrackClicks      bool              `json:"track_clicks" mapstructure:"track_clicks"`
	TrackOpens       bool              `json:"track_opens" mapstructure:"track_opens"`
	ViewContentLink  bool              `json:"view_content_link" mapstructure:"view_content_link"`
}

// Attachment is the email file attachment.
//
// Set either Content or FileReader. A FileReader can only be read once, so
// MailService.Send reads it into Content before sending; the email can then
// be retried or sent through another provider. Set ContentID to embed the
// attachment inline, referenced from the HTML as <img src="cid:ContentID">.
type Attachment struct {
	FileName   string    `json:"file_name" mapstructure:"file_name"`
	FileReader io.Reader `json:"-" mapstructure:"-"`
	FileType   string    `json:"file_type" mapstructure:"file_type"`
	Content    []byte    `json:"content,omitempty" mapstructure:"content"`
	ContentID  string    `json:"content_id,omitempty" mapstructure:"content_id"`
}

// TemplateExecutor is a parsed template; both *text/template.Template and
// *html/template.Template implement it
type TemplateExecutor interface {
	ExecuteTemplate(wr io.Writer, name string, data any) error
	Name() string
}

// AddAttachment adds a new attachment read from the reader
func (e *Email) AddAttachment(name, fileType string, reader io.Reader) {
	e.Attachments = append(e.Attachments, Attachment{
		FileType:   fileType,
		FileName:   name,
		FileReader: reader,
	})
}

// AddAttachmentBytes adds a new attachment with the given content
func (e *Email) AddAttachmentBytes(name, fileType string, content []byte) {
	e.Attachments = append(e.Attachments, Attachment{
		Content:  content,
		FileName: name,
		FileType: fileType,
	})
}

// AddInlineAttachment adds an attachment embedded in the HTML body, referenced
// as <img src="cid:contentID">
func (e *Email) AddInlineAttachment(name, fileType, contentID string, content []byte) {
	e.Attachments = append(e.Attachments, Attachment{
		Content:   content,
		ContentID: contentID,
		FileName:  name,
		FileType:  fileType,
	})
}

// SetHeader sets a custom header, replacing any header with the same name
// (compared case-insensitively). Headers go-mail sets from the email fields
// (ie: From, To, Subject) are rejected when the email is sent.
func (e *Email) SetHeader(name, value string) {
	if e.Headers == nil {
		e.Headers = make(map[string]string)
	}
	for existing := range e.Headers {
		if strings.EqualFold(existing, name) {
			delete(e.Headers, existing)
		}
	}
	e.Headers[name] = value
}

// SetListUnsubscribe sets the List-Unsubscribe header (RFC 2369) to the given
// https:, http: or mailto: targets. With oneClick, it also sets
// List-Unsubscribe-Post (RFC 8058), which requires an https: target. Gmail and
// Yahoo require one-click unsubscribe for bulk senders.
func (e *Email) SetListUnsubscribe(oneClick bool, targets ...string) error {
	if len(targets) == 0 {
		return fmt.Errorf("%w: at least one target is required", ErrInvalidListUnsubscribe)
	}

	values := make([]string, 0, len(targets))
	hasHTTPS := false
	for _, target := range targets {
		parsed, err := url.Parse(target)
		if err != nil || strings.ContainsAny(target, listUnsubscribeInvalidChar) {
			return fmt.Errorf("%w: %q", ErrInvalidListUnsubscribe, target)
		}
		switch strings.ToLower(parsed.Scheme) {
		case "https":
			hasHTTPS = true
		case "http", "mailto":
		default:
			return fmt.Errorf("%w: %q must be an https:, http: or mailto: URL", ErrInvalidListUnsubscribe, target)
		}
		values = append(values, "<"+target+">")
	}
	if oneClick && !hasHTTPS {
		return fmt.Errorf("%w: one-click unsubscribe requires an https: target", ErrInvalidListUnsubscribe)
	}

	e.SetHeader(headerListUnsubscribe, strings.Join(values, ", "))
	if oneClick {
		e.SetHeader(headerListUnsubscribePost, listUnsubscribeOneClick)
	} else {
		for name := range e.Headers {
			if strings.EqualFold(name, headerListUnsubscribePost) {
				delete(e.Headers, name)
			}
		}
	}
	return nil
}

// With adds provider options that customize the native request of a provider
// (ie: a PostmarkOption to set the MessageStream) and returns the email
func (e *Email) With(opts ...ProviderOption) *Email {
	e.ProviderOptions = append(e.ProviderOptions, opts...)
	return e
}

// ApplyTemplates will take the template files and process them with the email data (can be e or overridden).
//
// Use ParseTextTemplate for the text template: a plain-text template parsed
// with html/template (ie: ParseTemplate) has its output HTML-escaped, so for
// backwards compatibility that output is unescaped here. An HTML template from
// ParseHTMLTemplate has its CSS inlined after it is executed.
func (e *Email) ApplyTemplates(htmlTemplate *template.Template, textTemplate TemplateExecutor, emailData any) (err error) {
	// Use the default email if nil is given
	if emailData == nil {
		emailData = e
	}

	// Do we have an HTML template?
	if htmlTemplate != nil {
		var content string
		if content, err = executeHTMLTemplate(htmlTemplate, emailData); err != nil {
			return err
		}
		e.HTMLContent = content
	}

	// Do we have a text template?
	if !isNilTemplate(textTemplate) {
		var content string
		if content, err = executeTextTemplate(textTemplate, emailData); err != nil {
			return err
		}
		e.PlainTextContent = content
	}

	return nil
}

// ParseTemplate parses an HTML template file (html/template, so data is
// HTML-escaped). Use ParseTextTemplate for plain-text templates.
// This method returns the template which should be stored in memory for quick access
func (e *Email) ParseTemplate(filename string) (parsed *template.Template, err error) {
	return template.New(filepath.Base(filename)).ParseFiles(filename)
}

// ParseTextTemplate parses a plain-text template file (text/template, so data
// is not HTML-escaped)
// This method returns the template which should be stored in memory for quick access
func (e *Email) ParseTextTemplate(filename string) (*texttemplate.Template, error) {
	return texttemplate.New(filepath.Base(filename)).ParseFiles(filename)
}

// ParseHTMLTemplate parses an HTML template with style injection: the
// {{.Styles}} placeholder is replaced with the email CSS, and ApplyTemplates
// inlines the CSS into the rendered HTML (inlining after execution keeps
// template actions inside tables and attributes intact).
// This method returns the template which should be stored in memory for quick access
func (e *Email) ParseHTMLTemplate(htmlLocation string) (htmlTemplate *template.Template, err error) {
	// Read HTML template file
	var tempBytes []byte
	if tempBytes, err = os.ReadFile(htmlLocation); err != nil { //nolint:gosec // G304: the template path is supplied by the application
		return nil, err
	}

	// Either no style placeholder or no CSS set on the email
	if !bytes.Contains(tempBytes, []byte(stylesTemplatePlaceholder)) || len(e.CSS) == 0 {
		return e.ParseTemplate(htmlLocation)
	}

	// Validate the CSS up front so a bad stylesheet fails here, not on every send
	if _, err = parser.Parse(string(e.CSS)); err != nil {
		return nil, err
	}

	// Inject the styles and mark the template for CSS inlining
	tempBytes = bytes.ReplaceAll(tempBytes, []byte(stylesTemplatePlaceholder), e.CSS)
	if htmlTemplate, err = template.New(filepath.Base(htmlLocation)).Parse(string(tempBytes)); err != nil {
		return nil, err
	}
	if _, err = htmlTemplate.New(inlineCSSTemplateName).Parse(""); err != nil {
		return nil, err
	}

	return htmlTemplate, nil
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

	return email
}

// SendEmail will send an email using the given provider
func (m *MailService) SendEmail(ctx context.Context, email *Email, provider ServiceProvider) error {
	_, err := m.Send(ctx, email, provider)
	return err
}

// Send validates and sends an email, returning the provider's message id.
//
// With several providers, each is tried in order until one accepts the email
// (failover); with none, every available provider is tried in the order they
// were loaded. Failover resends the whole email, so a provider that fails
// after accepting it (ie: a timeout on the response) can cause a duplicate.
//
// Reader-based attachments are read into Attachment.Content first, so the
// same email can be retried or sent again.
func (m *MailService) Send(ctx context.Context, email *Email, providers ...ServiceProvider) (*SendResult, error) {
	if len(providers) == 0 {
		providers = slices.Clone(m.AvailableProviders)
	}
	if len(providers) == 0 {
		return nil, ErrNoServiceProvider
	}

	// Check that every provider is available
	for _, id := range providers {
		if !m.hasProvider(id) {
			return nil, providerNotFoundErr(id, m.AvailableProviders)
		}
	}

	// Validate the email and load the attachments
	if err := m.validateEmail(email); err != nil {
		return nil, err
	}
	if err := bufferAttachments(email, m.maxAttachmentSize()); err != nil {
		return nil, err
	}

	// Send it via the providers, in order
	errs := make([]error, 0, len(providers))
	for _, id := range providers {
		result, err := m.sendWith(ctx, id, email)
		if err == nil {
			return result, nil
		}
		if len(providers) == 1 {
			return nil, err
		}
		errs = append(errs, fmt.Errorf("%s: %w", id, err))
		if ctx.Err() != nil {
			break
		}
	}
	return nil, errors.Join(errs...)
}

// sendWith sends the email with one provider, checking the requested features
// and bounding the send with the configured timeout
func (m *MailService) sendWith(ctx context.Context, id ServiceProvider, email *Email) (*SendResult, error) {
	provider := m.providers[id]

	// Check the optional features against the provider
	if unsupported := unsupportedFeatures(provider, email); len(unsupported) > 0 {
		if m.StrictFeatures || slices.Contains(unsupported, FeatureSendAt) {
			return nil, fmt.Errorf("%w: %s does not support %v", ErrUnsupportedFeature, id, unsupported)
		}
		m.logger().WarnContext(ctx, "gomail: the email uses features the provider does not support; they will be ignored",
			slog.String("provider", id.String()), slog.Any("features", unsupported))
	}

	if timeout := m.sendTimeout(); timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	result, err := provider.Send(ctx, email)
	if err != nil {
		return nil, err
	}

	// Copy the result so a provider may safely return a shared value
	var sent SendResult
	if result != nil {
		sent = *result
	}
	sent.Provider = id
	return &sent, nil
}

// validateEmail performs standard email validation checks
func (m *MailService) validateEmail(email *Email) error {
	if email == nil {
		return ErrMissingContent
	}
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
	if _, err := parseEnvelope(email); err != nil {
		return err
	}
	if err := validateHeaders(email.Headers); err != nil {
		return err
	}
	if err := validateMetadata(email.Metadata); err != nil {
		return err
	}
	return validateAttachments(email.Attachments)
}

// providerNotFoundErr builds the error returned when a requested provider is not
// in the list of available providers
func providerNotFoundErr(provider ServiceProvider, available []ServiceProvider) error {
	return fmt.Errorf("service provider: %s was not in the list of available service providers: %v, email not sent: %w", provider, available, ErrProviderNotFound)
}

// executeHTMLTemplate renders an HTML template, inlining its CSS when it was
// parsed by ParseHTMLTemplate
func executeHTMLTemplate(htmlTemplate *template.Template, data any) (string, error) {
	var buffer bytes.Buffer
	if err := htmlTemplate.ExecuteTemplate(&buffer, htmlTemplate.Name(), data); err != nil {
		return "", err
	}
	if htmlTemplate.Lookup(inlineCSSTemplateName) == nil {
		return buffer.String(), nil
	}
	return inliner.Inline(buffer.String())
}

// executeTextTemplate renders a plain-text template; output from an
// html/template is unescaped since plain text must not contain HTML entities
func executeTextTemplate(textTemplate TemplateExecutor, data any) (string, error) {
	var buffer bytes.Buffer
	if err := textTemplate.ExecuteTemplate(&buffer, textTemplate.Name(), data); err != nil {
		return "", err
	}
	if _, isHTML := textTemplate.(*template.Template); isHTML {
		return html.UnescapeString(buffer.String()), nil
	}
	return buffer.String(), nil
}

// isNilTemplate reports whether the template is nil (including a typed nil pointer)
func isNilTemplate(t TemplateExecutor) bool {
	if t == nil {
		return true
	}
	v := reflect.ValueOf(t)
	return v.Kind() == reflect.Pointer && v.IsNil()
}
