package gomail

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/mail"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/mailgun/mailgun-go/v5"
	"github.com/mailgun/mailgun-go/v5/mtypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mailgun test values
const (
	testMailgunAPIKey    = "key-test"
	testMailgunDomain    = "mg.example.com"
	testMailgunMessageID = "<20260101000000.1@mg.example.com>"
	testMailgunQueued    = `{"id":"` + testMailgunMessageID + `","message":"Queued. Thank you."}`
	testMailgunMaxBody   = 1 << 20 // largest request body the test server accepts
)

// errMailgunTransport is a test-only transport error
var errMailgunTransport = errors.New("dial tcp: no such host")

// mailgunFile is a file part of the multipart form the Mailgun SDK sends
type mailgunFile struct {
	name    string
	content string
}

// mailgunRequest is the HTTP request the Mailgun SDK sent
type mailgunRequest struct {
	values   map[string][]string
	files    map[string][]mailgunFile
	path     string
	username string
	password string
}

// mailgunCapture records the HTTP requests the Mailgun SDK sends
type mailgunCapture struct {
	err      error
	last     mailgunRequest
	requests int
	mu       sync.Mutex
}

// record parses and stores the multipart form request
func (c *mailgunCapture) record(w http.ResponseWriter, r *http.Request) {
	req, err := parseMailgunRequest(w, r)

	c.mu.Lock()
	defer c.mu.Unlock()
	c.requests++
	c.last, c.err = req, err
}

// request returns the last request, failing the test if it could not be parsed
func (c *mailgunCapture) request(t *testing.T) mailgunRequest {
	t.Helper()

	c.mu.Lock()
	defer c.mu.Unlock()
	require.NoError(t, c.err)
	require.Positive(t, c.requests, "no request was sent")
	return c.last
}

// count returns the number of requests received
func (c *mailgunCapture) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.requests
}

// parseMailgunRequest reads the form values and files of a Mailgun send request
// (the multipart body is streamed part by part, bounded by testMailgunMaxBody)
func parseMailgunRequest(w http.ResponseWriter, r *http.Request) (mailgunRequest, error) {
	req := mailgunRequest{
		files:  make(map[string][]mailgunFile),
		path:   r.URL.Path,
		values: make(map[string][]string),
	}
	req.username, req.password, _ = r.BasicAuth()

	r.Body = http.MaxBytesReader(w, r.Body, testMailgunMaxBody)
	reader, err := r.MultipartReader()
	if err != nil {
		return req, err
	}

	for {
		part, partErr := reader.NextPart()
		if errors.Is(partErr, io.EOF) {
			return req, nil
		}
		if partErr != nil {
			return req, partErr
		}

		content, readErr := io.ReadAll(part)
		if readErr != nil {
			return req, readErr
		}

		key := part.FormName()
		if name := part.FileName(); len(name) > 0 {
			req.files[key] = append(req.files[key], mailgunFile{name: name, content: string(content)})
			continue
		}
		req.values[key] = append(req.values[key], string(content))
	}
}

// newCapturingMailgunServer starts an httptest server that records each request
// and responds with the given status and body, and returns a Mailgun provider
// using the real SDK pointed at it
func newCapturingMailgunServer(t *testing.T, domain string, status int, body string) (*MailgunProvider, *mailgunCapture) {
	t.Helper()

	capture := &mailgunCapture{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capture.record(w, r)
		w.Header().Set("Content-Type", "application/json")
		if status == http.StatusTooManyRequests {
			w.Header().Set("X-RateLimit-Reset", "1893456000")
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	client := mailgun.NewMailgun(testMailgunAPIKey)
	client.SetHTTPClient(srv.Client())
	require.NoError(t, client.SetAPIBase(srv.URL))

	return NewMailgunProvider(client, domain), capture
}

// TestMailgunProviderSendWire checks the HTTP request the real SDK sends
func TestMailgunProviderSendWire(t *testing.T) {
	t.Parallel()

	provider, capture := newCapturingMailgunServer(t, testMailgunDomain, http.StatusOK, testMailgunQueued)

	sendAt := time.Date(2030, time.January, 2, 3, 4, 5, 0, time.FixedZone("EST", -5*3600))
	email := newProviderTestEmail(t)
	email.Subject = "Wire subject"
	email.Recipients = []string{"To Person <to@domain.com>", "second@domain.com"}
	email.RecipientsCc = []string{"cc@domain.com", "to@domain.com"}
	email.RecipientsBcc = []string{"bcc@domain.com"}
	email.ReplyToAddress = "Support <support@domain.com>"
	email.Tags = []string{"welcome", "Welcome", "", "digest"}
	email.Metadata = map[string]string{"user_id": "42", "plan": `{"tier":"pro"}`}
	email.SendAt = sendAt
	email.SetHeader("X-Campaign", "spring")
	email.AddInlineAttachment("logo.png", "image/png", "logo.png", []byte{0x89, 'P', 'N', 'G'})

	result, err := provider.Send(context.Background(), email)
	require.NoError(t, err)
	assert.Equal(t, testMailgunMessageID, result.MessageID)
	assert.Equal(t, mtypes.SendMessageResponse{ID: testMailgunMessageID, Message: "Queued. Thank you."}, result.Response)

	// Request line and authentication
	req := capture.request(t)
	assert.Equal(t, "/v3/"+testMailgunDomain+"/messages", req.path)
	assert.Equal(t, "api", req.username)
	assert.Equal(t, testMailgunAPIKey, req.password)

	// Envelope and content (duplicate recipients are removed)
	values := req.values
	assert.Equal(t, []string{`"No Reply" <no-reply@example.com>`}, values["from"])
	assert.Equal(t, []string{`"To Person" <to@domain.com>`, "second@domain.com"}, values["to"])
	assert.Equal(t, []string{"cc@domain.com"}, values["cc"])
	assert.Equal(t, []string{"bcc@domain.com"}, values["bcc"])
	assert.Equal(t, []string{"Wire subject"}, values["subject"])
	assert.Equal(t, []string{"Test"}, values["text"])
	assert.Equal(t, []string{"<html>Test</html>"}, values["html"])

	// Headers, including the importance headers
	assert.Equal(t, []string{`"Support" <support@domain.com>`}, values["h:Reply-To"])
	assert.Equal(t, []string{"spring"}, values["h:X-Campaign"])
	assert.Equal(t, []string{headerXPriorityValue}, values["h:"+headerXPriority])
	assert.Equal(t, []string{headerHighValue}, values["h:"+headerXMSMailPriority])
	assert.Equal(t, []string{headerHighValue}, values["h:"+headerImportance])

	// Tags (empty and case-insensitive duplicates dropped), metadata and options
	assert.Equal(t, []string{"welcome", "digest"}, values["o:tag"])
	assert.Equal(t, []string{"42"}, values["v:user_id"])
	assert.Equal(t, []string{`{"tier":"pro"}`}, values["v:plan"])
	assert.Equal(t, []string{"yes"}, values["o:tracking-clicks"])
	assert.Equal(t, []string{"yes"}, values["o:tracking-opens"])
	require.Len(t, values["o:deliverytime"], 1)
	deliveryTime, err := mail.ParseDate(values["o:deliverytime"][0])
	require.NoError(t, err)
	assert.True(t, sendAt.Equal(deliveryTime), "delivery time %s", deliveryTime)

	// Attachments; the inline attachment is named by its content id
	expected, err := os.ReadFile("examples/test-attachment-file.txt")
	require.NoError(t, err)
	assert.Equal(t, []mailgunFile{{name: "test-attachment-file.txt", content: string(expected)}}, req.files["attachment"])
	assert.Equal(t, []mailgunFile{{name: "logo.png", content: "\x89PNG"}}, req.files["inline"])
}

// TestMailgunProviderSendMinimal checks the sending domain defaults to the from
// address domain, tracking is explicitly disabled, and optional fields are omitted
func TestMailgunProviderSendMinimal(t *testing.T) {
	t.Parallel()

	provider, capture := newCapturingMailgunServer(t, "", http.StatusOK, testMailgunQueued)

	_, err := provider.Send(context.Background(), newValidEmail())
	require.NoError(t, err)

	req := capture.request(t)
	assert.Equal(t, "/v3/"+testDomainEmail+"/messages", req.path)
	assert.Equal(t, []string{"no-reply@example.com"}, req.values["from"])
	assert.Equal(t, []string{testRecipientSuccess}, req.values["to"])
	assert.Equal(t, []string{"no"}, req.values["o:tracking-clicks"])
	assert.Equal(t, []string{"no"}, req.values["o:tracking-opens"])
	for _, key := range []string{"cc", "bcc", "html", "h:Reply-To", "o:tag", "o:deliverytime"} {
		assert.NotContains(t, req.values, key)
	}
	assert.Empty(t, req.files)
}

// TestMailgunProviderOptions checks that MailgunOption customizes the message
func TestMailgunProviderOptions(t *testing.T) {
	t.Parallel()

	provider, capture := newCapturingMailgunServer(t, testMailgunDomain, http.StatusOK, testMailgunQueued)

	email := newValidEmail().With(
		MailgunOption(func(m *mailgun.PlainMessage) {
			m.EnableTestMode()
			m.AddDomain("mg.tenant.com")
		}),
		MailgunOption(nil),
		PostmarkOption(nil), // other providers' options are ignored
	)
	_, err := provider.Send(context.Background(), email)
	require.NoError(t, err)

	req := capture.request(t)
	assert.Equal(t, "/v3/mg.tenant.com/messages", req.path)
	assert.Equal(t, []string{"yes"}, req.values["o:testmode"])
}

// TestMailgunProviderErrors checks that API errors are wrapped in ErrMailgunError
// and the SDK error types are preserved
func TestMailgunProviderErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		body     string
		contains string
		status   int
	}{
		{"400 bad request", `{"message":"'from' parameter is not a valid address"}`, "not a valid address", http.StatusBadRequest},
		{"401 invalid api key", `{"message":"Invalid private key"}`, "Invalid private key", http.StatusUnauthorized},
		{"500 server error", `{"message":"Internal Server Error"}`, "Internal Server Error", http.StatusInternalServerError},
		{"429 rate limit", `{"message":"Too many requests"}`, "Too many requests", http.StatusTooManyRequests},
		{"200 with empty id", `{"id":"","message":"Queued. Thank you."}`, "empty message id", http.StatusOK},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			provider, _ := newCapturingMailgunServer(t, testMailgunDomain, test.status, test.body)
			_, err := provider.Send(context.Background(), newValidEmail())
			require.ErrorIs(t, err, ErrMailgunError)
			require.ErrorContains(t, err, test.contains)
			if test.status == http.StatusOK {
				return
			}

			// The SDK error is preserved for errors.As
			assert.Equal(t, test.status, mailgun.GetStatusFromErr(err))
			var rateLimited *mailgun.RateLimitedError
			if test.status == http.StatusTooManyRequests {
				require.ErrorAs(t, err, &rateLimited)
				require.NotNil(t, rateLimited.ResetAt)
				assert.Equal(t, time.Unix(1893456000, 0).UTC(), *rateLimited.ResetAt)
			} else {
				assert.NotErrorAs(t, err, &rateLimited)
			}
		})
	}
}

// TestMailgunProviderTransportError checks a client error is wrapped in ErrMailgunError
func TestMailgunProviderTransportError(t *testing.T) {
	t.Parallel()

	provider := NewMailgunProvider(mailgunClientFunc(func(context.Context, mailgun.Message) (mtypes.SendMessageResponse, error) {
		return mtypes.SendMessageResponse{}, errMailgunTransport
	}), testMailgunDomain)

	_, err := provider.Send(context.Background(), newValidEmail())
	require.ErrorIs(t, err, ErrMailgunError)
	require.ErrorIs(t, err, errMailgunTransport)
}

// TestMailgunProviderTooManyTags checks the Mailgun tag limit is enforced before sending
func TestMailgunProviderTooManyTags(t *testing.T) {
	t.Parallel()

	var calls int
	provider := NewMailgunProvider(mailgunClientFunc(func(context.Context, mailgun.Message) (mtypes.SendMessageResponse, error) {
		calls++
		return mtypes.SendMessageResponse{ID: testMailgunMessageID}, nil
	}), testMailgunDomain)

	email := newValidEmail()
	for i := range mailgun.MaxNumberOfTags {
		email.Tags = append(email.Tags, "tag-"+strconv.Itoa(i))
	}
	_, err := provider.Send(context.Background(), email)
	require.NoError(t, err, "the maximum number of tags is allowed")

	email.Tags = append(email.Tags, "one-too-many")
	_, err = provider.Send(context.Background(), email)
	require.ErrorIs(t, err, ErrMailgunError)
	require.ErrorContains(t, err, "tag limit")
	assert.Equal(t, 1, calls, "an email over the tag limit is never sent")
}

// TestMailgunProviderContextCanceled checks a canceled context aborts the send
func TestMailgunProviderContextCanceled(t *testing.T) {
	t.Parallel()

	provider, capture := newCapturingMailgunServer(t, testMailgunDomain, http.StatusOK, testMailgunQueued)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := provider.Send(ctx, newValidEmail())
	require.ErrorIs(t, err, ErrMailgunError)
	require.ErrorIs(t, err, context.Canceled)
	assert.Zero(t, capture.count())
}

// TestMailgunProviderInvalidEmail checks envelope and attachment errors
func TestMailgunProviderInvalidEmail(t *testing.T) {
	t.Parallel()

	provider := NewMailgunProvider(nil, testMailgunDomain)

	badRecipient := newValidEmail()
	badRecipient.Recipients = []string{"not an address"}
	_, err := provider.Send(context.Background(), badRecipient)
	require.ErrorIs(t, err, ErrInvalidRecipient)

	badAttachment := newValidEmail()
	badAttachment.AddAttachment(testFileName, "text/plain", errReader{})
	_, err = provider.Send(context.Background(), badAttachment)
	require.ErrorIs(t, err, io.ErrUnexpectedEOF)
}

// TestMailgunProviderSupportsFeature checks the Mailgun feature support
func TestMailgunProviderSupportsFeature(t *testing.T) {
	t.Parallel()

	provider := NewMailgunProvider(nil, "")
	for _, feature := range []Feature{FeatureMetadata, FeatureSendAt, FeatureTags, FeatureTrackClicks, FeatureTrackOpens} {
		assert.True(t, provider.SupportsFeature(feature), feature)
	}
	for _, feature := range []Feature{FeatureAutoText, FeatureIdempotencyKey, FeatureViewContentLink} {
		assert.False(t, provider.SupportsFeature(feature), feature)
	}
}

// TestMailgunTags checks empty and duplicate tags are dropped
func TestMailgunTags(t *testing.T) {
	t.Parallel()

	assert.Nil(t, mailgunTags(nil))
	assert.Empty(t, mailgunTags([]string{""}))
	assert.Equal(t, []string{"Welcome", "digest"}, mailgunTags([]string{"Welcome", "", "welcome", "digest", "WELCOME"}))
}

// TestNewMailgunProviderWithSDKClient checks the SDK client satisfies MailgunClient
func TestNewMailgunProviderWithSDKClient(t *testing.T) {
	t.Parallel()

	client := mailgun.NewMailgun(testMailgunAPIKey)
	provider := NewMailgunProvider(client, testMailgunDomain)
	assert.Same(t, client, provider.client)
	assert.Equal(t, testMailgunDomain, provider.domain)
}

// mailgunClientFunc adapts a function to MailgunClient
type mailgunClientFunc func(ctx context.Context, message mailgun.Message) (mtypes.SendMessageResponse, error)

// Send calls the function
func (f mailgunClientFunc) Send(ctx context.Context, message mailgun.Message) (mtypes.SendMessageResponse, error) {
	return f(ctx, message)
}
