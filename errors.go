package gomail

import "errors"

// Package-level sentinel errors to satisfy err113 linter rule
var (
	// Config validation errors
	ErrMissingFromUsername = errors.New("missing required field: from_username")
	ErrMissingFromDomain   = errors.New("missing required field: from_domain")
	ErrNoServiceProvider   = errors.New("attempted to startup the email service provider(s) however there's no available service provider")

	// Email validation errors
	ErrMissingSubject          = errors.New("email is missing a subject")
	ErrMissingContent          = errors.New("email is missing content (plain & html)")
	ErrMissingRecipient        = errors.New("email is missing a recipient")
	ErrInvalidFromAddress      = errors.New("invalid FromAddress, domain not found")
	ErrProviderNotFound        = errors.New("service provider was not in the list of available service providers, email not sent")
	ErrMaxToRecipientsReached  = errors.New("max TO recipient limit reached")
	ErrMaxCcRecipientsReached  = errors.New("max CC recipient limit reached")
	ErrMaxBccRecipientsReached = errors.New("max BCC recipient limit reached")
	ErrInvalidAWSResponse      = errors.New("aws ses did not return expected valid response")
	ErrMessageNotSent          = errors.New("message status and not sent")
	ErrPostmarkError           = errors.New("error from postmark")
	ErrSendGridError           = errors.New("error from sendgrid")
)
