package gomail

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	texttemplate "text/template"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errProviderDown is a test-only provider failure
var errProviderDown = errors.New("provider is down")

// TestMailService_NewEmail tests the method NewEmail()
func TestMailService_NewEmail(t *testing.T) {
	t.Parallel()

	service := new(MailService)
	service.AutoText = true
	service.EmailCSS = []byte("body{}")
	service.FromUsername = testUsernameEmail
	service.FromName = testFromNameEmail
	service.FromDomain = testDomainEmail
	service.Important = true
	service.TrackClicks = true
	service.TrackOpens = true

	email := service.NewEmail()
	assert.Equal(t, service.FromUsername+"@"+service.FromDomain, email.FromAddress)
	assert.Equal(t, email.FromAddress, email.ReplyToAddress)
	assert.Equal(t, service.FromName, email.FromName)
	assert.Equal(t, service.EmailCSS, email.CSS)
	assert.True(t, email.AutoText)
	assert.True(t, email.Important)
	assert.True(t, email.TrackClicks)
	assert.True(t, email.TrackOpens)
}

// ExampleMailService_NewEmail example using the NewEmail()
func ExampleMailService_NewEmail() {
	service := new(MailService)
	service.FromUsername = testUsernameEmail
	service.FromName = testFromNameEmail
	service.FromDomain = testDomainEmail

	email := service.NewEmail()
	fmt.Printf("new email with from address: %s", email.FromAddress)
	// output: new email with from address: no-reply@example.com
}

// BenchmarkMailService_NewEmail runs benchmark on NewEmail()
func BenchmarkMailService_NewEmail(b *testing.B) {
	service := new(MailService)
	service.FromUsername = testUsernameEmail
	service.FromName = testFromNameEmail
	service.FromDomain = testDomainEmail
	for b.Loop() {
		_ = service.NewEmail()
	}
}

// TestEmail_AddAttachment tests the attachment helpers
func TestEmail_AddAttachment(t *testing.T) {
	t.Parallel()

	email := &Email{}
	reader := strings.NewReader("data")
	email.AddAttachment("reader.txt", "text/plain", reader)
	email.AddAttachmentBytes("bytes.pdf", "application/pdf", []byte("%PDF"))
	email.AddInlineAttachment("logo.png", "image/png", "logo", []byte("png"))

	require.Len(t, email.Attachments, 3)
	assert.Equal(t, Attachment{FileName: "reader.txt", FileType: "text/plain", FileReader: reader}, email.Attachments[0])
	assert.Equal(t, Attachment{FileName: "bytes.pdf", FileType: "application/pdf", Content: []byte("%PDF")}, email.Attachments[1])
	assert.Equal(t, Attachment{FileName: "logo.png", FileType: "image/png", Content: []byte("png"), ContentID: "logo"}, email.Attachments[2])
}

// ExampleEmail_AddAttachment example using the AddAttachment()
func ExampleEmail_AddAttachment() {
	service := new(MailService)
	service.FromUsername = testUsernameEmail
	service.FromName = testFromNameEmail
	service.FromDomain = testDomainEmail

	email := service.NewEmail()
	email.AddAttachment("testName", "testType", strings.NewReader("contents"))

	fmt.Printf("attachment: %s", email.Attachments[0].FileName)
	// output: attachment: testName
}

// BenchmarkEmail_AddAttachment runs benchmark on AddAttachment()
func BenchmarkEmail_AddAttachment(b *testing.B) {
	email := new(Email)
	for b.Loop() {
		email.AddAttachment("testName", "testType", nil)
	}
}

// TestEmail_SetHeader checks headers are replaced case-insensitively
func TestEmail_SetHeader(t *testing.T) {
	t.Parallel()

	email := &Email{}
	email.SetHeader("X-Tag", "one")
	email.SetHeader("x-tag", "two")
	email.SetHeader("X-Other", "three")
	assert.Equal(t, map[string]string{"x-tag": "two", "X-Other": "three"}, email.Headers)
}

// TestEmail_SetListUnsubscribe checks the List-Unsubscribe headers
func TestEmail_SetListUnsubscribe(t *testing.T) {
	t.Parallel()

	t.Run("one-click", func(t *testing.T) {
		email := &Email{}
		require.NoError(t, email.SetListUnsubscribe(true, "https://example.com/u?id=1", "mailto:u@example.com?subject=unsubscribe"))
		assert.Equal(t, map[string]string{
			headerListUnsubscribe:     "<https://example.com/u?id=1>, <mailto:u@example.com?subject=unsubscribe>",
			headerListUnsubscribePost: listUnsubscribeOneClick,
		}, email.Headers)
		require.NoError(t, validateHeaders(email.Headers))
	})

	t.Run("without one-click removes the post header", func(t *testing.T) {
		email := &Email{}
		require.NoError(t, email.SetListUnsubscribe(true, "https://example.com/u"))
		require.NoError(t, email.SetListUnsubscribe(false, "mailto:u@example.com"))
		assert.Equal(t, map[string]string{headerListUnsubscribe: "<mailto:u@example.com>"}, email.Headers)
	})

	tests := []struct {
		name     string
		oneClick bool
		targets  []string
	}{
		{"no targets", false, nil},
		{"unsupported scheme", false, []string{"ftp://example.com/u"}},
		{"relative url", false, []string{"/unsubscribe"}},
		{"angle bracket", false, []string{"https://example.com/<u>"}},
		{"comma", false, []string{"https://example.com/a,b"}},
		{"line break", false, []string{"https://example.com/u\r\nBcc: x"}},
		{"invalid url", false, []string{"https://exa mple.com"}},
		{"one-click needs https", true, []string{"mailto:u@example.com", "http://example.com/u"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			email := &Email{}
			err := email.SetListUnsubscribe(test.oneClick, test.targets...)
			require.ErrorIs(t, err, ErrInvalidListUnsubscribe)
			assert.Empty(t, email.Headers)
		})
	}
}

// TestEmail_With checks provider options are appended
func TestEmail_With(t *testing.T) {
	t.Parallel()

	email := &Email{}
	first, second := PostmarkOption(nil), SendGridOption(nil)
	assert.Same(t, email, email.With(first))
	email.With(second)
	assert.Len(t, email.ProviderOptions, 2)
}

// TestEmail_ParseTemplate tests the method ParseTemplate()
func TestEmail_ParseTemplate(t *testing.T) {
	t.Parallel()

	email := &Email{}
	parsedTemplate, err := email.ParseTemplate(filepath.Join("examples", "example_template.txt"))
	require.NoError(t, err)
	assert.Equal(t, "example_template.txt", parsedTemplate.Name())

	_, err = email.ParseTemplate(filepath.Join("examples", "missing_file.txt"))
	require.Error(t, err)
}

// TestEmail_ParseTextTemplate tests the method ParseTextTemplate()
func TestEmail_ParseTextTemplate(t *testing.T) {
	t.Parallel()

	email := &Email{}
	parsedTemplate, err := email.ParseTextTemplate(filepath.Join("examples", "example_template.txt"))
	require.NoError(t, err)
	assert.Equal(t, "example_template.txt", parsedTemplate.Name())

	_, err = email.ParseTextTemplate(filepath.Join("examples", "missing_file.txt"))
	require.Error(t, err)
}

// writeTemplateFile writes a template file into a temporary directory
func writeTemplateFile(t *testing.T, name, content string) string {
	t.Helper()

	file := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(file, []byte(content), 0o600))
	return file
}

// TestEmail_ApplyTemplatesPlainTextIsNotEscaped checks plain-text output is
// never HTML-escaped, whichever template package parsed it
func TestEmail_ApplyTemplatesPlainTextIsNotEscaped(t *testing.T) {
	t.Parallel()

	file := writeTemplateFile(t, "reset.txt", "Hi {{.Name}}, reset: {{.URL}} {{.Plus}}")
	data := map[string]string{"Name": `O'Brien & "Co" <team>`, "URL": "https://x.io/r?a=1&b=2", "Plus": "a+b"}
	expected := `Hi O'Brien & "Co" <team>, reset: https://x.io/r?a=1&b=2 a+b`

	t.Run("text template", func(t *testing.T) {
		email := &Email{}
		textTemplate, err := email.ParseTextTemplate(file)
		require.NoError(t, err)
		require.NoError(t, email.ApplyTemplates(nil, textTemplate, data))
		assert.Equal(t, expected, email.PlainTextContent)
	})

	t.Run("legacy html template", func(t *testing.T) {
		email := &Email{}
		htmlTemplate, err := email.ParseTemplate(file)
		require.NoError(t, err)
		require.NoError(t, email.ApplyTemplates(nil, htmlTemplate, data))
		assert.Equal(t, expected, email.PlainTextContent)
	})
}

// TestEmail_ApplyTemplates tests the method ApplyTemplates() with the examples
func TestEmail_ApplyTemplates(t *testing.T) {
	t.Parallel()

	service := &MailService{FromDomain: testDomainEmail, FromName: testFromNameEmail, FromUsername: testUsernameEmail}
	email := service.NewEmail()

	textTemplate, err := email.ParseTextTemplate(filepath.Join("examples", "example_template.txt"))
	require.NoError(t, err)

	email.CSS, err = os.ReadFile(filepath.Join("examples", "example_theme.css"))
	require.NoError(t, err)

	var htmlTemplate *template.Template
	htmlTemplate, err = email.ParseHTMLTemplate(filepath.Join("examples", "example_template_css.html"))
	require.NoError(t, err)
	assert.Equal(t, "example_template_css.html", htmlTemplate.Name())

	// Apply the service data
	require.NoError(t, email.ApplyTemplates(htmlTemplate, textTemplate, service))
	assert.Contains(t, email.PlainTextContent, "Sending email from: "+testFromNameEmail)
	assert.Contains(t, email.HTMLContent, `class="example-class" style="color: #045704; font-weight: bold;"`)
	assert.NotContains(t, email.HTMLContent, "<style")

	// Apply the email itself as data
	require.NoError(t, email.ApplyTemplates(htmlTemplate, textTemplate, nil))

	// Error from a missing template variable
	require.Error(t, email.ApplyTemplates(htmlTemplate, textTemplate, "no data"))
	require.Error(t, email.ApplyTemplates(nil, textTemplate, "no data"))
}

// TestEmail_ApplyTemplatesNilTemplates checks nil and typed-nil templates are skipped
func TestEmail_ApplyTemplatesNilTemplates(t *testing.T) {
	t.Parallel()

	email := &Email{HTMLContent: "keep", PlainTextContent: "keep"}
	var htmlNil *template.Template
	var textNil *texttemplate.Template

	require.NoError(t, email.ApplyTemplates(nil, nil, nil))
	require.NoError(t, email.ApplyTemplates(htmlNil, htmlNil, nil))
	require.NoError(t, email.ApplyTemplates(nil, textNil, nil))
	assert.Equal(t, "keep", email.HTMLContent)
	assert.Equal(t, "keep", email.PlainTextContent)
}

// TestEmail_ParseHTMLTemplateKeepsTemplateActions checks CSS inlining after
// execution keeps loops inside tables and actions inside attributes intact
func TestEmail_ParseHTMLTemplateKeepsTemplateActions(t *testing.T) {
	t.Parallel()

	file := writeTemplateFile(t, "table.html", `<html><head><style>{{.Styles}}</style></head><body>`+
		`<table>{{range .Items}}<tr><td class="c">{{.}}</td></tr>{{end}}</table>`+
		`<a href="{{.URL}}" {{if .Title}}title="{{.Title}}"{{end}}>go</a></body></html>`)

	email := &Email{CSS: []byte(".c { color: red; }")}
	htmlTemplate, err := email.ParseHTMLTemplate(file)
	require.NoError(t, err)

	data := map[string]any{"Items": []string{"first", "second"}, "URL": "https://x.io/?a=1&b=2", "Title": "Go"}
	require.NoError(t, email.ApplyTemplates(htmlTemplate, nil, data))

	assert.Contains(t, email.HTMLContent, `<td class="c" style="color: red;">first</td>`)
	assert.Contains(t, email.HTMLContent, `<td class="c" style="color: red;">second</td>`)
	assert.Contains(t, email.HTMLContent, `title="Go"`)
	assert.Contains(t, email.HTMLContent, `href="https://x.io/?a=1&amp;b=2"`)
}

// TestEmail_ParseHTMLTemplateWithoutStyles checks templates without the
// placeholder (or without CSS) are not inlined
func TestEmail_ParseHTMLTemplateWithoutStyles(t *testing.T) {
	t.Parallel()

	email := &Email{}
	htmlTemplate, err := email.ParseHTMLTemplate(filepath.Join("examples", "example_template_css.html"))
	require.NoError(t, err)
	assert.Nil(t, htmlTemplate.Lookup(inlineCSSTemplateName))

	htmlTemplate, err = email.ParseHTMLTemplate(filepath.Join("examples", "example_template.html"))
	require.NoError(t, err)
	assert.Nil(t, htmlTemplate.Lookup(inlineCSSTemplateName))

	_, err = email.ParseHTMLTemplate(filepath.Join("examples", "missing_file.html"))
	require.Error(t, err)
}

// TestEmail_ParseHTMLTemplateErrors checks the parse error paths
func TestEmail_ParseHTMLTemplateErrors(t *testing.T) {
	t.Parallel()

	t.Run("invalid template", func(t *testing.T) {
		file := writeTemplateFile(t, "broken.html", "<html><head><style>{{.Styles}}</style></head><body>{{.Broken}</body></html>")
		email := &Email{CSS: []byte("body { color: red; }")}
		parsed, err := email.ParseHTMLTemplate(file)
		require.Error(t, err)
		assert.Nil(t, parsed)
	})

	t.Run("invalid css", func(t *testing.T) {
		file := writeTemplateFile(t, "bad_css.html", "<html><head><style>{{.Styles}}</style></head><body></body></html>")
		email := &Email{CSS: []byte("}")}
		parsed, err := email.ParseHTMLTemplate(file)
		require.Error(t, err)
		assert.Nil(t, parsed)
	})
}

// TestEmail_ApplyTemplatesInlineError checks an inliner failure is returned
func TestEmail_ApplyTemplatesInlineError(t *testing.T) {
	t.Parallel()

	file := writeTemplateFile(t, "inline.html", "<html><head><style>{{.Styles}}</style></head><body>{{.Extra}}</body></html>")
	email := &Email{CSS: []byte("body { color: red; }")}
	htmlTemplate, err := email.ParseHTMLTemplate(file)
	require.NoError(t, err)

	// The rendered HTML contains a second, malformed stylesheet
	err = email.ApplyTemplates(htmlTemplate, nil, map[string]template.HTML{"Extra": "<style>}</style>"})
	require.Error(t, err)
}

// TestIsNilTemplate checks nil template detection
func TestIsNilTemplate(t *testing.T) {
	t.Parallel()

	var htmlNil *template.Template
	assert.True(t, isNilTemplate(nil))
	assert.True(t, isNilTemplate(htmlNil))
	assert.False(t, isNilTemplate(texttemplate.New("x")))
}

// TestMailService_Send checks a successful send returns the provider result
func TestMailService_Send(t *testing.T) {
	t.Parallel()

	provider := &fakeProvider{result: &SendResult{MessageID: testMessageID, Response: "raw"}}
	service := newTestService(t, map[ServiceProvider]Provider{Postmark: provider})

	result, err := service.Send(context.Background(), newValidEmail(), Postmark)
	require.NoError(t, err)
	assert.Equal(t, &SendResult{MessageID: testMessageID, Provider: Postmark, Response: "raw"}, result)
	assert.True(t, provider.hadDeadline(), "sends are bounded by the default timeout")

	require.NoError(t, service.SendEmail(context.Background(), newValidEmail(), Postmark))
	assert.Equal(t, 2, provider.callCount())
}

// TestMailService_SendNilResult checks a provider returning no result
func TestMailService_SendNilResult(t *testing.T) {
	t.Parallel()

	service := newTestService(t, map[ServiceProvider]Provider{SMTP: &fakeProvider{}})
	result, err := service.Send(context.Background(), newValidEmail(), SMTP)
	require.NoError(t, err)
	assert.Equal(t, &SendResult{Provider: SMTP}, result)
}

// TestMailService_SendFailover checks providers are tried in order
func TestMailService_SendFailover(t *testing.T) {
	t.Parallel()

	t.Run("second provider succeeds", func(t *testing.T) {
		first := &fakeProvider{err: errProviderDown}
		second := &fakeProvider{result: &SendResult{MessageID: "second"}}
		service := newTestService(t, map[ServiceProvider]Provider{SendGrid: first, Postmark: second})

		result, err := service.Send(context.Background(), newValidEmail(), SendGrid, Postmark)
		require.NoError(t, err)
		assert.Equal(t, Postmark, result.Provider)
		assert.Equal(t, 1, first.callCount())
	})

	t.Run("all providers fail", func(t *testing.T) {
		first := &fakeProvider{err: errProviderDown}
		second := &fakeProvider{err: ErrPostmarkError}
		service := newTestService(t, map[ServiceProvider]Provider{SendGrid: first, Postmark: second})

		_, err := service.Send(context.Background(), newValidEmail(), SendGrid, Postmark)
		require.ErrorIs(t, err, errProviderDown)
		require.ErrorIs(t, err, ErrPostmarkError)
		assert.Contains(t, err.Error(), "SendGrid: provider is down")
		assert.Contains(t, err.Error(), "Postmark: error from postmark")
	})

	t.Run("default order is the available providers", func(t *testing.T) {
		first := &fakeProvider{err: errProviderDown}
		second := &fakeProvider{result: &SendResult{}}
		service := newTestService(t, map[ServiceProvider]Provider{Mandrill: first, Resend: second})

		result, err := service.Send(context.Background(), newValidEmail())
		require.NoError(t, err)
		assert.Equal(t, Resend, result.Provider)
	})

	t.Run("canceled context stops the failover", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		first := &fakeProvider{sendFn: func(context.Context, *Email) (*SendResult, error) {
			cancel()
			return nil, context.Canceled
		}}
		second := &fakeProvider{result: &SendResult{}}
		service := newTestService(t, map[ServiceProvider]Provider{SendGrid: first, Postmark: second})

		_, err := service.Send(ctx, newValidEmail(), SendGrid, Postmark)
		require.ErrorIs(t, err, context.Canceled)
		assert.Zero(t, second.callCount())
	})

	t.Run("attachments are resent to the next provider", func(t *testing.T) {
		var seen []string
		read := func(_ context.Context, email *Email) (*SendResult, error) {
			attachments, err := readAttachments(email)
			if err != nil {
				return nil, err
			}
			seen = append(seen, string(attachments[0].content))
			return nil, errProviderDown
		}
		service := newTestService(t, map[ServiceProvider]Provider{
			SendGrid: &fakeProvider{sendFn: read},
			Postmark: &fakeProvider{sendFn: read},
		})

		email := newValidEmail()
		email.AddAttachment(testFileName, "text/plain", strings.NewReader("contents"))
		_, err := service.Send(context.Background(), email, SendGrid, Postmark)
		require.Error(t, err)
		assert.Equal(t, []string{"contents", "contents"}, seen)
	})
}

// TestMailService_SendProviderNotFound checks unknown providers
func TestMailService_SendProviderNotFound(t *testing.T) {
	t.Parallel()

	service := newTestService(t, map[ServiceProvider]Provider{Mandrill: &fakeProvider{}})

	err := service.SendEmail(context.Background(), newValidEmail(), AwsSes)
	require.ErrorIs(t, err, ErrProviderNotFound)
	assert.Equal(t, "service provider: AwsSes was not in the list of available service providers: [Mandrill], email not sent: "+
		ErrProviderNotFound.Error(), err.Error())

	_, err = service.Send(context.Background(), newValidEmail(), Mandrill, ServiceProvider(42))
	require.ErrorIs(t, err, ErrProviderNotFound)

	_, err = new(MailService).Send(context.Background(), newValidEmail())
	require.ErrorIs(t, err, ErrNoServiceProvider)
}

// TestMailService_SendFeatures checks unsupported feature handling
func TestMailService_SendFeatures(t *testing.T) {
	t.Parallel()

	newProvider := func() *featureFakeProvider {
		return &featureFakeProvider{fakeProvider{result: &SendResult{}, supported: []Feature{FeatureTags}}}
	}

	t.Run("unsupported feature is logged", func(t *testing.T) {
		var logs bytes.Buffer
		provider := newProvider()
		service := newTestService(t, map[ServiceProvider]Provider{SMTP: provider})
		service.Logger = slog.New(slog.NewTextHandler(&logs, nil))

		email := newValidEmail()
		email.TrackOpens = true
		email.Tags = []string{"supported"}
		_, err := service.Send(context.Background(), email, SMTP)
		require.NoError(t, err)
		assert.Equal(t, 1, provider.callCount())
		assert.Contains(t, logs.String(), "level=WARN")
		assert.Contains(t, logs.String(), "provider=SMTP")
		assert.Contains(t, logs.String(), "features=[track_opens]")
	})

	t.Run("supported features are not logged", func(t *testing.T) {
		var logs bytes.Buffer
		service := newTestService(t, map[ServiceProvider]Provider{SMTP: newProvider()})
		service.Logger = slog.New(slog.NewTextHandler(&logs, nil))

		email := newValidEmail()
		email.Tags = []string{"supported"}
		_, err := service.Send(context.Background(), email, SMTP)
		require.NoError(t, err)
		assert.Empty(t, logs.String())
	})

	t.Run("strict features returns an error", func(t *testing.T) {
		provider := newProvider()
		service := newTestService(t, map[ServiceProvider]Provider{SMTP: provider})
		service.StrictFeatures = true

		email := newValidEmail()
		email.TrackClicks = true
		_, err := service.Send(context.Background(), email, SMTP)
		require.ErrorIs(t, err, ErrUnsupportedFeature)
		assert.Contains(t, err.Error(), "SMTP does not support [track_clicks]")
		assert.Zero(t, provider.callCount())
	})

	t.Run("unsupported send at is always an error", func(t *testing.T) {
		provider := newProvider()
		service := newTestService(t, map[ServiceProvider]Provider{SMTP: provider})

		email := newValidEmail()
		email.SendAt = time.Now().Add(time.Hour)
		_, err := service.Send(context.Background(), email, SMTP)
		require.ErrorIs(t, err, ErrUnsupportedFeature)
		assert.Zero(t, provider.callCount())
	})

	t.Run("send at fails over to a provider that supports it", func(t *testing.T) {
		scheduler := &featureFakeProvider{fakeProvider{result: &SendResult{}, supported: []Feature{FeatureSendAt}}}
		service := newTestService(t, map[ServiceProvider]Provider{SMTP: newProvider(), SendGrid: scheduler})

		email := newValidEmail()
		email.SendAt = time.Now().Add(time.Hour)
		result, err := service.Send(context.Background(), email, SMTP, SendGrid)
		require.NoError(t, err)
		assert.Equal(t, SendGrid, result.Provider)
	})
}

// TestMailService_SendTimeout checks the per-send timeout configuration
func TestMailService_SendTimeout(t *testing.T) {
	t.Parallel()

	t.Run("timeout ends a slow send", func(t *testing.T) {
		provider := &fakeProvider{sendFn: func(ctx context.Context, _ *Email) (*SendResult, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		}}
		service := newTestService(t, map[ServiceProvider]Provider{SMTP: provider})
		service.SendTimeout = 20 * time.Millisecond

		_, err := service.Send(context.Background(), newValidEmail(), SMTP)
		require.ErrorIs(t, err, context.DeadlineExceeded)
	})

	t.Run("negative timeout disables it", func(t *testing.T) {
		provider := &fakeProvider{result: &SendResult{}}
		service := newTestService(t, map[ServiceProvider]Provider{SMTP: provider})
		service.SendTimeout = -1

		_, err := service.Send(context.Background(), newValidEmail(), SMTP)
		require.NoError(t, err)
		assert.False(t, provider.hadDeadline())
	})
}

// TestMailService_SendReusesEmail checks an email with reader attachments can be sent twice
func TestMailService_SendReusesEmail(t *testing.T) {
	t.Parallel()

	var sizes []int
	provider := &fakeProvider{sendFn: func(_ context.Context, email *Email) (*SendResult, error) {
		attachments, err := readAttachments(email)
		if err != nil {
			return nil, err
		}
		sizes = append(sizes, len(attachments[0].content))
		return &SendResult{}, nil
	}}
	service := newTestService(t, map[ServiceProvider]Provider{SMTP: provider})

	email := newValidEmail()
	email.AddAttachment(testFileName, "text/plain", strings.NewReader("twelve bytes"))
	for range 2 {
		_, err := service.Send(context.Background(), email, SMTP)
		require.NoError(t, err)
	}
	assert.Equal(t, []int{12, 12}, sizes, "the second send must not get an empty attachment")
}

// TestMailService_SendAttachmentLimit checks the attachment size limit
func TestMailService_SendAttachmentLimit(t *testing.T) {
	t.Parallel()

	service := newTestService(t, map[ServiceProvider]Provider{SMTP: &fakeProvider{result: &SendResult{}}})
	service.MaxAttachmentSize = 4

	email := newValidEmail()
	email.AddAttachmentBytes(testFileName, "text/plain", []byte("12345"))
	_, err := service.Send(context.Background(), email, SMTP)
	require.ErrorIs(t, err, ErrAttachmentsTooLarge)

	service.MaxAttachmentSize = -1
	_, err = service.Send(context.Background(), email, SMTP)
	require.NoError(t, err)
}

// TestMailService_SendValidation checks invalid emails are rejected before sending
func TestMailService_SendValidation(t *testing.T) {
	t.Parallel()

	provider := &fakeProvider{result: &SendResult{}}
	service := newTestService(t, map[ServiceProvider]Provider{SMTP: provider})

	tests := []struct {
		name     string
		setup    func(email *Email)
		expected error
	}{
		{"missing subject", func(email *Email) { email.Subject = "" }, ErrMissingSubject},
		{"missing content", func(email *Email) { email.PlainTextContent = "" }, ErrMissingContent},
		{"missing recipient", func(email *Email) { email.Recipients = nil }, ErrMissingRecipient},
		{"too many to", func(email *Email) { email.Recipients = make([]string, 51) }, ErrMaxToRecipientsReached},
		{"too many cc", func(email *Email) { email.RecipientsCc = make([]string, 51) }, ErrMaxCcRecipientsReached},
		{"too many bcc", func(email *Email) { email.RecipientsBcc = make([]string, 51) }, ErrMaxBccRecipientsReached},
		{"invalid from", func(email *Email) { email.FromAddress = "nope" }, ErrInvalidFromAddress},
		{"invalid recipient", func(email *Email) { email.Recipients = []string{"nope"} }, ErrInvalidRecipient},
		{"invalid header", func(email *Email) { email.SetHeader("Bcc", "x@example.com") }, ErrInvalidHeader},
		{"invalid metadata", func(email *Email) { email.Metadata = map[string]string{"": "v"} }, ErrInvalidMetadata},
		{"invalid attachment", func(email *Email) { email.Attachments = []Attachment{{FileName: "a"}} }, ErrInvalidAttachment},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			email := newValidEmail()
			test.setup(email)
			_, err := service.Send(context.Background(), email, SMTP)
			require.ErrorIs(t, err, test.expected)
		})
	}

	t.Run("nil email", func(t *testing.T) {
		_, err := service.Send(context.Background(), nil, SMTP)
		require.ErrorIs(t, err, ErrMissingContent)
	})

	assert.Zero(t, provider.callCount())
}

// TestMailService_SendConcurrent checks concurrent sends through one service
func TestMailService_SendConcurrent(t *testing.T) {
	t.Parallel()

	provider := &fakeProvider{result: &SendResult{}}
	service := newTestService(t, map[ServiceProvider]Provider{SMTP: provider})

	errs := make(chan error, 20)
	for range 20 {
		go func() {
			email := newValidEmail()
			email.AddAttachment(testFileName, "text/plain", strings.NewReader("data"))
			_, err := service.Send(context.Background(), email, SMTP)
			errs <- err
		}()
	}
	for range 20 {
		require.NoError(t, <-errs)
	}
	assert.Equal(t, 20, provider.callCount())
}
