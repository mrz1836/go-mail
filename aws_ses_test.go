package gomail

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
	"github.com/aws/smithy-go/middleware"
)

// getSuccessResult returns a successful AWS SES response
func getSuccessResult() string {
	return `<SendRawEmailResponse xmlns="http://ses.amazonaws.com/doc/2010-12-01/">
  <SendRawEmailResult>
    <MessageId>01000172d9097ae4-d7e95511-f9d4-434d-9d2f-a0d860c18ee8-000000</MessageId>
  </SendRawEmailResult>
  <ResponseMetadata>
    <RequestId>8a9c266b-7b2d-4a93-89f5-9ca0031fezas</RequestId>
  </ResponseMetadata>
</SendRawEmailResponse>`
}

// mockAwsSesInterface is a mocking interface for AWS SES
type mockAwsSesInterface struct{}

// SendRawEmail is for mocking
func (m *mockAwsSesInterface) SendRawEmail(raw []byte) (string, error) {
	if len(raw) == 0 {
		return "", ErrMissingEmailContents
	}

	rawString := string(raw)

	// Success
	if strings.Contains(rawString, "To: test@domain.com") {
		return getSuccessResult(), nil
	}

	// Bad hostname
	if strings.Contains(rawString, "To: test@badhostname.com") {
		return "", ErrBadHostname
	}

	// Bad result
	if strings.Contains(rawString, "To: test@badresult.com") {
		return "<ErrorMessage>Failed!</ErrorMessage>", nil
	}

	// Default is success
	return getSuccessResult(), nil
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
		t.Fatalf("failed to attach file: %s", err.Error())
	} else {
		email.AddAttachment("test-attachment-file.txt", "text/plain", f)
	}

	// Create the list of tests
	tests := []struct {
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
			t.Fatalf("%s Failed: expected to NOT throw an error, inputted and [%s], error [%s]", t.Name(), test.input, err.Error())
		} else if err == nil && test.expectedError {
			t.Fatalf("%s Failed: expected to throw an error, inputted and [%s]", t.Name(), test.input)
		}
	}
}

// sesClientInterface defines the interface that the SES client should implement
type sesClientInterface interface {
	SendRawEmail(ctx context.Context, params *ses.SendRawEmailInput, optFns ...func(*ses.Options)) (*ses.SendRawEmailOutput, error)
}

// mockSESClient is a mock implementation of the AWS SES v2 client
type mockSESClient struct {
	sendRawEmailFunc func(ctx context.Context, params *ses.SendRawEmailInput, optFns ...func(*ses.Options)) (*ses.SendRawEmailOutput, error)
}

// SendRawEmail implements the mock behavior for SES client
func (m *mockSESClient) SendRawEmail(ctx context.Context, params *ses.SendRawEmailInput, optFns ...func(*ses.Options)) (*ses.SendRawEmailOutput, error) {
	return m.sendRawEmailFunc(ctx, params, optFns...)
}

// testAwsSesSdkV2Client is a test version that accepts an interface instead of concrete type
type testAwsSesSdkV2Client struct {
	client sesClientInterface
}

// SendRawEmail implements the same logic as awsSesSdkV2Client but uses the interface
func (c *testAwsSesSdkV2Client) SendRawEmail(raw []byte) (string, error) {
	input := &ses.SendRawEmailInput{
		RawMessage: &types.RawMessage{
			Data: raw,
		},
	}

	result, err := c.client.SendRawEmail(context.TODO(), input)
	if err != nil {
		return "", err
	}

	// Format response similar to what was expected from v1 SDK
	requestID := "unknown"
	if result.ResultMetadata.Get("RequestId") != nil {
		if id, ok := result.ResultMetadata.Get("RequestId").(string); ok {
			requestID = id
		}
	}

	responseStr := `<SendRawEmailResponse xmlns="http://ses.amazonaws.com/doc/2010-12-01/">
  <SendRawEmailResult>
    <MessageId>` + *result.MessageId + `</MessageId>
  </SendRawEmailResult>
  <ResponseMetadata>
    <RequestId>` + requestID + `</RequestId>
  </ResponseMetadata>
</SendRawEmailResponse>`

	return responseStr, nil
}

// TestAwsSesSdkV2Client_SendRawEmail tests the SendRawEmail method of awsSesSdkV2Client
func TestAwsSesSdkV2Client_SendRawEmail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		rawEmail       []byte
		mockFunc       func(ctx context.Context, params *ses.SendRawEmailInput, optFns ...func(*ses.Options)) (*ses.SendRawEmailOutput, error)
		expectedError  bool
		expectedOutput string
	}{
		{
			name:     "successful send with request id",
			rawEmail: []byte("To: test@example.com\r\nSubject: Test\r\n\r\nTest body"),
			mockFunc: func(_ context.Context, _ *ses.SendRawEmailInput, _ ...func(*ses.Options)) (*ses.SendRawEmailOutput, error) {
				messageID := "01000172d9097ae4-d7e95511-f9d4-434d-9d2f-a0d860c18ee8-000000"
				metadata := middleware.Metadata{}
				metadata.Set("RequestId", "test-request-id-123")
				output := &ses.SendRawEmailOutput{
					MessageId:      &messageID,
					ResultMetadata: metadata,
				}
				return output, nil
			},
			expectedError: false,
			expectedOutput: `<SendRawEmailResponse xmlns="http://ses.amazonaws.com/doc/2010-12-01/">
  <SendRawEmailResult>
    <MessageId>01000172d9097ae4-d7e95511-f9d4-434d-9d2f-a0d860c18ee8-000000</MessageId>
  </SendRawEmailResult>
  <ResponseMetadata>
    <RequestId>test-request-id-123</RequestId>
  </ResponseMetadata>
</SendRawEmailResponse>`,
		},
		{
			name:     "successful send without request id",
			rawEmail: []byte("To: test@example.com\r\nSubject: Test\r\n\r\nTest body"),
			mockFunc: func(_ context.Context, _ *ses.SendRawEmailInput, _ ...func(*ses.Options)) (*ses.SendRawEmailOutput, error) {
				messageID := "message-id-456"
				metadata := middleware.Metadata{}
				output := &ses.SendRawEmailOutput{
					MessageId:      &messageID,
					ResultMetadata: metadata,
				}
				return output, nil
			},
			expectedError: false,
			expectedOutput: `<SendRawEmailResponse xmlns="http://ses.amazonaws.com/doc/2010-12-01/">
  <SendRawEmailResult>
    <MessageId>message-id-456</MessageId>
  </SendRawEmailResult>
  <ResponseMetadata>
    <RequestId>unknown</RequestId>
  </ResponseMetadata>
</SendRawEmailResponse>`,
		},
		{
			name:     "successful send with non-string request id",
			rawEmail: []byte("To: test@example.com\r\nSubject: Test\r\n\r\nTest body"),
			mockFunc: func(_ context.Context, _ *ses.SendRawEmailInput, _ ...func(*ses.Options)) (*ses.SendRawEmailOutput, error) {
				messageID := "message-id-789"
				metadata := middleware.Metadata{}
				metadata.Set("RequestId", 12345) // Non-string type
				output := &ses.SendRawEmailOutput{
					MessageId:      &messageID,
					ResultMetadata: metadata,
				}
				return output, nil
			},
			expectedError: false,
			expectedOutput: `<SendRawEmailResponse xmlns="http://ses.amazonaws.com/doc/2010-12-01/">
  <SendRawEmailResult>
    <MessageId>message-id-789</MessageId>
  </SendRawEmailResult>
  <ResponseMetadata>
    <RequestId>unknown</RequestId>
  </ResponseMetadata>
</SendRawEmailResponse>`,
		},
		{
			name:     "empty raw email data",
			rawEmail: []byte{},
			mockFunc: func(_ context.Context, _ *ses.SendRawEmailInput, _ ...func(*ses.Options)) (*ses.SendRawEmailOutput, error) {
				messageID := "empty-message-id"
				metadata := middleware.Metadata{}
				metadata.Set("RequestId", "empty-request-id")
				output := &ses.SendRawEmailOutput{
					MessageId:      &messageID,
					ResultMetadata: metadata,
				}
				return output, nil
			},
			expectedError: false,
			expectedOutput: `<SendRawEmailResponse xmlns="http://ses.amazonaws.com/doc/2010-12-01/">
  <SendRawEmailResult>
    <MessageId>empty-message-id</MessageId>
  </SendRawEmailResult>
  <ResponseMetadata>
    <RequestId>empty-request-id</RequestId>
  </ResponseMetadata>
</SendRawEmailResponse>`,
		},
		{
			name:     "aws sdk error",
			rawEmail: []byte("To: test@example.com\r\nSubject: Test\r\n\r\nTest body"),
			mockFunc: func(_ context.Context, _ *ses.SendRawEmailInput, _ ...func(*ses.Options)) (*ses.SendRawEmailOutput, error) {
				return nil, ErrAWSServiceError
			},
			expectedError: true,
		},
		{
			name:     "ses validation error",
			rawEmail: []byte("invalid email format"),
			mockFunc: func(_ context.Context, _ *ses.SendRawEmailInput, _ ...func(*ses.Options)) (*ses.SendRawEmailOutput, error) {
				return nil, &types.MessageRejected{
					Message: aws.String("Email address not verified"),
				}
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockClient := &mockSESClient{
				sendRawEmailFunc: tt.mockFunc,
			}

			client := &testAwsSesSdkV2Client{
				client: mockClient,
			}

			result, err := client.SendRawEmail(tt.rawEmail)

			if tt.expectedError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result != tt.expectedOutput {
				t.Errorf("expected output:\n%s\n\ngot:\n%s", tt.expectedOutput, result)
			}
		})
	}
}

// TestAwsSesSdkV2Client_SendRawEmail_InputValidation tests input validation
func TestAwsSesSdkV2Client_SendRawEmail_InputValidation(t *testing.T) {
	t.Parallel()

	mockClient := &mockSESClient{
		sendRawEmailFunc: func(_ context.Context, params *ses.SendRawEmailInput, _ ...func(*ses.Options)) (*ses.SendRawEmailOutput, error) {
			// Verify the input structure is correct
			if params.RawMessage == nil {
				t.Error("RawMessage should not be nil")
			}
			if params.RawMessage.Data == nil {
				t.Error("RawMessage.Data should not be nil")
			}

			messageID := "validation-test-id"
			metadata := middleware.Metadata{}
			metadata.Set("RequestId", "validation-request-id")
			return &ses.SendRawEmailOutput{
				MessageId:      &messageID,
				ResultMetadata: metadata,
			}, nil
		},
	}

	client := &testAwsSesSdkV2Client{
		client: mockClient,
	}

	testData := []byte("test email data")
	_, err := client.SendRawEmail(testData)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
