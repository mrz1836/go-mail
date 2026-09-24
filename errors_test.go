package gomail

import "errors"

// Test-specific sentinel errors used only by the *_test.go mocks and assertions.
// These are intentionally kept out of the public API (errors.go) since they are
// pure test fixtures.
var (
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
