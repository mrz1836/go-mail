/*
Package main is examples using the go-mail package
*/
package main

import (
	"log"
	"os"
	"strconv"

	gomail "github.com/mrz1836/go-mail"
)

func main() {

	// Run the Mandrill example
	mandrillExample()

	// Run the Postmark example
	// postmarkExample()

	// Run the SMTP example
	// smtpExample()

	// Run the AWS SES example
	// awsSesExample()

	// Example using ALL options available
	// allOptionsExample()
}

// mandrillExample shows an example using Mandrill as the provider
func mandrillExample() {

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
		log.Printf("error in StartUp: %s using provider: %x", err.Error(), provider)
	}

	// Create and send a basic email
	email := mail.NewEmail()
	email.HTMLContent = "<html><body>This is a <b>go-mail</b> example email using <i>HTML</i></body></html>"
	email.Recipients = []string{toRecipients}
	email.Subject = "example go-mail email using Mandrill"

	// Send the email
	if err = mail.SendEmail(email, provider); err != nil {
		log.Fatalf("error in SendEmail: %s using provider: %x", err.Error(), provider)
	}
	log.Printf("email sent!")
}

// postmarkExample shows an example using Postmark as the provider
func postmarkExample() {

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
		log.Printf("error in StartUp: %s using provider: %x", err.Error(), provider)
	}

	// Create and send a basic email
	email := mail.NewEmail()
	email.HTMLContent = "<html><body>This is a <b>go-mail</b> example email using <i>HTML</i></body></html>"
	email.Recipients = []string{toRecipients}
	email.Subject = "example go-mail email using Postmark"

	// Send the email
	if err = mail.SendEmail(email, provider); err != nil {
		log.Fatalf("error in SendEmail: %s using provider: %x", err.Error(), provider)
	}
	log.Printf("email sent!")
}

// smtpExample shows an example using SMTP as the provider
func smtpExample() {

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
	if len(mail.SMTPUsername) == 0 {
		log.Fatal("missing env: EMAIL_SMTP_USERNAME")
	}
	if mail.SMTPPort == 0 {
		log.Fatal("missing env: EMAIL_SMTP_PORT")
	}
	provider := gomail.SMTP

	// Start the service
	err := mail.StartUp()
	if err != nil {
		log.Printf("error in StartUp: %s using provider: %x", err.Error(), provider)
	}

	// Create and send a basic email
	email := mail.NewEmail()
	email.HTMLContent = "<html><body>This is a <b>go-mail</b> example email using <i>HTML</i></body></html>"
	email.Recipients = []string{toRecipients}
	email.Subject = "example go-mail email using SMTP"

	// Send the email
	if err = mail.SendEmail(email, provider); err != nil {
		log.Fatalf("error in SendEmail: %s using provider: %x", err.Error(), provider)
	}
	log.Printf("email sent!")
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
		log.Printf("error in StartUp: %s using provider: %x", err.Error(), provider)
	}

	// Create and send a basic email
	email := mail.NewEmail()
	email.HTMLContent = "<html><body>This is a <b>go-mail</b> example email using <i>HTML</i></body></html>"
	email.Recipients = []string{toRecipients}
	email.Subject = "example go-mail email using AWS SES"

	// Send the email
	if err = mail.SendEmail(email, provider); err != nil {
		log.Fatalf("error in SendEmail: %s using provider: %x", err.Error(), provider)
	}
	log.Printf("email sent!")
}

// allOptionsExample is using the most amount of options/features
func allOptionsExample() {

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

	provider := gomail.SMTP // Other options: AwsSes Mandrill Postmark

	// Start the service
	err := mail.StartUp()
	if err != nil {
		log.Printf("error in StartUp: %s using provider: %x", err.Error(), provider)
	}

	// Available services given the config above
	log.Printf("available service providers: %x", mail.AvailableProviders)

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
	if err = mail.SendEmail(email, provider); err != nil {
		log.Fatalf("error in SendEmail: %s using provider: %x", err.Error(), provider)
	}

	// Congrats!
	log.Printf("all emails sent!")
}
