package gomail

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// Normal result from a successful email
var successResult = `<SendRawEmailResponse xmlns="http://ses.amazonaws.com/doc/2010-12-01/">
  <SendRawEmailResult>
    <MessageId>01000172d9097ae4-d7e95511-f9d4-434d-9d2f-a0d860c18ee8-000000</MessageId>
  </SendRawEmailResult>
  <ResponseMetadata>
    <RequestId>8a9c266b-7b2d-4a93-89f5-9ca0031fezas</RequestId>
  </ResponseMetadata>
</SendRawEmailResponse>`

// mockAwsSesInterface is a mocking interface for AWS SES
type mockAwsSesInterface struct{}

// SendEmail is for mocking
func (m *mockAwsSesInterface) SendEmail(from, to, subject, body string) (string, error) {
	return "", nil
}

// SendEmailHTML is for mocking
func (m *mockAwsSesInterface) SendEmailHTML(from, to, subject, bodyText, bodyHTML string) (string, error) {
	return "", nil
}

// SendRawEmail is for mocking
func (m *mockAwsSesInterface) SendRawEmail(raw []byte) (string, error) {

	if len(raw) == 0 {
		return "", fmt.Errorf("missing email contents")
	}

	rawString := string(raw)

	// Success
	if strings.Contains(rawString, "To: test@domain.com") {
		return successResult, nil
	}

	// Bad hostname
	if strings.Contains(rawString, "To: test@badhostname.com") {
		return "", fmt.Errorf("bad hostname error")
	}

	// Bad result
	if strings.Contains(rawString, "To: test@badresult.com") {
		return "<ErrorMessage>Failed!</ErrorMessage>", nil
	}

	// Default is success
	return successResult, nil
}

// newMockAwsSesClient will create a new mock client for AWS SES
func newMockAwsSesClient() awsSesInterface {
	return &mockAwsSesInterface{}
}

// TestSendViaAwsSes will test the sendViaAwsSes() method
func TestSendViaAwsSes(t *testing.T) {
	t.Parallel()

	// Start the service
	mail := new(MailService)

	// Set all the defaults, toggle all warnings
	mail.AutoText = true
	mail.FromDomain = "example.com"
	mail.FromName = "No Reply"
	mail.FromUsername = "no-reply"
	mail.Important = true
	mail.TrackClicks = true
	mail.TrackOpens = true

	// Setup mock client
	client := newMockAwsSesClient()

	// New email
	email := mail.NewEmail()
	email.HTMLContent = "<html>Test</html>"
	email.PlainTextContent = "Test"

	// Add an attachment
	f, err := os.Open("examples/test-attachment-file.txt")
	if err != nil {
		t.Errorf("failed to attach file: %s", err.Error())
	} else {
		email.AddAttachment("test-attachment-file.txt", "text/plain", f)
	}

	// Create the list of tests
	var tests = []struct {
		input         string
		expectedError bool
	}{
		{"test@domain.com", false},
		{"test@badhostname.com", true},
		{"test@badresult.com", true},
	}

	// Loop tests
	for _, test := range tests {
		email.Recipients = []string{test.input}
		email.RecipientsCc = []string{test.input}
		email.RecipientsBcc = []string{test.input}
		email.ReplyToAddress = test.input
		if err = sendViaAwsSes(client, email); err != nil && !test.expectedError {
			t.Errorf("%s Failed: expected to NOT throw an error, inputted and [%s], error [%s]", t.Name(), test.input, err.Error())
		} else if err == nil && test.expectedError {
			t.Errorf("%s Failed: expected to throw an error, inputted and [%s]", t.Name(), test.input)
		}
	}
}
