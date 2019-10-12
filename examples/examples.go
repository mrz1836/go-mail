/*
Package main is examples using the go-mail package
*/
package main

import (
	"fmt"
	"os"
	"strconv"

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

	// SMTP
	mail.SmtpHost = os.Getenv("EMAIL_SMTP_HOST")                  //example.com
	mail.SmtpPort, _ = strconv.Atoi(os.Getenv("EMAIL_SMTP_PORT")) //25
	mail.SmtpUsername = os.Getenv("EMAIL_SMTP_USERNAME")          //johndoe
	mail.SmtpPassword = os.Getenv("EMAIL_SMTP_PASSWORD")          //secretPassword

	// Startup the services
	mail.StartUp()

	// Available services given the config above
	logger.Printf("available service providers: %x", mail.AvailableProviders)

	// Create a new email
	email := mail.NewEmail()

	email.PlainTextContent = "This is a go-mail test email using plain-text"
	email.HTMLContent = "<html><body>This is a <b>go-mail</b> test email using <i>HTML</i></body></html>"
	email.Recipients = []string{os.Getenv("EMAIL_TEST_TO_RECIPIENT")}
	email.RecipientsCc = []string{os.Getenv("EMAIL_TEST_CC_RECIPIENT")}
	email.RecipientsBcc = []string{os.Getenv("EMAIL_TEST_BCC_RECIPIENT")}
	email.Subject = "testing go-mail package - test message"
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
	provider := gomail.Smtp // AwsSes Mandrill Postmark
	err = mail.SendEmail(email, provider)
	if err != nil {
		logger.Data(2, logger.ERROR, fmt.Sprintf("error in SendEmail: %s using provider %x", err.Error(), provider))
	}

	// Congrats!
	logger.Data(2, logger.DEBUG, "all emails sent!")
}
