package gomail

import (
	"fmt"

	"github.com/mattbaird/gochimp"
	"github.com/mrz1836/go-logger"
	"github.com/mrz1836/postmark"
	"github.com/sourcegraph/go-ses"
)

// Package constants
const (
	// Email Service Providers
	AwsSes   ServiceProvider = iota // AWS SES Service
	Mandrill                        // Mandrill Email Service
	Postmark                        // Postmark Email Service
)

const (
	// awsSesDefaultEndpoint default endpoint for AWS SES
	awsSesDefaultEndpoint = "https://email.us-east-1.amazonaws.com"
	maxToRecipients       = 50
	maxCcRecipients       = 50
	maxBccRecipients      = 50
)

// ServiceProvider is the provider
type ServiceProvider int

// MailService is the email configuration to use for loading the service
type MailService struct {
	AutoText            bool              `json:"auto_text" mapstructure:"auto_text"`                         // whether or not to automatically generate a text part for messages that are not given text
	AvailableProviders  []ServiceProvider `json:"available_providers" mapstructure:"available_providers"`     // list of providers that loaded successfully
	AwsSesAccessID      string            `json:"aws_ses_access_id" mapstructure:"aws_ses_access_id"`         // aws iam access id for ses service
	AwsSesEndpoint      string            `json:"aws_ses_endpoint" mapstructure:"aws_ses_endpoint"`           // ie: https://email.us-east-1.amazonaws.com
	AwsSesSecretKey     string            `json:"aws_ses_secret_key" mapstructure:"aws_ses_secret_key"`       // aws iam secret key for corresponding access id
	FromDomain          string            `json:"from_domain" mapstructure:"from_domain"`                     // ie: example.com
	FromName            string            `json:"from_name" mapstructure:"from_name"`                         // ie: No Reply
	FromUsername        string            `json:"from_username" mapstructure:"from_username"`                 // ie: no-reply
	Important           bool              `json:"important" mapstructure:"important"`                         // whether or not this message is important, and should be delivered ahead of non-important messages
	MandrillAPIKey      string            `json:"mandrill_api_key" mapstructure:"mandrill_api_key"`           // mandrill api key
	PostmarkServerToken string            `json:"postmark_server_token" mapstructure:"postmark_server_token"` // ie: abc123...
	TrackClicks         bool              `json:"track_clicks" mapstructure:"track_clicks"`                   // whether or not to turn on click tracking for the message
	TrackOpens          bool              `json:"track_opens" mapstructure:"track_opens"`                     // whether or not to turn on open tracking for the message

	mandrillService *gochimp.MandrillAPI // internal mandrill api service
	awsSesService   ses.Config           // internal AWS SES api service
	postmarkService *postmark.Client     // internal postmark api service
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

// StartUp is fired once to load the email service
func (m *MailService) StartUp() {

	// Check the basics
	if len(m.FromUsername) == 0 {
		logger.Fatalln("missing required from_username field")
	} else if len(m.FromDomain) == 0 {
		logger.Fatalln("missing required from_domain field")
	} else if len(m.FromName) == 0 {
		logger.Fatalln("missing required from_name field")
	}

	// If the key is set, try loading the service
	if len(m.MandrillAPIKey) == 0 {
		logger.Data(2, logger.DEBUG, "mandrill credentials not set, skipping mandrill...")
	} else {
		var err error
		m.mandrillService, err = gochimp.NewMandrill(m.MandrillAPIKey)
		if err != nil {
			logger.Fatalln(fmt.Sprintf("error instantiating Mandrill API: %s", err))
		}

		// Add to the list of available providers
		m.AvailableProviders = append(m.AvailableProviders, Mandrill)
	}

	// Set defaults
	m.awsSesService.Endpoint = awsSesDefaultEndpoint

	// If the AWS SES credentials exist
	if len(m.AwsSesAccessID) == 0 || len(m.AwsSesSecretKey) == 0 {
		logger.Data(2, logger.DEBUG, "aws ses credentials not set, skipping ses...")
	} else {

		// Set the credentials
		m.awsSesService.AccessKeyID = m.AwsSesAccessID
		m.awsSesService.SecretAccessKey = m.AwsSesSecretKey
		if len(m.AwsSesEndpoint) > 0 {
			m.awsSesService.Endpoint = m.AwsSesEndpoint
		}

		// Add to the list of available providers
		m.AvailableProviders = append(m.AvailableProviders, AwsSes)
	}

	// If the Postmark credentials exist
	if len(m.PostmarkServerToken) == 0 {
		logger.Data(2, logger.DEBUG, "postmark credentials not set, skipping postmark...")
	} else {
		m.postmarkService = postmark.NewClient(m.PostmarkServerToken, "")

		// Add to the list of available providers
		m.AvailableProviders = append(m.AvailableProviders, Postmark)
	}

	// No service providers found
	if len(m.AvailableProviders) == 0 {
		logger.Fatalln("attempted to startup the email service however there's no available service provider")
	}
}
