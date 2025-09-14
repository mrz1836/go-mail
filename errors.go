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

	// Test-specific errors
	ErrMissingEmailContents = errors.New("missing email contents")
	ErrBadHostname          = errors.New("bad hostname error")
	ErrSMTPAuth             = errors.New("535 5.7.8")
	ErrDNSLookup            = errors.New("dial tcp: lookup smtp.badhostname.com: no such host")
	ErrInvalidAPIKey        = errors.New("-1: Invalid API key")
	ErrValidationError      = errors.New(`-2: Validation error: {"message":{"from_email":"The domain portion of the email address is invalid (the portion after the @: badhostname.com)"}}`)
	ErrPostmarkFromError    = errors.New("400 The 'From' address you supplied is not a Sender Signature on your account")
	ErrPostmarkTokenError   = errors.New("10 The Server Token you provided in the X-Postmark-Server-Token request header was invalid")
	ErrAWSServiceError      = errors.New("AWS SES service error")
)
