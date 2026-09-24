package gomail

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/ses/types"
	"github.com/aws/smithy-go/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
func (m *mockAwsSesInterface) SendRawEmail(_ context.Context, raw []byte) (string, error) {
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

	// Setup mock client and a ready-to-send email
	client := newMockAwsSesClient()
	email := newProviderTestEmail(t)

	// Create the list of tests
	cases := []providerSendCase{
		{"successful send", testRecipientSuccess, false},
		{"bad hostname", "test@badhostname.com", true},
		{"bad result", "test@badresult.com", true},
	}

	// Loop tests
	runProviderSendCases(t, email, cases, func() error {
		return sendViaAwsSes(context.Background(), client, email)
	})
}

// TestSendViaAwsSes_MimeBufError verifies sendViaAwsSes returns the error from
// mailyak's MimeBuf() when building the MIME message fails. The failure is forced
// through public Email inputs alone: an attachment whose reader always errors
// makes mailyak's writeAttachments (invoked by MimeBuf) return that error before
// the SES client is ever called
func TestSendViaAwsSes_MimeBufError(t *testing.T) {
	t.Parallel()

	email := &Email{
		Recipients: []string{testRecipientSuccess},
		Subject:    "MimeBuf error path",
		Attachments: []Attachment{
			{FileName: "bad.txt", FileType: "text/plain", FileReader: errReader{}},
		},
	}

	// The SES client is never reached because MimeBuf fails first
	client := newMockAwsSesClient()

	err := sendViaAwsSes(context.Background(), client, email)
	require.ErrorIs(t, err, ErrBadHostname)
}

// mockSESClient is a mock implementation of the AWS SES v2 client
type mockSESClient struct {
	sendRawEmailFunc func(ctx context.Context, params *ses.SendRawEmailInput, optFns ...func(*ses.Options)) (*ses.SendRawEmailOutput, error)
}

// SendRawEmail implements the mock behavior for SES client
func (m *mockSESClient) SendRawEmail(ctx context.Context, params *ses.SendRawEmailInput, optFns ...func(*ses.Options)) (*ses.SendRawEmailOutput, error) {
	return m.sendRawEmailFunc(ctx, params, optFns...)
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

			client := &awsSesSdkV2Client{
				client: mockClient,
			}

			result, err := client.SendRawEmail(context.Background(), tt.rawEmail)

			if tt.expectedError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			assert.Equal(t, tt.expectedOutput, result)
		})
	}
}

// TestAwsSesSdkV2Client_SendRawEmail_InputValidation tests input validation
func TestAwsSesSdkV2Client_SendRawEmail_InputValidation(t *testing.T) {
	t.Parallel()

	mockClient := &mockSESClient{
		sendRawEmailFunc: func(_ context.Context, params *ses.SendRawEmailInput, _ ...func(*ses.Options)) (*ses.SendRawEmailOutput, error) {
			// Verify the input structure is correct
			require.NotNil(t, params.RawMessage)
			require.NotNil(t, params.RawMessage.Data)

			messageID := "validation-test-id"
			metadata := middleware.Metadata{}
			metadata.Set("RequestId", "validation-request-id")
			return &ses.SendRawEmailOutput{
				MessageId:      &messageID,
				ResultMetadata: metadata,
			}, nil
		},
	}

	client := &awsSesSdkV2Client{
		client: mockClient,
	}

	testData := []byte("test email data")
	_, err := client.SendRawEmail(context.Background(), testData)
	require.NoError(t, err)
}
