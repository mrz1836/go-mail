package gomail

import (
	"cmp"
	"context"
	"fmt"
	"net"
	"net/smtp"
	"slices"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/mattbaird/gochimp"
	"github.com/mrz1836/postmark"
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
	default:
		return "Unknown"
	}
}

const (
	awsSesDefaultEndpoint = "https://email.us-east-1.amazonaws.com"
	awsSesDefaultRegion   = "us-east-1"
	maxBccRecipients      = 50
	maxCcRecipients       = 50
	maxToRecipients       = 50

	// Importance headers applied when an email is marked as important
	headerXPriority       = "X-Priority"
	headerXPriorityValue  = "1 (Highest)"
	headerXMSMailPriority = "X-MSMail-Priority"
	headerImportance      = "Importance"
	headerHighValue       = "High"
)

// MailService is the configuration to use for loading the service and provider's clients
//
// DO NOT CHANGE ORDER - Optimized for memory (maligned)
type MailService struct {
	AvailableProviders  []ServiceProvider `json:"available_providers" mapstructure:"available_providers"`     // list of providers that loaded successfully
	EmailCSS            []byte            `json:"email_css" mapstructure:"email_css"`                         // default css pre-parsed into bytes
	AwsSesAccessID      string            `json:"aws_ses_access_id" mapstructure:"aws_ses_access_id"`         // aws iam access id for ses service
	AwsSesEndpoint      string            `json:"aws_ses_endpoint" mapstructure:"aws_ses_endpoint"`           // ie: https://email.us-east-1.amazonaws.com
	AwsSesSecretKey     string            `json:"aws_ses_secret_key" mapstructure:"aws_ses_secret_key"`       // aws iam secret key for corresponding access id
	AwsSesRegion        string            `json:"aws_ses_region" mapstructure:"aws_ses_region"`               // AWS region
	FromDomain          string            `json:"from_domain" mapstructure:"from_domain"`                     // ie: example.com
	FromName            string            `json:"from_name" mapstructure:"from_name"`                         // ie: No Reply
	FromUsername        string            `json:"from_username" mapstructure:"from_username"`                 // ie: no-reply
	MandrillAPIKey      string            `json:"mandrill_api_key" mapstructure:"mandrill_api_key"`           // mandrill api key
	PostmarkServerToken string            `json:"postmark_server_token" mapstructure:"postmark_server_token"` // ie: abc123...
	SMTPHost            string            `json:"smtp_host" mapstructure:"smtp_host"`                         // ie: example.com
	SMTPPassword        string            `json:"smtp_password" mapstructure:"smtp_password"`                 // ie: secretPassword
	SendGridAPIKey      string            `json:"sendgrid_api_key" mapstructure:"sendgrid_api_key"`           // sendgrid api key
	awsSesService       awsSesInterface   // AWS SES client
	mandrillService     mandrillInterface // Mandrill api client
	postmarkService     postmarkInterface // Postmark api client
	sendGridService     sendGridInterface // SendGrid api client
	smtpAddr            string            // SMTP server address (host:port)
	smtpAuth            smtp.Auth         // Auth credentials for SMTP
	smtpClientFactory   smtpClientFactory // Builds the SMTP client for each send (nil uses newSMTPClient)
	SMTPUsername        string            `json:"smtp_username" mapstructure:"smtp_username"`               // ie: testuser
	MaxBccRecipients    int               `json:"max_bcc_recipients" mapstructure:"max_bcc_recipients"`     // max amount for BCC
	MaxCcRecipients     int               `json:"max_cc_recipients" mapstructure:"max_cc_recipients"`       // max amount for CC
	MaxToRecipients     int               `json:"max_to_recipients" mapstructure:"max_to_recipients"`       // max amount for TO
	SMTPPort            int               `json:"smtp_port" mapstructure:"smtp_port"`                       // ie: 25
	AutoText            bool              `json:"auto_text" mapstructure:"auto_text"`                       // whether to automatically generate a text part for messages that are not given text
	Important           bool              `json:"important" mapstructure:"important"`                       // whether this message is important, and should be delivered ahead of non-important messages
	TrackClicks         bool              `json:"track_clicks" mapstructure:"track_clicks"`                 // whether to turn on click tracking for the message
	TrackOpens          bool              `json:"track_opens" mapstructure:"track_opens"`                   // whether to turn on open tracking for the message
	AwsSesUseIAMRole    bool              `json:"aws_ses_use_iam_role" mapstructure:"aws_ses_use_iam_role"` // load AWS SES using the default credential chain (IAM role) instead of static access keys
}

// StartUp is fired once to load the email service
func (m *MailService) StartUp() (err error) {
	// Required to have user and domain
	if len(m.FromUsername) == 0 {
		err = ErrMissingFromUsername
		return err
	} else if len(m.FromDomain) == 0 {
		err = ErrMissingFromDomain
		return err
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

	// If the key is set, try loading the service
	if len(m.MandrillAPIKey) > 0 {

		// Will Never return an error - set new MandrillApi
		m.mandrillService, _ = gochimp.NewMandrill(m.MandrillAPIKey)

		// Add to the list of available providers
		m.AvailableProviders = append(m.AvailableProviders, Mandrill)
	}

	// If the AWS SES static credentials exist, or the default credential chain is enabled
	awsSesStaticCreds := len(m.AwsSesAccessID) > 0 && len(m.AwsSesSecretKey) > 0
	if awsSesStaticCreds || m.AwsSesUseIAMRole {

		// Build the SES client (static credentials or the default chain)
		svc, sesErr := m.loadAwsSesService(awsSesStaticCreds)
		if sesErr != nil {
			return sesErr
		}

		// Register the loaded provider
		m.awsSesService = svc
		m.AvailableProviders = append(m.AvailableProviders, AwsSes)
	}

	// If the Postmark credentials exist
	if len(m.PostmarkServerToken) > 0 {
		m.postmarkService = postmark.NewClient(m.PostmarkServerToken, "")

		// Add to the list of available providers
		m.AvailableProviders = append(m.AvailableProviders, Postmark)
	}

	// If the smtp credentials exist
	if len(m.SMTPHost) > 0 && len(m.SMTPUsername) > 0 && len(m.SMTPPassword) > 0 {

		// Set the credentials
		m.smtpAuth = smtp.PlainAuth("", m.SMTPUsername, m.SMTPPassword, m.SMTPHost)

		// Store the connection string; a new client is built for every send
		m.smtpAddr = net.JoinHostPort(m.SMTPHost, strconv.Itoa(m.SMTPPort))

		// Add to the list of available providers
		m.AvailableProviders = append(m.AvailableProviders, SMTP)
	}

	// If the SendGrid api key is set, load the service
	if len(m.SendGridAPIKey) > 0 {
		m.sendGridService = newSendGridClient(m.SendGridAPIKey)

		// Add to the list of available providers
		m.AvailableProviders = append(m.AvailableProviders, SendGrid)
	}

	// No service providers found
	if len(m.AvailableProviders) == 0 {
		err = ErrNoServiceProvider
	}

	return err
}

// loadAwsSesService builds the AWS SES client for StartUp. When staticCreds is
// true the configured access id and secret key are used; otherwise credentials
// are resolved from the AWS default credential chain (environment, shared
// config, web identity, or an ECS/EC2/Lambda IAM role). A custom endpoint is
// applied when AwsSesEndpoint is set.
func (m *MailService) loadAwsSesService(staticCreds bool) (awsSesInterface, error) {
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
	awsConfig, err := config.LoadDefaultConfig(context.TODO(), awsOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create the SES client, applying a custom endpoint when provided
	var optFns []func(*ses.Options)
	if len(m.AwsSesEndpoint) > 0 {
		optFns = append(optFns, func(o *ses.Options) {
			o.BaseEndpoint = &m.AwsSesEndpoint
		})
	}

	return &awsSesSdkV2Client{client: ses.NewFromConfig(awsConfig, optFns...)}, nil
}

// containsServiceProvider is a simple lookup for a service provider in a list of providers
func containsServiceProvider(s []ServiceProvider, e ServiceProvider) bool {
	return slices.Contains(s, e)
}
