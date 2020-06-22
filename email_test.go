package gomail

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"path/filepath"
	"testing"
)

// TestMailService_NewEmail tests the method NewEmail()
func TestMailService_NewEmail(t *testing.T) {
	t.Parallel()

	mail := new(MailService)

	mail.AutoText = true
	mail.FromUsername = "no-reply"
	mail.FromName = "No Reply"
	mail.FromDomain = "example.com"
	mail.Important = true

	email := mail.NewEmail()

	if email.FromAddress != mail.FromUsername+"@"+mail.FromDomain {
		t.Fatalf("%s: FromAddress is invalid", t.Name())
	}

	if email.ReplyToAddress != email.FromAddress {
		t.Fatalf("%s: ReplyToAddress is invalid", t.Name())
	}

	if !email.AutoText {
		t.Fatalf("%s: AutoText is invalid", t.Name())
	}

	if !email.Important {
		t.Fatalf("%s: Important is invalid", t.Name())
	}

	if email.FromName != mail.FromName {
		t.Fatalf("%s: FromName is invalid", t.Name())
	}
}

// ExampleMailService_NewEmail example using the NewEmail()
func ExampleMailService_NewEmail() {
	mail := new(MailService)
	mail.FromUsername = "no-reply"
	mail.FromName = "No Reply"
	mail.FromDomain = "example.com"

	email := mail.NewEmail()
	fmt.Printf("new email with from address: %s", email.FromAddress)
	// output: new email with from address: no-reply@example.com
}

// BenchmarkMailService_NewEmail runs benchmark on NewEmail()
func BenchmarkMailService_NewEmail(b *testing.B) {
	mail := new(MailService)
	mail.FromUsername = "no-reply"
	mail.FromName = "No Reply"
	mail.FromDomain = "example.com"
	for i := 0; i < b.N; i++ {
		_ = mail.NewEmail()
	}
}

// TestEmail_AddAttachment tests the method AddAttachment()
func TestEmail_AddAttachment(t *testing.T) {
	t.Parallel()

	mail := new(MailService)
	mail.FromUsername = "no-reply"
	mail.FromName = "No Reply"
	mail.FromDomain = "example.com"

	email := mail.NewEmail()
	email.AddAttachment("testName", "testType", nil)
	email.AddAttachment("testName2", "testType2", nil)

	if len(email.Attachments) != 2 {
		t.Fatalf("%s: expected 2 attachments, got: %d", t.Name(), len(email.Attachments))
	}

	if email.Attachments[0].FileName != "testName" || email.Attachments[0].FileType != "testType" {
		t.Fatalf("%s: expected value was wrong, got: %s", t.Name(), email.Attachments[0].FileName)
	}

	if email.Attachments[1].FileName != "testName2" || email.Attachments[1].FileType != "testType2" {
		t.Fatalf("%s: expected value was wrong, got: %s", t.Name(), email.Attachments[0].FileName)
	}
}

// ExampleEmail_AddAttachment example using the AddAttachment()
func ExampleEmail_AddAttachment() {
	mail := new(MailService)
	mail.FromUsername = "no-reply"
	mail.FromName = "No Reply"
	mail.FromDomain = "example.com"

	email := mail.NewEmail()
	email.AddAttachment("testName", "testType", nil)

	fmt.Printf("attachment: %s", email.Attachments[0].FileName)
	// output: attachment: testName
}

// BenchmarkEmail_AddAttachment runs benchmark on AddAttachment()
func BenchmarkEmail_AddAttachment(b *testing.B) {
	mail := new(MailService)
	mail.FromUsername = "no-reply"
	mail.FromName = "No Reply"
	mail.FromDomain = "example.com"
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
	mail.FromUsername = "no-reply"
	mail.FromName = "No Reply"
	mail.FromDomain = "example.com"
	mail.Important = true

	email := mail.NewEmail()

	// Parse a text template into memory
	parsedTemplate, err := email.ParseTemplate(filepath.Join("examples", "example_template.txt"))
	if err != nil {
		t.Fatalf("%s: error occurred: %s", t.Name(), err.Error())
	} else if parsedTemplate == nil {
		t.Fatalf("%s: template was nil", t.Name())
	} else if parsedTemplate.Name() != "example_template.txt" {
		t.Fatalf("%s: template name expected [%s] does not match [%s]", t.Name(), "example_template.txt", parsedTemplate.Name())
	}

	// Parse - missing file
	parsedTemplate, err = email.ParseTemplate(filepath.Join("examples", "missing_file.txt"))
	if err == nil {
		t.Fatalf("%s: error expected but was nil", t.Name())
	}
}

// TestEmail_ParseHTMLTemplate tests the method ParseHTMLTemplate()
func TestEmail_ParseHTMLTemplate(t *testing.T) {
	t.Parallel()

	mail := new(MailService)

	mail.AutoText = true
	mail.FromUsername = "no-reply"
	mail.FromName = "No Reply"
	mail.FromDomain = "example.com"
	mail.Important = true

	email := mail.NewEmail()

	// Parse a HTML template into memory
	parsedTemplate, err := email.ParseHTMLTemplate(filepath.Join("examples", "example_template.html"))
	if err != nil {
		t.Fatalf("%s: error occurred: %s", t.Name(), err.Error())
	} else if parsedTemplate == nil {
		t.Fatalf("%s: template was nil", t.Name())
	} else if parsedTemplate.Name() != "example_template.html" {
		t.Fatalf("%s: template name expected [%s] does not match [%s]", t.Name(), "example_template.html", parsedTemplate.Name())
	}

	// Parse a HTML template and process CSS styles
	parsedTemplate, err = email.ParseHTMLTemplate(filepath.Join("examples", "example_template_css.html"))
	if err != nil {
		t.Fatalf("%s: error occurred: %s", t.Name(), err.Error())
	} else if parsedTemplate == nil {
		t.Fatalf("%s: template was nil", t.Name())
	} else if parsedTemplate.Name() != "example_template_css.html" {
		t.Fatalf("%s: template name expected [%s] does not match [%s]", t.Name(), "example_template_css.html", parsedTemplate.Name())
	}

	// Parse - missing file
	parsedTemplate, err = email.ParseHTMLTemplate(filepath.Join("examples", "missing_file.html"))
	if err == nil {
		t.Fatalf("%s: error expected but was nil", t.Name())
	}
}

// TestEmail_ApplyTemplates tests the method ApplyTemplates()
func TestEmail_ApplyTemplates(t *testing.T) {
	t.Parallel()

	mail := new(MailService)

	mail.AutoText = true
	mail.FromUsername = "no-reply"
	mail.FromName = "No Reply"
	mail.FromDomain = "example.com"
	mail.Important = true

	email := mail.NewEmail()

	// Parse a text template into memory
	parsedTemplate, err := email.ParseTemplate(filepath.Join("examples", "example_template.txt"))
	if err != nil {
		t.Fatalf("%s: error occurred: %s", t.Name(), err.Error())
	} else if parsedTemplate == nil {
		t.Fatalf("%s: template was nil", t.Name())
	} else if parsedTemplate.Name() != "example_template.txt" {
		t.Fatalf("%s: template name expected [%s] does not match [%s]", t.Name(), "example_template.txt", parsedTemplate.Name())
	}

	// Set the css theme
	if email.CSS, err = ioutil.ReadFile(filepath.Join("examples", "example_theme.css")); err != nil {
		t.Fatalf("%s: error occurred: %s", t.Name(), err.Error())
	}

	// Parse a HTML template and process CSS styles
	var parsedHTMLTemplate *template.Template
	parsedHTMLTemplate, err = email.ParseHTMLTemplate(filepath.Join("examples", "example_template_css.html"))
	if err != nil {
		t.Fatalf("%s: error occurred: %s", t.Name(), err.Error())
	} else if parsedHTMLTemplate == nil {
		t.Fatalf("%s: template was nil", t.Name())
	} else if parsedHTMLTemplate.Name() != "example_template_css.html" {
		t.Fatalf("%s: template name expected [%s] does not match [%s]", t.Name(), "example_template_css.html", parsedHTMLTemplate.Name())
	}

	// Apply the data to the template
	if err = email.ApplyTemplates(parsedHTMLTemplate, parsedTemplate, mail); err != nil {
		t.Fatalf("%s: error occurred: %s", t.Name(), err.Error())
	}

	// Get error from missing template variable
	if err = email.ApplyTemplates(parsedHTMLTemplate, parsedTemplate, "no data"); err == nil {
		t.Fatalf("%s: error should have occurred", t.Name())
	}
}
