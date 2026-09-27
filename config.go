package gomail

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/mattbaird/gochimp"
	"github.com/mrz1836/postmark"
	"github.com/resend/resend-go/v4"
	sendgrid "github.com/sendgrid/sendgrid-go"
)

// ServiceProvider is the provider
type ServiceProvider int

// Email Service Providers
const (
	AwsSes   ServiceProvider = iota // AWS SES Email Service
	Mandrill                        // Mandrill Email Service
	Postmark                        // Postmark Email Service
	SMTP                            // SMTP Email Service
	SendGrid                        // SendGrid Email Service
	Resend                          // Resend Email Service
)

// String returns the human-readable name of the service provider
func (s ServiceProvider) String() string {
	switch s {
	case AwsSes:
		return "AwsSes"
	case Mandrill:
		return "Mandrill"
	case Postmark:
		return "Postmark"
	case SMTP:
		return "SMTP"
	case SendGrid:
		return "SendGrid"
	case Resend:
		return "Resend"
	default:
		return "Unknown"
	}
}

const (
	awsSesDefaultRegion      = "us-east-1"
	defaultMaxAttachmentSize = 40 << 20 // 40 MiB, the largest message the API providers accept
	defaultSendTimeout       = time.Minute
	maxBccRecipients         = 50
	maxCcRecipients          = 50
	maxToRecipients          = 50
	redactedValue            = "[REDACTED]"

	// Importance headers applied when an email is marked as important
	headerXPriority       = "X-Priority"
	headerXPriorityValue  = "1 (Highest)"
	headerXMSMailPriority = "X-MSMail-Priority"
	headerImportance      = "Importance"
	headerHighValue       = "High"
)

// MailService is the configuration to use for loading the service and provider's clients
//
// Secrets (API keys, tokens and passwords) are redacted when a MailService is
// marshaled to JSON or formatted with fmt/slog.
//
// DO NOT CHANGE ORDER - Optimized for memory (maligned)
type MailService struct { //nolint:recvcheck // value receivers on MarshalJSON/String/GoString redact both values and pointers
	AvailableProviders     []ServiceProvider            `json:"available_providers" mapstructure:"available_providers"` // list of providers that loaded successfully
	EmailCSS               []byte                       `json:"email_css" mapstructure:"email_css"`                     // default css pre-parsed into bytes
	Logger                 *slog.Logger                 `json:"-" mapstructure:"-"`                                     // logger for warnings (default: slog.Default())
	providers              map[ServiceProvider]Provider // registered providers
	AwsSesAccessID         string                       `json:"aws_ses_access_id" mapstructure:"aws_ses_access_id"`                 // aws iam access id for ses service
	AwsSesConfigurationSet string                       `json:"aws_ses_configuration_set" mapstructure:"aws_ses_configuration_set"` // optional ses configuration set (ie: for open/click tracking events)
	AwsSesEndpoint         string                       `json:"aws_ses_endpoint" mapstructure:"aws_ses_endpoint"`                   // ie: https://email.us-east-1.amazonaws.com
	AwsSesSecretKey        string                       `json:"aws_ses_secret_key" mapstructure:"aws_ses_secret_key"`               // aws iam secret key for corresponding access id
	AwsSesRegion           string                       `json:"aws_ses_region" mapstructure:"aws_ses_region"`                       // AWS region
	FromDomain             string                       `json:"from_domain" mapstructure:"from_domain"`                             // ie: example.com
	FromName               string                       `json:"from_name" mapstructure:"from_name"`                                 // ie: No Reply
	FromUsername           string                       `json:"from_username" mapstructure:"from_username"`                         // ie: no-reply
	MandrillAPIKey         string                       `json:"mandrill_api_key" mapstructure:"mandrill_api_key"`                   // mandrill api key
	PostmarkServerToken    string                       `json:"postmark_server_token" mapstructure:"postmark_server_token"`         // ie: abc123...
	ResendAPIKey           string                       `json:"resend_api_key" mapstructure:"resend_api_key"`                       // resend api key (ie: re_xxxx...)
	SMTPHost               string                       `json:"smtp_host" mapstructure:"smtp_host"`                                 // ie: example.com
	SMTPPassword           string                       `json:"smtp_password" mapstructure:"smtp_password"`                         // ie: secretPassword
	SendGridAPIKey         string                       `json:"sendgrid_api_key" mapstructure:"sendgrid_api_key"`                   // sendgrid api key
	SMTPUsername           string                       `json:"smtp_username" mapstructure:"smtp_username"`                         // ie: testuser (leave empty for a relay without authentication)
	MaxAttachmentSize      int64                        `json:"max_attachment_size" mapstructure:"max_attachment_size"`             // max total attachment bytes per email (0: 40 MiB, negative: unlimited)
	SendTimeout            time.Duration                `json:"send_timeout" mapstructure:"send_timeout"`                           // max time for one provider send (0: 1 minute, negative: none)
	MaxBccRecipients       int                          `json:"max_bcc_recipients" mapstructure:"max_bcc_recipients"`               // max amount for BCC
	MaxCcRecipients        int                          `json:"max_cc_recipients" mapstructure:"max_cc_recipients"`                 // max amount for CC
	MaxToRecipients        int                          `json:"max_to_recipients" mapstructure:"max_to_recipients"`                 // max amount for TO
	SMTPPort               int                          `json:"smtp_port" mapstructure:"smtp_port"`                                 // ie: 587 (default), 465 (implicit TLS) or 25
	AutoText               bool                         `json:"auto_text" mapstructure:"auto_text"`                                 // whether to automatically generate a text part for messages that are not given text
	Important              bool                         `json:"important" mapstructure:"important"`                                 // whether this message is important, and should be delivered ahead of non-important messages
	TrackClicks            bool                         `json:"track_clicks" mapstructure:"track_clicks"`                           // whether to turn on click tracking for the message
	TrackOpens             bool                         `json:"track_opens" mapstructure:"track_opens"`                             // whether to turn on open tracking for the message
	AwsSesUseIAMRole       bool                         `json:"aws_ses_use_iam_role" mapstructure:"aws_ses_use_iam_role"`           // load AWS SES using the default credential chain (IAM role) instead of static access keys
	SMTPImplicitTLS        bool                         `json:"smtp_implicit_tls" mapstructure:"smtp_implicit_tls"`                 // connect to the SMTP server over TLS (SMTPS); always on for port 465
	StrictFeatures         bool                         `json:"strict_features" mapstructure:"strict_features"`                     // return ErrUnsupportedFeature instead of logging a warning when a provider does not support a requested feature
}

// StartUp is fired once to load the email service.
//
// A provider is loaded for every service with credentials configured. Calling
// StartUp again is safe, and a provider already registered with
// RegisterProvider (ie: a custom client or a test fake) is kept.
func (m *MailService) StartUp() (err error) {
	// Required to have user and domain
	if len(m.FromUsername) == 0 {
		return ErrMissingFromUsername
	} else if len(m.FromDomain) == 0 {
		return ErrMissingFromDomain
	}

	// Set any defaults (only when a cap was not explicitly configured)
	if m.MaxToRecipients == 0 {
		m.MaxToRecipients = maxToRecipients
	}
	if m.MaxCcRecipients == 0 {
		m.MaxCcRecipients = maxCcRecipients
	}
	if m.MaxBccRecipients == 0 {
		m.MaxBccRecipients = maxBccRecipients
	}

	// Load a provider for every service with credentials configured
	if err = m.loadProviders(); err != nil {
		return err
	}

	// No service providers found
	if len(m.providers) == 0 {
		return ErrNoServiceProvider
	}
	return nil
}

// RegisterProvider registers a provider under the given ID, replacing any
// provider already registered under it, and adds the ID to AvailableProviders.
//
// Use it to add a custom provider (with your own ServiceProvider value), to
// supply a built-in provider with a custom client (ie: a SendGrid client with
// EU data residency), or to replace a provider with a fake in tests. Register
// providers before sending; registration is not safe to run concurrently with
// sends.
func (m *MailService) RegisterProvider(id ServiceProvider, provider Provider) error {
	if provider == nil {
		return ErrNilProvider
	}
	m.addProvider(id, provider)
	return nil
}

// MarshalJSON marshals the service with every secret redacted
func (m MailService) MarshalJSON() ([]byte, error) {
	type redactedMailService MailService
	return json.Marshal(redactedMailService(m.redacted()))
}

// String returns the service with every secret redacted (used by %v and %+v)
func (m MailService) String() string {
	type redactedMailService MailService
	return fmt.Sprintf("%+v", redactedMailService(m.redacted()))
}

// GoString returns the service with every secret redacted (used by %#v)
func (m MailService) GoString() string {
	type redactedMailService MailService
	return fmt.Sprintf("%#v", redactedMailService(m.redacted()))
}

// loadProviders loads a built-in provider for every service with credentials
// configured, skipping any service that already has a registered provider
func (m *MailService) loadProviders() error {
	loaders := []struct {
		load    func() (Provider, error)
		id      ServiceProvider
		enabled bool
	}{
		{id: Mandrill, enabled: len(m.MandrillAPIKey) > 0, load: m.newMandrillProvider},
		{id: AwsSes, enabled: m.awsSesStaticCredentials() || m.AwsSesUseIAMRole, load: m.newSESProvider},
		{id: Postmark, enabled: len(m.PostmarkServerToken) > 0, load: func() (Provider, error) {
			return NewPostmarkProvider(postmark.NewClient(m.PostmarkServerToken, "")), nil
		}},
		{id: SMTP, enabled: len(m.SMTPHost) > 0, load: func() (Provider, error) { // credentials are optional (ie: a local relay)
			return NewSMTPProvider(SMTPConfig{
				Host:        m.SMTPHost,
				ImplicitTLS: m.SMTPImplicitTLS,
				Password:    m.SMTPPassword,
				Port:        m.SMTPPort,
				Username:    m.SMTPUsername,
			}), nil
		}},
		{id: SendGrid, enabled: len(m.SendGridAPIKey) > 0, load: func() (Provider, error) {
			return NewSendGridProvider(sendgrid.NewSendClient(m.SendGridAPIKey)), nil
		}},
		{id: Resend, enabled: len(m.ResendAPIKey) > 0, load: func() (Provider, error) {
			return NewResendProvider(resend.NewClient(m.ResendAPIKey).Emails), nil
		}},
	}

	for _, loader := range loaders {
		if !loader.enabled || m.hasProvider(loader.id) {
			continue
		}
		provider, err := loader.load()
		if err != nil {
			return err
		}
		m.addProvider(loader.id, provider)
	}
	return nil
}

// newMandrillProvider builds the Mandrill provider, bounding requests by the send timeout
func (m *MailService) newMandrillProvider() (Provider, error) {
	api, err := gochimp.NewMandrill(m.MandrillAPIKey)
	if err != nil {
		return nil, err
	}
	api.Timeout = m.sendTimeout()
	return NewMandrillProvider(api), nil
}

// newSESProvider builds the AWS SES provider
func (m *MailService) newSESProvider() (Provider, error) {
	client, err := m.loadAwsSesClient(m.awsSesStaticCredentials())
	if err != nil {
		return nil, err
	}
	return NewSESProvider(client, m.AwsSesConfigurationSet), nil
}

// awsSesStaticCredentials reports whether a static AWS access key pair is configured
func (m *MailService) awsSesStaticCredentials() bool {
	return len(m.AwsSesAccessID) > 0 && len(m.AwsSesSecretKey) > 0
}

// hasProvider reports whether a provider is registered under the ID
func (m *MailService) hasProvider(id ServiceProvider) bool {
	_, ok := m.providers[id]
	return ok
}

// addProvider registers the provider and records it as available
func (m *MailService) addProvider(id ServiceProvider, provider Provider) {
	if m.providers == nil {
		m.providers = make(map[ServiceProvider]Provider)
	}
	m.providers[id] = provider
	if !containsServiceProvider(m.AvailableProviders, id) {
		m.AvailableProviders = append(m.AvailableProviders, id)
	}
}

// loadAwsSesClient builds the AWS SES client for StartUp. When staticCreds is
// true the configured access id and secret key are used; otherwise credentials
// are resolved from the AWS default credential chain (environment, shared
// config, web identity, or an ECS/EC2/Lambda IAM role). A custom endpoint is
// applied when AwsSesEndpoint is set.
func (m *MailService) loadAwsSesClient(staticCreds bool) (*ses.Client, error) {
	// Set the region (default to us-east-1 if not provided)
	region := cmp.Or(m.AwsSesRegion, awsSesDefaultRegion)

	// Always set the region; add static credentials only when supplied
	awsOptions := []func(*config.LoadOptions) error{config.WithRegion(region)}
	if staticCreds {
		awsOptions = append(awsOptions, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(m.AwsSesAccessID, m.AwsSesSecretKey, ""),
		))
	}

	// Load the AWS config
	awsConfig, err := config.LoadDefaultConfig(context.Background(), awsOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create the SES client, applying a custom endpoint when provided
	var optFns []func(*ses.Options)
	if len(m.AwsSesEndpoint) > 0 {
		endpoint := m.AwsSesEndpoint
		optFns = append(optFns, func(o *ses.Options) {
			o.BaseEndpoint = &endpoint
		})
	}

	return ses.NewFromConfig(awsConfig, optFns...), nil
}

// sendTimeout returns the timeout for one provider send (0 means none)
func (m *MailService) sendTimeout() time.Duration {
	switch {
	case m.SendTimeout < 0:
		return 0
	case m.SendTimeout == 0:
		return defaultSendTimeout
	default:
		return m.SendTimeout
	}
}

// maxAttachmentSize returns the attachment size limit (0 means none)
func (m *MailService) maxAttachmentSize() int64 {
	switch {
	case m.MaxAttachmentSize < 0:
		return 0
	case m.MaxAttachmentSize == 0:
		return defaultMaxAttachmentSize
	default:
		return m.MaxAttachmentSize
	}
}

// logger returns the configured logger, or the default logger
func (m *MailService) logger() *slog.Logger {
	if m.Logger != nil {
		return m.Logger
	}
	return slog.Default()
}

// redacted returns a copy of the service with every secret replaced
func (m *MailService) redacted() MailService {
	c := *m
	for _, secret := range []*string{
		&c.AwsSesSecretKey, &c.MandrillAPIKey, &c.PostmarkServerToken,
		&c.ResendAPIKey, &c.SMTPPassword, &c.SendGridAPIKey,
	} {
		if len(*secret) > 0 {
			*secret = redactedValue
		}
	}
	return c
}

// containsServiceProvider is a simple lookup for a service provider in a list of providers
func containsServiceProvider(s []ServiceProvider, e ServiceProvider) bool {
	return slices.Contains(s, e)
}
