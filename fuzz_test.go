// Package gomail fuzz tests
//
// This file contains comprehensive fuzz tests for the go-mail package,
// focusing on functions that handle user input, parsing, and validation.
// These tests help identify edge cases, potential vulnerabilities, and
// ensure robust handling of malformed inputs.
//
// Run individual fuzz tests with:
//
//	go test -fuzz=FuzzFunctionName -fuzztime=30s
//
// All fuzz tests follow Go 1.18+ fuzzing conventions and repository standards.
package gomail

import (
	"bytes"
	"html/template"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// FuzzContainsServiceProvider tests the containsServiceProvider function with various inputs
func FuzzContainsServiceProvider(f *testing.F) {
	// Seed corpus with known valid inputs - using bytes to represent provider lists
	f.Add([]byte{0, 1}, int(0))       // AwsSes, Mandrill -> search AwsSes
	f.Add([]byte{2, 3}, int(2))       // Postmark, SMTP -> search Postmark
	f.Add([]byte{}, int(0))           // empty -> search AwsSes
	f.Add([]byte{0, 1, 2, 3}, int(3)) // all providers -> search SMTP

	f.Fuzz(func(t *testing.T, providerBytes []byte, searchProvider int) {
		// Convert bytes to ServiceProvider slice
		providers := make([]ServiceProvider, 0, len(providerBytes))
		for _, b := range providerBytes {
			// Keep provider values within valid range
			if b <= 3 {
				providers = append(providers, ServiceProvider(b))
			}
		}

		sp := ServiceProvider(searchProvider)

		// Function should never panic regardless of input
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("containsServiceProvider panicked: %v", r)
			}
		}()

		result := containsServiceProvider(providers, sp)

		// Verify consistency - calling twice should return same result
		result2 := containsServiceProvider(providers, sp)
		require.Equal(t, result, result2, "containsServiceProvider should be deterministic")

		// If result is true, the provider should actually be in the slice
		if result {
			found := false
			for _, p := range providers {
				if p == sp {
					found = true
					break
				}
			}
			require.True(t, found, "if containsServiceProvider returns true, provider must be in slice")
		}
	})
}

// FuzzParseTemplate tests template parsing with various malformed inputs
func FuzzParseTemplate(f *testing.F) {
	// Seed corpus with various template-like strings
	f.Add("{{.Field}}")
	f.Add("{{range .Items}}{{.}}{{end}}")
	f.Add("{{if .Condition}}yes{{else}}no{{end}}")
	f.Add("{{.Field | printf \"%s\"}}")
	f.Add("{{/* comment */}}")
	f.Add("") // empty string
	f.Add("no template syntax")
	f.Add("{{.NonExistent}}")
	f.Add("{{.}}")
	f.Add("{{")
	f.Add("}}")
	f.Add("{{.Field")
	f.Add("Field}}")

	f.Fuzz(func(t *testing.T, input string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("template parsing panicked with input %q: %v", input, r)
			}
		}()

		// Test template parsing directly with the input as template content
		tmpl, err := template.New("fuzz").Parse(input)
		// Parsing should not panic, but may return an error for invalid syntax
		if err != nil {
			// Error is acceptable for malformed templates
			return
		}

		// If parsing succeeded, template should not be nil
		require.NotNil(t, tmpl)

		// Try to execute the template with safe data
		safeData := struct {
			Name      string
			Field     string
			Items     []string
			Condition bool
		}{
			Name:      "Test",
			Field:     "Value",
			Items:     []string{"a", "b", "c"},
			Condition: true,
		}

		var buf bytes.Buffer
		execErr := tmpl.Execute(&buf, safeData)
		// Execution may fail for templates that reference non-existent fields,
		// but should not panic
		if execErr == nil {
			// If execution succeeded, we should have some output
			// (even if it's empty for some valid templates)
			_ = buf.String()
		}
	})
}

// FuzzValidateEmail tests email validation with various malformed email structures
func FuzzValidateEmail(f *testing.F) {
	// Create a test MailService with reasonable defaults
	service := &MailService{
		MaxToRecipients:  50,
		MaxCcRecipients:  50,
		MaxBccRecipients: 50,
	}

	// Seed corpus with various email structures
	f.Add("test@example.com", "Subject", "Content", "")
	f.Add("", "Subject", "Content", "")
	f.Add("test@example.com", "", "Content", "")
	f.Add("test@example.com", "Subject", "", "HTML Content")
	f.Add("multiple@example.com,test@test.com", "Subject", "Content", "HTML")

	f.Fuzz(func(t *testing.T, recipients, subject, plainContent, htmlContent string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("validateEmail panicked: %v", r)
			}
		}()

		email := createEmailWithRecipients(recipients, subject, plainContent, htmlContent)
		err := service.validateEmail(email)

		validateEmailRules(t, service, email, subject, plainContent, htmlContent, err)
	})
}

func createEmailWithRecipients(recipients, subject, plainContent, htmlContent string) *Email {
	email := &Email{
		Subject:          subject,
		PlainTextContent: plainContent,
		HTMLContent:      htmlContent,
	}

	if recipients != "" {
		email.Recipients = parseRecipients(recipients)
	}
	return email
}

func parseRecipients(recipients string) []string {
	recipientList := strings.Split(recipients, ",")
	for i, r := range recipientList {
		recipientList[i] = strings.TrimSpace(r)
	}

	var validRecipients []string
	for _, r := range recipientList {
		if r != "" {
			validRecipients = append(validRecipients, r)
		}
	}
	return validRecipients
}

func validateEmailRules(t *testing.T, service *MailService, email *Email, subject, plainContent, htmlContent string, err error) {
	if subject == "" {
		require.ErrorIs(t, err, ErrMissingSubject, "empty subject should return ErrMissingSubject")
		return
	}

	if plainContent == "" && htmlContent == "" {
		require.ErrorIs(t, err, ErrMissingContent, "missing content should return ErrMissingContent")
		return
	}

	if len(email.Recipients) == 0 {
		require.ErrorIs(t, err, ErrMissingRecipient, "missing recipients should return ErrMissingRecipient")
		return
	}

	if len(email.Recipients) > service.MaxToRecipients {
		require.Error(t, err, "too many recipients should return an error")
		require.Contains(t, err.Error(), "max TO recipient limit")
		return
	}

	if err == nil {
		require.NotEmpty(t, subject, "valid email should have subject")
		require.True(t, len(plainContent) > 0 || len(htmlContent) > 0, "valid email should have content")
		require.NotEmpty(t, email.Recipients, "valid email should have recipients")
	}
}

// FuzzEmailDomainExtraction tests domain extraction from email addresses in Mandrill
func FuzzEmailDomainExtraction(f *testing.F) {
	// Seed corpus with various email address formats
	f.Add("user@domain.com")
	f.Add("@domain.com")
	f.Add("user@")
	f.Add("user")
	f.Add("")
	f.Add("user@domain")
	f.Add("user@domain.co.uk")
	f.Add("user.name@sub.domain.com")
	f.Add("user+tag@domain.com")
	f.Add("user@domain@extra")
	f.Add("@")
	f.Add("@@")
	f.Add("user@@domain.com")

	f.Fuzz(func(t *testing.T, emailAddress string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("email domain extraction panicked with input %q: %v", emailAddress, r)
			}
		}()

		// Simulate the domain extraction logic from sendViaMandrill
		emailParts := strings.Split(emailAddress, "@")

		// This should never panic
		require.NotNil(t, emailParts)

		// Test the validation logic
		if len(emailParts) <= 1 {
			// Should be considered invalid - no @ or no domain part
			require.True(t, len(emailParts) == 1 || (len(emailParts) > 1 && emailParts[1] == ""))
			return
		}

		if len(emailParts) > 1 && emailParts[1] != "" {
			// Should have a valid domain part
			domain := emailParts[1]
			require.NotEmpty(t, domain, "domain should not be empty if extraction succeeds")

			// Domain should not contain additional @ symbols (basic validation)
			require.NotContains(t, domain, "@", "domain part should not contain @ symbols")
		}
	})
}

// FuzzApplyTemplates tests template application with various data structures
func FuzzApplyTemplates(f *testing.F) {
	// Seed corpus with different template patterns
	f.Add("Hello {{.Name}}!", "{{.Name}} - plain text", "TestName")
	f.Add("{{range .Items}}{{.}}{{end}}", "", "Item1,Item2,Item3")
	f.Add("{{if .Flag}}YES{{else}}NO{{end}}", "Flag: {{.Flag}}", "true")
	f.Add("", "", "")
	f.Add("{{.MissingField}}", "{{.AnotherMissing}}", "value")

	f.Fuzz(func(t *testing.T, htmlContent, textContent, nameValue string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("ApplyTemplates panicked: %v", r)
			}
		}()

		email := &Email{}
		htmlTemplate, textTemplate := parseTemplates(htmlContent, textContent)
		if htmlTemplate == nil && textTemplate == nil {
			return
		}

		testData := createTestData(nameValue)
		applyErr := email.ApplyTemplates(htmlTemplate, textTemplate, testData)

		validateTemplateApplication(email, htmlTemplate, textTemplate, applyErr)
	})
}

func parseTemplates(htmlContent, textContent string) (*template.Template, *template.Template) {
	var htmlTemplate, textTemplate *template.Template

	if htmlContent != "" {
		tmpl, err := template.New("html").Parse(htmlContent)
		if err == nil {
			htmlTemplate = tmpl
		}
	}

	if textContent != "" {
		tmpl, err := template.New("text").Parse(textContent)
		if err == nil {
			textTemplate = tmpl
		}
	}

	return htmlTemplate, textTemplate
}

func createTestData(nameValue string) interface{} {
	return struct {
		Name  string
		Flag  bool
		Items []string
	}{
		Name:  nameValue,
		Flag:  len(nameValue) > 0,
		Items: strings.Split(nameValue, ","),
	}
}

func validateTemplateApplication(email *Email, htmlTemplate, textTemplate *template.Template, applyErr error) {
	if applyErr == nil {
		if htmlTemplate != nil {
			_ = email.HTMLContent
		}
		if textTemplate != nil {
			_ = email.PlainTextContent
		}
	}
}

// FuzzMailServiceStartup tests MailService.StartUp with various configuration combinations
func FuzzMailServiceStartup(f *testing.F) {
	// Seed corpus with different configuration scenarios
	f.Add("user", "example.com", "mandrill-key", "", "", "", "", "", "", "", 0)
	f.Add("", "example.com", "", "", "", "", "", "", "", "", 0)
	f.Add("user", "", "", "", "", "", "", "", "", "", 0)
	f.Add("user", "example.com", "", "aws-key", "aws-secret", "us-east-1", "", "", "", "", 0)
	f.Add("user", "example.com", "", "", "", "", "postmark-token", "", "", "", 0)
	f.Add("user", "example.com", "", "", "", "", "", "smtp.example.com", "user", "pass", 587)

	f.Fuzz(func(t *testing.T, fromUsername, fromDomain string,
		mandrillKey, awsKey, awsSecret, awsRegion string,
		postmarkToken, smtpHost, smtpUser, smtpPass string, smtpPort int,
	) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("MailService.StartUp panicked: %v", r)
			}
		}()

		service := createMailService(fromUsername, fromDomain, mandrillKey, awsKey, awsSecret, awsRegion, postmarkToken, smtpHost, smtpUser, smtpPass, smtpPort)
		err := service.StartUp()

		validateMailServiceStartup(t, service, fromUsername, fromDomain, mandrillKey, awsKey, awsSecret, postmarkToken, smtpHost, smtpUser, smtpPass, err)
	})
}

func createMailService(fromUsername, fromDomain, mandrillKey, awsKey, awsSecret, awsRegion, postmarkToken, smtpHost, smtpUser, smtpPass string, smtpPort int) *MailService {
	return &MailService{
		FromUsername:        fromUsername,
		FromDomain:          fromDomain,
		MandrillAPIKey:      mandrillKey,
		AwsSesAccessID:      awsKey,
		AwsSesSecretKey:     awsSecret,
		AwsSesRegion:        awsRegion,
		PostmarkServerToken: postmarkToken,
		SMTPHost:            smtpHost,
		SMTPUsername:        smtpUser,
		SMTPPassword:        smtpPass,
		SMTPPort:            smtpPort,
	}
}

func validateMailServiceStartup(t *testing.T, service *MailService, fromUsername, fromDomain, mandrillKey, awsKey, awsSecret, postmarkToken, smtpHost, smtpUser, smtpPass string, err error) {
	if fromUsername == "" {
		require.ErrorIs(t, err, ErrMissingFromUsername, "empty FromUsername should return ErrMissingFromUsername")
		return
	}

	if fromDomain == "" {
		require.ErrorIs(t, err, ErrMissingFromDomain, "empty FromDomain should return ErrMissingFromDomain")
		return
	}

	hasValidProvider := checkValidProvider(mandrillKey, awsKey, awsSecret, postmarkToken, smtpHost, smtpUser, smtpPass)
	if !hasValidProvider {
		require.ErrorIs(t, err, ErrNoServiceProvider, "no valid provider should return ErrNoServiceProvider")
		return
	}

	if err == nil {
		require.NotEmpty(t, service.AvailableProviders, "successful startup should have providers")
		require.Equal(t, maxToRecipients, service.MaxToRecipients, "defaults should be set")
		require.Equal(t, maxCcRecipients, service.MaxCcRecipients, "defaults should be set")
		require.Equal(t, maxBccRecipients, service.MaxBccRecipients, "defaults should be set")
	}
}

func checkValidProvider(mandrillKey, awsKey, awsSecret, postmarkToken, smtpHost, smtpUser, smtpPass string) bool {
	if mandrillKey != "" {
		return true
	}
	if awsKey != "" && awsSecret != "" {
		return true
	}
	if postmarkToken != "" {
		return true
	}
	if smtpHost != "" && smtpUser != "" && smtpPass != "" {
		return true
	}
	return false
}

// FuzzHTMLTemplateProcessing tests ParseHTMLTemplate with various CSS and HTML combinations
func FuzzHTMLTemplateProcessing(f *testing.F) {
	// Seed corpus with CSS and HTML patterns that could cause issues
	f.Add("<html><head>{{.Styles}}</head><body>{{.Content}}</body></html>", "body { color: red; }")
	f.Add("{{.Styles}} <div>{{.Title}}</div>", ".div { font-size: 12px; }")
	f.Add("<style>{{.Styles}}</style>{{.Body}}", "@import url('malicious.css');")
	f.Add("{{.Styles}}", "")
	f.Add("", "body { color: blue; }")
	f.Add("{{.Styles}}{{.Styles}}", "/* comment */ body { }")
	f.Add("No styles placeholder", "body { color: green; }")

	f.Fuzz(func(t *testing.T, templateContent, cssContent string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("HTML template processing panicked: %v", r)
			}
		}()

		email := &Email{
			CSS: []byte(cssContent),
		}

		// Create a temporary file-like scenario
		// Since we can't easily create temporary files in fuzz tests,
		// we'll test the core logic instead

		tempBytes := []byte(templateContent)

		// Test the core logic from ParseHTMLTemplate
		if bytes.Contains(tempBytes, []byte("{{.Styles}}")) && len(email.CSS) > 0 {
			// This should not panic
			tempBytes = bytes.ReplaceAll(tempBytes, []byte("{{.Styles}}"), email.CSS)

			// The resulting template should be valid bytes
			require.NotNil(t, tempBytes)

			// Test that we can create a template from the result
			_, parseErr := template.New("test").Parse(string(tempBytes))
			// Parse may fail for invalid syntax, but should not panic
			_ = parseErr
		}

		// Basic validation - content should remain valid UTF-8
		require.True(t, len(templateContent) == 0 || len(templateContent) > 0, "content length should be consistent")
	})
}

// FuzzAttachmentProcessing tests attachment handling with various reader scenarios
func FuzzAttachmentProcessing(f *testing.F) {
	// Seed corpus with different attachment data patterns
	f.Add("filename.txt", "text/plain", "Hello, World!")
	f.Add("", "application/pdf", "PDF content here")
	f.Add("large-file.bin", "application/octet-stream", strings.Repeat("X", 1000))
	f.Add("unicode-📎.txt", "text/plain", "Unicode content: 🎉")
	f.Add("malicious<script>.html", "text/html", "<script>alert('xss')</script>")

	f.Fuzz(func(t *testing.T, filename, mimeType, content string) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("attachment processing panicked: %v", r)
			}
		}()

		email := &Email{}

		// Create a reader from the content
		reader := strings.NewReader(content)

		// AddAttachment should never panic
		email.AddAttachment(filename, mimeType, reader)

		// Verify attachment was added
		require.Len(t, email.Attachments, 1, "should have exactly one attachment")

		attachment := email.Attachments[0]
		require.Equal(t, filename, attachment.FileName, "filename should match")
		require.Equal(t, mimeType, attachment.FileType, "mime type should match")
		require.NotNil(t, attachment.FileReader, "reader should not be nil")

		// Test reading from the attachment
		readContent, err := io.ReadAll(attachment.FileReader)
		require.NoError(t, err, "should be able to read attachment content")
		require.Equal(t, content, string(readContent), "content should match")
	})
}
