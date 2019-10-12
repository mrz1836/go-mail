/*
Package main is examples using the go-mail package
*/
package main

import (
	"fmt"
	"os"

	"github.com/mrz1836/go-logger"
	"github.com/mrz1836/go-mail"
)

// main will load the examples
func main() {

	// Define your service configuration
	mail := new(gomail.MailService)
	mail.FromName = "No Reply"
	mail.FromUsername = "no-reply"
	mail.FromDomain = os.Getenv("EMAIL_FROM_DOMAIN") //example.com

	// Mandrill
	mail.MandrillAPIKey = os.Getenv("EMAIL_MANDRILL_KEY") //aOfw3WU...

	// AWS SES
	mail.AwsSesAccessID = os.Getenv("EMAIL_AWS_SES_ACCESS_ID")   //AKIAY...
	mail.AwsSesSecretKey = os.Getenv("EMAIL_AWS_SES_SECRET_KEY") //tOpw3WU...

	// Postmark
	mail.PostmarkServerToken = os.Getenv("EMAIL_POSTMARK_SERVER_TOKEN") //AKIAY...

	// Startup the services
	mail.StartUp()

	// Available services given the config above
	logger.Printf("available service providers: %x", mail.AvailableProviders)

	// Create a new email
	email := mail.NewEmail()

	email.PlainTextContent = "This is a test alert from the backend system using plain-text"
	email.HTMLContent = "<html><body>This is a test alert from the backend system using <b>HTML</b></body></html>"
	email.Recipients = []string{os.Getenv("EMAIL_TEST_TO_RECIPIENT")}
	email.RecipientsCc = []string{os.Getenv("EMAIL_TEST_CC_RECIPIENT")}
	email.RecipientsBcc = []string{os.Getenv("EMAIL_TEST_BCC_RECIPIENT")}
	email.Subject = "alert broadcast system - test message"
	email.Tags = []string{"admin_alert"}
	email.Important = true

	// Add an attachment
	f, err := os.Open("test-attachment-file.txt")
	if err != nil {
		logger.Data(2, logger.DEBUG, "unable to load file for attachment")
	} else {
		email.AddAttachment("test-attachment-file.txt", "text/plain", f)
	}

	// Send the email (basic example using one provider)
	provider := gomail.Postmark // AwsSes Mandrill
	err = mail.SendEmail(email, provider)
	if err != nil {
		logger.Data(2, logger.ERROR, fmt.Sprintf("error in SendEmail: %s using provider %x", err.Error(), provider))
	}

	// Congrats!
	logger.Data(2, logger.DEBUG, "all emails sent!")
}
