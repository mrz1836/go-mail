/*
Package main is examples using the go-mail package
*/
package main

import (
	"context"
	"log"
	"os"
	"strconv"

	"github.com/mrz1836/postmark"
	sgmail "github.com/sendgrid/sendgrid-go/helpers/mail"

	gomail "github.com/mrz1836/go-mail"
)

func main() {
	// Run the AWS SES example
	awsSesExample()

	// Run the AWS SES example using the default credential chain (e.g. an IAM role)
	// awsSesIAMRoleExample()

	// Run the Mandrill example
	// mandrillExample()

	// Run the Postmark example
	// postmarkExample()

	// Run the SMTP example
	// smtpExample()

	// Run the SendGrid example
	// sendGridExample()

	// Run the Resend example
	// resendExample()

	// Example using ALL options available
	// allOptionsExample()

	// Example using provider-specific features, failover and the send result
	// advancedExample()
}

// awsSesExample shows an example using AWS SES as the provider
func awsSesExample() {
	// Config
	mail := new(gomail.MailService)
	mail.FromName = "No Reply"
	mail.FromUsername = "no-reply"
	mail.FromDomain = os.Getenv("EMAIL_FROM_DOMAIN")
	if len(mail.FromDomain) == 0 {
		log.Fatal("missing env: EMAIL_FROM_DOMAIN")
	}

	// Set the to field
	toRecipients := os.Getenv("EMAIL_TEST_TO_RECIPIENT")
	if len(toRecipients) == 0 {
		log.Fatal("missing env: EMAIL_TEST_TO_RECIPIENT")
	}

	// Provider
	mail.AwsSesAccessID = os.Getenv("EMAIL_AWS_SES_ACCESS_ID")
	mail.AwsSesSecretKey = os.Getenv("EMAIL_AWS_SES_SECRET_KEY")
	if len(mail.AwsSesAccessID) == 0 {
		log.Fatal("missing env: EMAIL_AWS_SES_ACCESS_ID")
	}
	if len(mail.AwsSesSecretKey) == 0 {
		log.Fatal("missing env: EMAIL_AWS_SES_SECRET_KEY")
	}
	provider := gomail.AwsSes

	// Start the service
	err := mail.StartUp()
	if err != nil {
		log.Printf("error in StartUp: %s using provider: %s", err.Error(), provider)
	}

	// Create and send a basic email
	email := mail.NewEmail()
	email.HTMLContent = "<html><body>This is a <b>go-mail</b> example email using <i>HTML</i></body></html>"
	email.Recipients = []string{toRecipients}
	email.Subject = "example go-mail email using AWS SES"

	// Send the email
	if err = mail.SendEmail(context.Background(), email, provider); err != nil {
		log.Fatalf("error in SendEmail: %s using provider: %s", err.Error(), provider)
	}
	log.Printf("email sent!")
}

// awsSesIAMRoleExample shows an example using AWS SES with the default
// credential chain (for example an IAM role on Lambda, ECS, or EC2) instead of
// static access keys
func awsSesIAMRoleExample() { //nolint:unused // this is an example function

	// Config
	mail := new(gomail.MailService)
	mail.FromName = "No Reply"
	mail.FromUsername = "no-reply"
	mail.FromDomain = os.Getenv("EMAIL_FROM_DOMAIN")
	if len(mail.FromDomain) == 0 {
		log.Fatal("missing env: EMAIL_FROM_DOMAIN")
	}

	// Set the to field
	toRecipients := os.Getenv("EMAIL_TEST_TO_RECIPIENT")
	if len(toRecipients) == 0 {
		log.Fatal("missing env: EMAIL_TEST_TO_RECIPIENT")
	}

	// Provider: no static keys — credentials come from the default chain
	mail.AwsSesUseIAMRole = true
	mail.AwsSesRegion = os.Getenv("EMAIL_AWS_SES_REGION")
	provider := gomail.AwsSes

	// Start the service
	err := mail.StartUp()
	if err != nil {
		log.Printf("error in StartUp: %s using provider: %s", err.Error(), provider)
	}

	// Create and send a basic email
	email := mail.NewEmail()
	email.HTMLContent = "<html><body>This is a <b>go-mail</b> example email using <i>HTML</i></body></html>"
	email.Recipients = []string{toRecipients}
	email.Subject = "example go-mail email using AWS SES (IAM role)"

	// Send the email
	if err = mail.SendEmail(context.Background(), email, provider); err != nil {
		log.Fatalf("error in SendEmail: %s using provider: %s", err.Error(), provider)
	}
	log.Printf("email sent!")
}

// mandrillExample shows an example using Mandrill as the provider
func mandrillExample() { //nolint:unused // this is an example function

	// Config
	mail := new(gomail.MailService)
	mail.FromName = "No Reply"
	mail.FromUsername = "no-reply"
	mail.FromDomain = os.Getenv("EMAIL_FROM_DOMAIN")
	if len(mail.FromDomain) == 0 {
		log.Fatal("missing env: EMAIL_FROM_DOMAIN")
	}

	// Set the to field
	toRecipients := os.Getenv("EMAIL_TEST_TO_RECIPIENT")
	if len(toRecipients) == 0 {
		log.Fatal("missing env: EMAIL_TEST_TO_RECIPIENT")
	}

	// Provider
	mail.MandrillAPIKey = os.Getenv("EMAIL_MANDRILL_KEY")
	if len(mail.MandrillAPIKey) == 0 {
		log.Fatal("missing env: EMAIL_MANDRILL_KEY")
	}
	provider := gomail.Mandrill

	// Start the service
	err := mail.StartUp()
	if err != nil {
		log.Printf("error in StartUp: %s using provider: %s", err.Error(), provider)
	}

	// Create and send a basic email
	email := mail.NewEmail()
	email.HTMLContent = "<html><body>This is a <b>go-mail</b> example email using <i>HTML</i></body></html>"
	email.Recipients = []string{toRecipients}
	email.Subject = "example go-mail email using Mandrill"

	// Send the email
	if err = mail.SendEmail(context.Background(), email, provider); err != nil {
		log.Fatalf("error in SendEmail: %s using provider: %s", err.Error(), provider)
	}
	log.Printf("email sent!")
}

// postmarkExample shows an example using Postmark as the provider
func postmarkExample() { //nolint:unused // this is an example function

	// Config
	mail := new(gomail.MailService)
	mail.FromName = "No Reply"
	mail.FromUsername = "no-reply"
	mail.FromDomain = os.Getenv("EMAIL_FROM_DOMAIN")
	if len(mail.FromDomain) == 0 {
		log.Fatal("missing env: EMAIL_FROM_DOMAIN")
	}

	// Set the to field
	toRecipients := os.Getenv("EMAIL_TEST_TO_RECIPIENT")
	if len(toRecipients) == 0 {
		log.Fatal("missing env: EMAIL_TEST_TO_RECIPIENT")
	}

	// Provider
	mail.PostmarkServerToken = os.Getenv("EMAIL_POSTMARK_SERVER_TOKEN")
	if len(mail.PostmarkServerToken) == 0 {
		log.Fatal("missing env: EMAIL_POSTMARK_SERVER_TOKEN")
	}
	provider := gomail.Postmark

	// Start the service
	err := mail.StartUp()
	if err != nil {
		log.Printf("error in StartUp: %s using provider: %s", err.Error(), provider)
	}

	// Create and send a basic email
	email := mail.NewEmail()
	email.HTMLContent = "<html><body>This is a <b>go-mail</b> example email using <i>HTML</i></body></html>"
	email.Recipients = []string{toRecipients}
	email.Subject = "example go-mail email using Postmark"

	// Send the email
	if err = mail.SendEmail(context.Background(), email, provider); err != nil {
		log.Fatalf("error in SendEmail: %s using provider: %s", err.Error(), provider)
	}
	log.Printf("email sent!")
}

// smtpExample shows an example using SMTP as the provider
func smtpExample() { //nolint:unused // this is an example function

	// Config
	mail := new(gomail.MailService)
	mail.FromName = "No Reply"
	mail.FromUsername = "no-reply"
	mail.FromDomain = os.Getenv("EMAIL_FROM_DOMAIN")
	if len(mail.FromDomain) == 0 {
		log.Fatal("missing env: EMAIL_FROM_DOMAIN")
	}

	// Set the to field
	toRecipients := os.Getenv("EMAIL_TEST_TO_RECIPIENT")
	if len(toRecipients) == 0 {
		log.Fatal("missing env: EMAIL_TEST_TO_RECIPIENT")
	}

	// Provider
	mail.SMTPHost = os.Getenv("EMAIL_SMTP_HOST")
	mail.SMTPPort, _ = strconv.Atoi(os.Getenv("EMAIL_SMTP_PORT"))
	mail.SMTPUsername = os.Getenv("EMAIL_SMTP_USERNAME")
	mail.SMTPPassword = os.Getenv("EMAIL_SMTP_PASSWORD")
	if len(mail.SMTPHost) == 0 {
		log.Fatal("missing env: EMAIL_SMTP_HOST")
	}
	// The port defaults to 587 (STARTTLS); use 465 for implicit TLS. Leave the
	// username empty for a relay that does not require authentication.
	provider := gomail.SMTP

	// Start the service
	err := mail.StartUp()
	if err != nil {
		log.Printf("error in StartUp: %s using provider: %s", err.Error(), provider)
	}

	// Create and send a basic email
	email := mail.NewEmail()
	email.HTMLContent = "<html><body>This is a <b>go-mail</b> example email using <i>HTML</i></body></html>"
	email.Recipients = []string{toRecipients}
	email.Subject = "example go-mail email using SMTP"

	// Send the email
	if err = mail.SendEmail(context.Background(), email, provider); err != nil {
		log.Fatalf("error in SendEmail: %s using provider: %s", err.Error(), provider)
	}
	log.Printf("email sent!")
}

// sendGridExample shows an example using SendGrid as the provider
func sendGridExample() { //nolint:unused // this is an example function

	// Config
	mail := new(gomail.MailService)
	mail.FromName = "No Reply"
	mail.FromUsername = "no-reply"
	mail.FromDomain = os.Getenv("EMAIL_FROM_DOMAIN")
	if len(mail.FromDomain) == 0 {
		log.Fatal("missing env: EMAIL_FROM_DOMAIN")
	}

	// Set the to field
	toRecipients := os.Getenv("EMAIL_TEST_TO_RECIPIENT")
	if len(toRecipients) == 0 {
		log.Fatal("missing env: EMAIL_TEST_TO_RECIPIENT")
	}

	// Provider
	mail.SendGridAPIKey = os.Getenv("EMAIL_SENDGRID_API_KEY")
	if len(mail.SendGridAPIKey) == 0 {
		log.Fatal("missing env: EMAIL_SENDGRID_API_KEY")
	}
	provider := gomail.SendGrid

	// Start the service
	err := mail.StartUp()
	if err != nil {
		log.Printf("error in StartUp: %s using provider: %s", err.Error(), provider)
	}

	// Create and send a basic email
	email := mail.NewEmail()
	email.HTMLContent = "<html><body>This is a <b>go-mail</b> example email using <i>HTML</i></body></html>"
	email.Recipients = []string{toRecipients}
	email.Subject = "example go-mail email using SendGrid"

	// SendGrid supports open & click tracking natively
	email.TrackClicks = true
	email.TrackOpens = true

	// Send the email
	if err = mail.SendEmail(context.Background(), email, provider); err != nil {
		log.Fatalf("error in SendEmail: %s using provider: %s", err.Error(), provider)
	}
	log.Printf("email sent!")
}

// resendExample shows an example using Resend as the provider
func resendExample() { //nolint:unused // this is an example function

	// Config
	mail := new(gomail.MailService)
	mail.FromName = "No Reply"
	mail.FromUsername = "no-reply"
	mail.FromDomain = os.Getenv("EMAIL_FROM_DOMAIN") // must be a domain verified in Resend
	if len(mail.FromDomain) == 0 {
		log.Fatal("missing env: EMAIL_FROM_DOMAIN")
	}

	// Set the to field
	toRecipients := os.Getenv("EMAIL_TEST_TO_RECIPIENT")
	if len(toRecipients) == 0 {
		log.Fatal("missing env: EMAIL_TEST_TO_RECIPIENT")
	}

	// Provider
	mail.ResendAPIKey = os.Getenv("EMAIL_RESEND_API_KEY")
	if len(mail.ResendAPIKey) == 0 {
		log.Fatal("missing env: EMAIL_RESEND_API_KEY")
	}
	provider := gomail.Resend

	// Start the service
	err := mail.StartUp()
	if err != nil {
		log.Printf("error in StartUp: %s using provider: %s", err.Error(), provider)
	}

	// Create and send a basic email
	email := mail.NewEmail()
	email.HTMLContent = "<html><body>This is a <b>go-mail</b> example email using <i>HTML</i></body></html>"
	email.Recipients = []string{toRecipients}
	email.Subject = "example go-mail email using Resend"
	email.Tags = []string{"example"}

	// Resend configures open & click tracking per domain (in the Resend dashboard), not per email
	email.TrackClicks = false
	email.TrackOpens = false

	// Send the email
	if err = mail.SendEmail(context.Background(), email, provider); err != nil {
		log.Fatalf("error in SendEmail: %s using provider: %s", err.Error(), provider)
	}
	log.Printf("email sent!")
}

// allOptionsExample is using the most number of options/features
func allOptionsExample() { //nolint:unused // this is an example function

	// Define your service configuration
	mail := new(gomail.MailService)
	mail.FromName = "No Reply"
	mail.FromUsername = "no-reply"
	mail.FromDomain = os.Getenv("EMAIL_FROM_DOMAIN") // example.com

	// Mandrill
	mail.MandrillAPIKey = os.Getenv("EMAIL_MANDRILL_KEY") // aOfw3WU...

	// AWS SES
	mail.AwsSesAccessID = os.Getenv("EMAIL_AWS_SES_ACCESS_ID")   // AKIAY...
	mail.AwsSesSecretKey = os.Getenv("EMAIL_AWS_SES_SECRET_KEY") // tOpw3WU...

	// Postmark
	mail.PostmarkServerToken = os.Getenv("EMAIL_POSTMARK_SERVER_TOKEN") // AKIAY...

	// SMTP
	mail.SMTPHost = os.Getenv("EMAIL_SMTP_HOST")                  // example.com
	mail.SMTPPort, _ = strconv.Atoi(os.Getenv("EMAIL_SMTP_PORT")) // 25
	mail.SMTPUsername = os.Getenv("EMAIL_SMTP_USERNAME")          // johndoe
	mail.SMTPPassword = os.Getenv("EMAIL_SMTP_PASSWORD")          // secretPassword

	// SendGrid
	mail.SendGridAPIKey = os.Getenv("EMAIL_SENDGRID_API_KEY") // SG.xxxx...

	// Resend
	mail.ResendAPIKey = os.Getenv("EMAIL_RESEND_API_KEY") // re_xxxx...

	provider := gomail.SMTP // Other options: AwsSes Mandrill Postmark SendGrid Resend

	// Start the service
	err := mail.StartUp()
	if err != nil {
		log.Printf("error in StartUp: %s using provider: %s", err.Error(), provider)
	}

	// Available services given the config above
	log.Printf("available service providers: %v", mail.AvailableProviders)

	// Create a new email
	email := mail.NewEmail()

	email.PlainTextContent = "This is a go-mail example email using plain-text"
	email.HTMLContent = "<html><body>This is a <b>go-mail</b> example email using <i>HTML</i></body></html>"
	email.Recipients = []string{os.Getenv("EMAIL_TEST_TO_RECIPIENT")}
	email.RecipientsCc = []string{os.Getenv("EMAIL_TEST_CC_RECIPIENT")}
	email.RecipientsBcc = []string{os.Getenv("EMAIL_TEST_BCC_RECIPIENT")}
	email.Subject = "testing go-mail - example email"
	email.Tags = []string{"admin_alert"}
	email.Important = true
	email.TrackClicks = true
	email.TrackOpens = true
	email.AutoText = true

	// Add an attachment
	var f *os.File
	f, err = os.Open("test-attachment-file.txt")
	if err != nil {
		log.Printf("unable to load file for attachment")
	} else {
		email.AddAttachment("test-attachment-file.txt", "text/plain", f)
	}

	// Send the email (basic example using one provider)
	if err = mail.SendEmail(context.Background(), email, provider); err != nil {
		log.Fatalf("error in SendEmail: %s using provider: %s", err.Error(), provider)
	}

	// Congrats!
	log.Printf("all emails sent!")
}

// advancedExample shows provider-specific options, failover, and the send result
func advancedExample() { //nolint:unused // this is an example function

	// Config: two providers so one can fail over to the other
	mail := new(gomail.MailService)
	mail.FromName = "Acme, Inc."
	mail.FromUsername = "no-reply"
	mail.FromDomain = os.Getenv("EMAIL_FROM_DOMAIN")
	mail.PostmarkServerToken = os.Getenv("EMAIL_POSTMARK_SERVER_TOKEN")
	mail.SendGridAPIKey = os.Getenv("EMAIL_SENDGRID_API_KEY")
	mail.StrictFeatures = false // true returns ErrUnsupportedFeature instead of logging a warning

	// Start the service
	if err := mail.StartUp(); err != nil {
		log.Fatalf("error in StartUp: %s", err.Error())
	}

	// Create the email; recipients may include a display name
	email := mail.NewEmail()
	email.Subject = "Your weekly digest"
	email.HTMLContent = `<html><body><img src="cid:logo"> Here is your digest</body></html>`
	email.PlainTextContent = "Here is your digest"
	email.Recipients = []string{"Jane Doe <" + os.Getenv("EMAIL_TEST_TO_RECIPIENT") + ">"}
	email.Tags = []string{"digest"}
	email.Metadata = map[string]string{"user_id": "42"} // returned in provider webhooks
	email.AddInlineAttachment("logo.png", "image/png", "logo", []byte("...png bytes..."))

	// Gmail and Yahoo require one-click unsubscribe for bulk senders
	if err := email.SetListUnsubscribe(true, "https://example.com/unsubscribe?u=42"); err != nil {
		log.Fatalf("error in SetListUnsubscribe: %s", err.Error())
	}

	// Provider-specific features: each option only applies to its own provider
	email.With(
		gomail.PostmarkOption(func(e *postmark.Email) { e.MessageStream = "broadcast" }),
		gomail.SendGridOption(func(m *sgmail.SGMailV3) { m.SetASM(sgmail.NewASM().SetGroupID(123)) }),
	)

	// Try Postmark first and fail over to SendGrid
	result, err := mail.Send(context.Background(), email, gomail.Postmark, gomail.SendGrid)
	if err != nil {
		log.Fatalf("error in Send: %s", err.Error())
	}
	log.Printf("email sent via %s with message id: %s", result.Provider, result.MessageID)
}
