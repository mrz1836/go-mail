package gomail

import (
	"fmt"
	"net/smtp"

	"github.com/mattbaird/gochimp"
	"github.com/mrz1836/go-ses"
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
)

const (
	awsSesDefaultEndpoint = "https://email.us-east-1.amazonaws.com"
	awsSesDefaultRegion   = "us-east-1"
	maxBccRecipients      = 50
	maxCcRecipients       = 50
	maxToRecipients       = 50
)

// MailService is the configuration to use for loading the service and provider's clients
//
// DO NOT CHANGE ORDER - Optimized for memory (maligned)
type MailService struct {
	awsConfig           ses.Config        // AWS SES config
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
	awsSesService       awsSesInterface   // AWS SES client
	mandrillService     mandrillInterface // Mandrill api client
	postmarkService     postmarkInterface // Postmark api client
	smtpAuth            smtp.Auth         // Auth credentials for SMTP
	smtpClient          smtpInterface     // SMTP client
	SMTPUsername        string            `json:"smtp_username" mapstructure:"smtp_username"`           // ie: testuser
	MaxBccRecipients    int               `json:"max_bcc_recipients" mapstructure:"max_bcc_recipients"` // max amount for BCC
	MaxCcRecipients     int               `json:"max_cc_recipients" mapstructure:"max_cc_recipients"`   // max amount for CC
	MaxToRecipients     int               `json:"max_to_recipients" mapstructure:"max_to_recipients"`   // max amount for TO
	SMTPPort            int               `json:"smtp_port" mapstructure:"smtp_port"`                   // ie: 25
	AutoText            bool              `json:"auto_text" mapstructure:"auto_text"`                   // whether or not to automatically generate a text part for messages that are not given text
	Important           bool              `json:"important" mapstructure:"important"`                   // whether or not this message is important, and should be delivered ahead of non-important messages
	TrackClicks         bool              `json:"track_clicks" mapstructure:"track_clicks"`             // whether or not to turn on click tracking for the message
	TrackOpens          bool              `json:"track_opens" mapstructure:"track_opens"`               // whether or not to turn on open tracking for the message
}

// StartUp is fired once to load the email service
func (m *MailService) StartUp() (err error) {

	// Required to have user and domain
	if len(m.FromUsername) == 0 {
		err = fmt.Errorf("missing required field: from_username")
		return
	} else if len(m.FromDomain) == 0 {
		err = fmt.Errorf("missing required field: from_domain")
		return
	}

	// Set any defaults
	m.awsConfig.Endpoint = awsSesDefaultEndpoint
	m.awsConfig.Region = awsSesDefaultRegion
	m.MaxToRecipients = maxToRecipients
	m.MaxCcRecipients = maxCcRecipients
	m.MaxBccRecipients = maxBccRecipients

	// If the key is set, try loading the service
	if len(m.MandrillAPIKey) > 0 {

		// Never will return an error - set new MandrillApi
		m.mandrillService, _ = gochimp.NewMandrill(m.MandrillAPIKey)

		// Add to the list of available providers
		m.AvailableProviders = append(m.AvailableProviders, Mandrill)
	}

	// If the AWS SES credentials exist
	if len(m.AwsSesAccessID) > 0 && len(m.AwsSesSecretKey) > 0 {

		// Set the credentials
		m.awsConfig.AccessKeyID = m.AwsSesAccessID
		m.awsConfig.SecretAccessKey = m.AwsSesSecretKey
		if len(m.AwsSesEndpoint) > 0 {
			m.awsConfig.Endpoint = m.AwsSesEndpoint
		}
		if len(m.AwsSesRegion) > 0 {
			m.awsConfig.Region = m.AwsSesRegion
		}

		// Use the ses.Config
		m.awsSesService = &m.awsConfig

		// Add to the list of available providers
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

		// Create a new client from the connection string
		m.smtpClient = newSMTPClient(fmt.Sprintf("%s:%d", m.SMTPHost, m.SMTPPort), m.smtpAuth)

		// Add to the list of available providers
		m.AvailableProviders = append(m.AvailableProviders, SMTP)
	}

	// No service providers found
	if len(m.AvailableProviders) == 0 {
		err = fmt.Errorf("attempted to startup the email service provider(s) however there's no available service provider")
	}

	return
}

// containsServiceProvider is a simple lookup for a service provider in a list of providers
func containsServiceProvider(s []ServiceProvider, e ServiceProvider) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
