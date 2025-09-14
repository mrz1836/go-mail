package gomail

import (
	"context"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testDomainEmail   = "example.com"
	testFromNameEmail = "No Reply"
	testUsernameEmail = "no-reply"
)

// TestMailService_NewEmail tests the method NewEmail()
func TestMailService_NewEmail(t *testing.T) {
	t.Parallel()

	mail := new(MailService)

	mail.AutoText = true
	mail.FromUsername = testUsernameEmail
	mail.FromName = testFromNameEmail
	mail.FromDomain = testDomainEmail
	mail.Important = true

	email := mail.NewEmail()

	assert.Equal(t, mail.FromUsername+"@"+mail.FromDomain, email.FromAddress)

	assert.Equal(t, email.FromAddress, email.ReplyToAddress)

	assert.True(t, email.AutoText)

	assert.True(t, email.Important)

	assert.Equal(t, mail.FromName, email.FromName)
}

// ExampleMailService_NewEmail example using the NewEmail()
func ExampleMailService_NewEmail() {
	mail := new(MailService)
	mail.FromUsername = testUsernameEmail
	mail.FromName = testFromNameEmail
	mail.FromDomain = testDomainEmail

	email := mail.NewEmail()
	fmt.Printf("new email with from address: %s", email.FromAddress)
	// output: new email with from address: no-reply@example.com
}

// BenchmarkMailService_NewEmail runs benchmark on NewEmail()
func BenchmarkMailService_NewEmail(b *testing.B) {
	mail := new(MailService)
	mail.FromUsername = testUsernameEmail
	mail.FromName = testFromNameEmail
	mail.FromDomain = testDomainEmail
	for i := 0; i < b.N; i++ {
		_ = mail.NewEmail()
	}
}

// TestEmail_AddAttachment tests the method AddAttachment()
func TestEmail_AddAttachment(t *testing.T) {
	t.Parallel()

	mail := new(MailService)
	mail.FromUsername = testUsernameEmail
	mail.FromName = testFromNameEmail
	mail.FromDomain = testDomainEmail

	email := mail.NewEmail()
	email.AddAttachment("testName", "testType", nil)
	email.AddAttachment("testName2", "testType2", nil)

	assert.Len(t, email.Attachments, 2)

	assert.Equal(t, "testName", email.Attachments[0].FileName)
	assert.Equal(t, "testType", email.Attachments[0].FileType)

	assert.Equal(t, "testName2", email.Attachments[1].FileName)
	assert.Equal(t, "testType2", email.Attachments[1].FileType)
}

// ExampleEmail_AddAttachment example using the AddAttachment()
func ExampleEmail_AddAttachment() {
	mail := new(MailService)
	mail.FromUsername = testUsernameEmail
	mail.FromName = testFromNameEmail
	mail.FromDomain = testDomainEmail

	email := mail.NewEmail()
	email.AddAttachment("testName", "testType", nil)

	fmt.Printf("attachment: %s", email.Attachments[0].FileName)
	// output: attachment: testName
}

// BenchmarkEmail_AddAttachment runs benchmark on AddAttachment()
func BenchmarkEmail_AddAttachment(b *testing.B) {
	mail := new(MailService)
	mail.FromUsername = testUsernameEmail
	mail.FromName = testFromNameEmail
	mail.FromDomain = testDomainEmail
	email := mail.NewEmail()
	for i := 0; i < b.N; i++ {
		email.AddAttachment("testName", "testType", nil)
	}
}

// TestEmail_ParseTemplate tests the method ParseTemplate()
func TestEmail_ParseTemplate(t *testing.T) {
	t.Parallel()

	mail := new(MailService)

	mail.AutoText = true
	mail.FromUsername = testUsernameEmail
	mail.FromName = testFromNameEmail
	mail.FromDomain = testDomainEmail
	mail.Important = true

	email := mail.NewEmail()

	// Parse a text template into memory
	parsedTemplate, err := email.ParseTemplate(filepath.Join("examples", "example_template.txt"))
	require.NoError(t, err)
	require.NotNil(t, parsedTemplate)
	assert.Equal(t, "example_template.txt", parsedTemplate.Name())

	// Parse - missing file
	_, err = email.ParseTemplate(filepath.Join("examples", "missing_file.txt"))
	require.Error(t, err)
}

// TestEmail_ParseHTMLTemplate tests the method ParseHTMLTemplate()
func TestEmail_ParseHTMLTemplate(t *testing.T) {
	t.Parallel()

	mail := new(MailService)

	mail.AutoText = true
	mail.FromUsername = testUsernameEmail
	mail.FromName = testFromNameEmail
	mail.FromDomain = testDomainEmail
	mail.Important = true

	email := mail.NewEmail()

	// Parse an HTML template into memory
	parsedTemplate, err := email.ParseHTMLTemplate(filepath.Join("examples", "example_template.html"))
	require.NoError(t, err)
	require.NotNil(t, parsedTemplate)
	assert.Equal(t, "example_template.html", parsedTemplate.Name())

	// Parse an HTML template and process CSS styles
	parsedTemplate, err = email.ParseHTMLTemplate(filepath.Join("examples", "example_template_css.html"))
	require.NoError(t, err)
	require.NotNil(t, parsedTemplate)
	assert.Equal(t, "example_template_css.html", parsedTemplate.Name())

	// Parse - missing file
	_, err = email.ParseHTMLTemplate(filepath.Join("examples", "missing_file.html"))
	require.Error(t, err)
}

// TestEmail_ApplyTemplates tests the method ApplyTemplates()
func TestEmail_ApplyTemplates(t *testing.T) {
	t.Parallel()

	mail := new(MailService)

	mail.AutoText = true
	mail.FromUsername = testUsernameEmail
	mail.FromName = testFromNameEmail
	mail.FromDomain = testDomainEmail
	mail.Important = true

	email := mail.NewEmail()

	// Parse a text template into memory
	parsedTemplate, err := email.ParseTemplate(filepath.Join("examples", "example_template.txt"))
	require.NoError(t, err)
	require.NotNil(t, parsedTemplate)
	assert.Equal(t, "example_template.txt", parsedTemplate.Name())

	// Set the css theme
	email.CSS, err = os.ReadFile(filepath.Join("examples", "example_theme.css"))
	require.NoError(t, err)

	// Parse an HTML template and process CSS styles
	var parsedHTMLTemplate *template.Template
	parsedHTMLTemplate, err = email.ParseHTMLTemplate(filepath.Join("examples", "example_template_css.html"))
	require.NoError(t, err)
	require.NotNil(t, parsedHTMLTemplate)
	assert.Equal(t, "example_template_css.html", parsedHTMLTemplate.Name())

	// Apply the data to the template
	err = email.ApplyTemplates(parsedHTMLTemplate, parsedTemplate, mail)
	require.NoError(t, err)

	// Apply no data
	err = email.ApplyTemplates(parsedHTMLTemplate, parsedTemplate, nil)
	require.NoError(t, err)

	// Get error from missing template variable
	err = email.ApplyTemplates(parsedHTMLTemplate, parsedTemplate, "no data")
	require.Error(t, err)
}

// TestMailService_SendEmail tests the method SendEmail()
func TestMailService_SendEmail(t *testing.T) {
	t.Parallel()

	mail := new(MailService)

	mail.AutoText = true
	mail.FromUsername = testUsernameEmail
	mail.FromName = testFromNameEmail
	mail.FromDomain = testDomainEmail
	mail.Important = true

	// Use the AWS SES provider
	mail.AwsSesAccessID = "1234567"
	mail.AwsSesSecretKey = "1234567"

	// Use the Postmark provider
	mail.PostmarkServerToken = "1234567"

	// Use the Mandrill provider
	mail.MandrillAPIKey = "1234567"

	// Use the SMTP provider
	mail.SMTPPort = 25
	mail.SMTPUsername = "fake"
	mail.SMTPPassword = "fake"
	mail.SMTPHost = testDomainEmail

	// Start the mail service
	err := mail.StartUp()
	require.NoError(t, err)

	// Set mock interface(s)
	mail.postmarkService = &mockPostmarkInterface{}
	mail.mandrillService = &mockMandrillInterface{}
	mail.smtpClient = newMockSMTPClient()
	mail.awsSesService = &mockAwsSesInterface{}

	email := mail.NewEmail()
	email.Subject = "Test subject"
	email.PlainTextContent = "Test email content"
	email.Recipients = append(email.Recipients, "someone@domain.com")

	// Valid (Postmark)
	err = mail.SendEmail(context.Background(), email, Postmark)
	require.NoError(t, err)

	// Valid (AWS SES)
	err = mail.SendEmail(context.Background(), email, AwsSes)
	require.NoError(t, err)

	// Valid (Mandrill)
	err = mail.SendEmail(context.Background(), email, Mandrill)
	require.NoError(t, err)

	// Valid (SMTP)
	err = mail.SendEmail(context.Background(), email, SMTP)
	require.NoError(t, err)
}

// TestMailService_SendEmailInValid tests the method SendEmail()
func TestMailService_SendEmailInValid(t *testing.T) {
	t.Parallel()

	mail := new(MailService)

	mail.AutoText = true
	mail.FromUsername = testUsernameEmail
	mail.FromName = testFromNameEmail
	mail.FromDomain = testDomainEmail
	mail.Important = true

	// Use the Postmark provider
	mail.PostmarkServerToken = "1234567"

	// Start the mail service
	err := mail.StartUp()
	require.NoError(t, err)

	// Set mock interface(s)
	mail.postmarkService = &mockPostmarkInterface{}

	email := mail.NewEmail()

	// Invalid provider
	err = mail.SendEmail(context.Background(), email, 999)
	require.Error(t, err)

	// Invalid provider - not in available list
	err = mail.SendEmail(context.Background(), email, AwsSes)
	require.Error(t, err)

	// Invalid - subject
	err = mail.SendEmail(context.Background(), email, Postmark)
	require.Error(t, err)
	email.Subject = "Subject exits now"

	// Invalid - plain text missing
	err = mail.SendEmail(context.Background(), email, Postmark)
	require.Error(t, err)
	email.PlainTextContent = "Plain text exits now"

	// Invalid - recipients missing
	err = mail.SendEmail(context.Background(), email, Postmark)
	require.Error(t, err)
	email.Recipients = append(email.Recipients, "someone@domain.com")

	// Too many TO recipients
	for recipients := 1; recipients <= maxToRecipients+1; recipients++ {
		email.Recipients = append(email.Recipients, "someone@domain.com")
	}
	err = mail.SendEmail(context.Background(), email, Postmark)
	require.Error(t, err)
	email.Recipients = []string{"someone@domain.com"}

	// Too many CC recipients
	for recipients := 1; recipients <= maxCcRecipients+1; recipients++ {
		email.RecipientsCc = append(email.RecipientsCc, "someone@domain.com")
	}
	err = mail.SendEmail(context.Background(), email, Postmark)
	require.Error(t, err)
	email.RecipientsCc = []string{"someone@domain.com"}

	// Too many BCC recipients
	for recipients := 1; recipients <= maxBccRecipients+1; recipients++ {
		email.RecipientsBcc = append(email.RecipientsBcc, "someone@domain.com")
	}
	err = mail.SendEmail(context.Background(), email, Postmark)
	require.Error(t, err)
}
