package gomail

import (
	"bytes"
	"crypto/rand"
	"io"
	"mime"
	"net/mail"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testMIMEDate returns the fixed date used when building test messages
func testMIMEDate() time.Time {
	return time.Date(2026, time.March, 4, 5, 6, 7, 0, time.UTC)
}

// buildTestMIME prepares and renders an email like the SMTP and SES providers do
func buildTestMIME(t *testing.T, email *Email, messageID string) []byte {
	t.Helper()

	env, err := parseEnvelope(email)
	require.NoError(t, err)
	attachments, err := readAttachments(email)
	require.NoError(t, err)
	raw, err := buildMIME(email, env, attachments, messageID, testMIMEDate())
	require.NoError(t, err)
	return raw
}

// TestBuildMIMEHeaders checks the message headers
func TestBuildMIMEHeaders(t *testing.T) {
	t.Parallel()

	email := newValidEmail()
	email.FromName = "Acme, Inc."
	email.Recipients = []string{"one@example.com", "Two Person <two@example.com>"}
	email.RecipientsCc = []string{testCcAddress}
	email.RecipientsBcc = []string{"secret@example.com"}
	email.ReplyToAddress = "Support <support@example.com>"
	email.Subject = "Héllo wörld"
	email.Important = true
	email.Headers = map[string]string{"X-Campaign": "spring", "List-Unsubscribe": "<https://example.com/u>"}

	raw := buildTestMIME(t, email, "<id@example.com>")
	parsed := parseMIME(t, raw)

	t.Run("from is quoted", func(t *testing.T) {
		from, err := parsed.header.AddressList("From")
		require.NoError(t, err)
		require.Len(t, from, 1)
		assert.Equal(t, "Acme, Inc.", from[0].Name)
		assert.Equal(t, "no-reply@example.com", from[0].Address)
	})

	t.Run("single to header with every recipient", func(t *testing.T) {
		assert.Len(t, parsed.header["To"], 1)
		to, err := parsed.header.AddressList("To")
		require.NoError(t, err)
		require.Len(t, to, 2)
		assert.Equal(t, "one@example.com", to[0].Address)
		assert.Equal(t, "Two Person", to[1].Name)
	})

	t.Run("cc and reply-to", func(t *testing.T) {
		cc, err := parsed.header.AddressList("Cc")
		require.NoError(t, err)
		require.Len(t, cc, 1)
		replyTo, err := parsed.header.AddressList("Reply-To")
		require.NoError(t, err)
		assert.Equal(t, "support@example.com", replyTo[0].Address)
	})

	t.Run("bcc is never written", func(t *testing.T) {
		assert.Empty(t, parsed.header.Get("Bcc"))
		assert.NotContains(t, string(raw), "secret@example.com")
	})

	t.Run("subject is encoded", func(t *testing.T) {
		decoded, err := new(mime.WordDecoder).DecodeHeader(parsed.header.Get("Subject"))
		require.NoError(t, err)
		assert.Equal(t, "Héllo wörld", decoded)
	})

	t.Run("date, message id and version", func(t *testing.T) {
		date, err := parsed.header.Date()
		require.NoError(t, err)
		assert.True(t, date.Equal(testMIMEDate()))
		assert.Equal(t, "<id@example.com>", parsed.header.Get("Message-Id"))
		assert.Equal(t, "1.0", parsed.header.Get("Mime-Version"))
	})

	t.Run("custom and importance headers", func(t *testing.T) {
		assert.Equal(t, "spring", parsed.header.Get("X-Campaign"))
		assert.Equal(t, "<https://example.com/u>", parsed.header.Get("List-Unsubscribe"))
		assert.Equal(t, headerXPriorityValue, parsed.header.Get(headerXPriority))
		assert.Equal(t, headerHighValue, parsed.header.Get(headerImportance))
	})

	t.Run("no message id when empty", func(t *testing.T) {
		noID := parseMIME(t, buildTestMIME(t, newValidEmail(), ""))
		assert.Empty(t, noID.header.Get("Message-Id"))
	})
}

// TestBuildMIMEStructure checks the MIME tree for each body/attachment combination
func TestBuildMIMEStructure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setup     func(email *Email)
		root      string
		leafTypes []string
	}{
		{
			name:      "plain text only",
			setup:     func(_ *Email) {},
			root:      mimeTypePlain,
			leafTypes: []string{mimeTypePlain},
		},
		{
			name: "html only",
			setup: func(email *Email) {
				email.PlainTextContent = ""
				email.HTMLContent = "<p>Hi</p>"
			},
			root:      mimeTypeHTML,
			leafTypes: []string{mimeTypeHTML},
		},
		{
			name:      "text and html",
			setup:     func(email *Email) { email.HTMLContent = "<p>Hi</p>" },
			root:      mimeTypeAlternative,
			leafTypes: []string{mimeTypePlain, mimeTypeHTML},
		},
		{
			name: "attachment",
			setup: func(email *Email) {
				email.HTMLContent = "<p>Hi</p>"
				email.AddAttachmentBytes("report.pdf", "application/pdf", []byte("%PDF-1.4"))
			},
			root:      mimeTypeMixed,
			leafTypes: []string{mimeTypePlain, mimeTypeHTML, "application/pdf"},
		},
		{
			name: "inline image with html only",
			setup: func(email *Email) {
				email.PlainTextContent = ""
				email.HTMLContent = `<img src="cid:logo">`
				email.AddInlineAttachment("logo.png", "image/png", "logo", []byte("png"))
			},
			root:      mimeTypeRelated,
			leafTypes: []string{mimeTypeHTML, "image/png"},
		},
		{
			name: "inline image and attachment with both bodies",
			setup: func(email *Email) {
				email.HTMLContent = `<img src="cid:logo">`
				email.AddInlineAttachment("logo.png", "image/png", "logo", []byte("png"))
				email.AddAttachmentBytes(testFileName, "text/csv", []byte("a,b"))
			},
			root:      mimeTypeMixed,
			leafTypes: []string{mimeTypePlain, mimeTypeHTML, "image/png", "text/csv"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			email := newValidEmail()
			test.setup(email)

			parsed := parseMIME(t, buildTestMIME(t, email, ""))
			assert.Equal(t, test.root, parsed.root)
			assert.Equal(t, test.leafTypes, parsed.mediaTypes())
		})
	}
}

// TestBuildMIMERelatedType checks the multipart/related root type parameter
func TestBuildMIMERelatedType(t *testing.T) {
	t.Parallel()

	email := newValidEmail()
	email.HTMLContent = `<img src="cid:logo">`
	email.AddInlineAttachment("logo.png", "image/png", "logo", []byte("png"))

	parsed := parseMIME(t, buildTestMIME(t, email, ""))
	mediaType, params, err := mime.ParseMediaType(parsed.header.Get("Content-Type"))
	require.NoError(t, err)
	assert.Equal(t, mimeTypeRelated, mediaType)
	assert.Equal(t, mimeTypeAlternative, params["type"])
	assert.NotEmpty(t, params["boundary"])
}

// TestBuildMIMEContent checks the body and attachment content round trips
func TestBuildMIMEContent(t *testing.T) {
	t.Parallel()

	binary := make([]byte, 1000)
	_, err := rand.Read(binary)
	require.NoError(t, err)

	email := newValidEmail()
	email.PlainTextContent = "Line one\nLine two with ünïcode and a very long line " + strings.Repeat("x", 200)
	email.HTMLContent = `<p style="color:red">Hi = there</p>`
	email.AddAttachmentBytes("données.bin", "", binary)
	email.AddInlineAttachment("logo.png", "image/png", "logo@example", []byte("png-bytes"))

	raw := buildTestMIME(t, email, "")
	parsed := parseMIME(t, raw)

	t.Run("text is quoted-printable utf-8", func(t *testing.T) {
		text := parsed.part(t, mimeTypePlain)
		assert.Equal(t, strings.ReplaceAll(email.PlainTextContent, "\n", "\r\n"), text.content)
		assert.Contains(t, string(raw), "Content-Type: text/plain; charset=utf-8")
	})

	t.Run("html round trips", func(t *testing.T) {
		assert.Equal(t, email.HTMLContent, parsed.part(t, mimeTypeHTML).content)
	})

	t.Run("binary attachment round trips with an encoded filename", func(t *testing.T) {
		att := parsed.part(t, defaultAttachType)
		assert.Equal(t, string(binary), att.content)
		assert.Equal(t, "attachment", att.disposition)
		assert.Equal(t, "données.bin", att.filename)
	})

	t.Run("inline attachment has a content id", func(t *testing.T) {
		att := parsed.part(t, "image/png")
		assert.Equal(t, "png-bytes", att.content)
		assert.Equal(t, "inline", att.disposition)
		assert.Equal(t, []string{"<logo@example>"}, att.header["Content-Id"])
	})

	t.Run("lines stay within the limit", func(t *testing.T) {
		for _, line := range strings.Split(string(raw), "\r\n") {
			assert.LessOrEqual(t, len(line), 998, "line too long: %q", line)
		}
	})

	t.Run("every line ends with crlf", func(t *testing.T) {
		assert.NotRegexp(t, "[^\r]\n", string(raw))
	})
}

// TestBuildMIMEFoldsLongHeaders checks that long recipient lists are folded
func TestBuildMIMEFoldsLongHeaders(t *testing.T) {
	t.Parallel()

	email := newValidEmail()
	email.Recipients = nil
	for i := range 50 {
		email.Recipients = append(email.Recipients, "recipient"+strings.Repeat("x", i%7)+string(rune('a'+i%26))+"@example.com")
	}
	email.Subject = strings.Repeat("word ", 60)

	raw := buildTestMIME(t, email, "")
	for _, line := range strings.Split(string(raw), "\r\n") {
		assert.LessOrEqual(t, len(line), headerFoldLength)
	}

	parsed := parseMIME(t, raw)
	to, err := parsed.header.AddressList("To")
	require.NoError(t, err)
	assert.Len(t, to, 50)
	assert.Equal(t, strings.TrimSpace(email.Subject), strings.TrimSpace(parsed.header.Get("Subject")))
}

// TestBuildMIMEStripsLineBreaks checks header values cannot inject headers
func TestBuildMIMEStripsLineBreaks(t *testing.T) {
	t.Parallel()

	email := newValidEmail()
	email.Subject = "Hello\r\nBcc: attacker@example.com"
	email.FromName = "Name\nX-Evil: 1"

	parsed := parseMIME(t, buildTestMIME(t, email, ""))
	assert.Empty(t, parsed.header.Get("Bcc"))
	assert.Empty(t, parsed.header.Get("X-Evil"))
	assert.Equal(t, "Hello Bcc: attacker@example.com", parsed.header.Get("Subject"))
}

// TestFoldHeader checks header folding edge cases
func TestFoldHeader(t *testing.T) {
	t.Parallel()

	long := "Subject: " + strings.Repeat("a", 100)
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"short line is unchanged", "Subject: hi", "Subject: hi"},
		{"long word folds after the name", long, "Subject:\r\n " + strings.Repeat("a", 100)},
		{"no whitespace cannot fold", strings.Repeat("a", 100), strings.Repeat("a", 100)},
		{
			"folds at the last space within the limit",
			"X: " + strings.Repeat("a ", 50),
			"X: " + strings.TrimSuffix(strings.Repeat("a ", 38), " ") + "\r\n " + strings.TrimSuffix(strings.Repeat("a ", 12), " ") + " ",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			folded := foldHeader(test.input)
			assert.Equal(t, test.expected, folded)
			assert.Equal(t, test.input, strings.ReplaceAll(folded, "\r\n", ""), "unfolding must restore the value")
		})
	}

	t.Run("never leaves a whitespace-only line", func(t *testing.T) {
		for _, spaces := range []int{100, 200} {
			folded := foldHeader("X: a" + strings.Repeat(" ", spaces) + "b")
			for _, line := range strings.Split(folded, "\r\n") {
				assert.NotEmpty(t, strings.TrimSpace(line))
			}
		}
	})

	t.Run("folds at the first space past the limit", func(t *testing.T) {
		long := "X-" + strings.Repeat("a", 100)
		assert.Equal(t, long+"\r\n b", foldHeader(long+" b"))
	})
}

// TestNewMessageID checks the generated Message-ID format
func TestNewMessageID(t *testing.T) {
	t.Parallel()

	id, err := newMessageID(&mail.Address{Address: testAddressA})
	require.NoError(t, err)
	assert.Regexp(t, `^<[0-9a-f]{32}@example\.com>$`, id)

	other, err := newMessageID(&mail.Address{Address: testAddressA})
	require.NoError(t, err)
	assert.NotEqual(t, id, other)

	local, err := newMessageID(&mail.Address{Address: "nodomain"})
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(local, "@localhost>"))
}

// TestCustomMessageID checks the Message-ID lookup in the custom headers
func TestCustomMessageID(t *testing.T) {
	t.Parallel()

	assert.Empty(t, customMessageID(&Email{}))
	assert.Equal(t, "<a@b>", customMessageID(&Email{Headers: map[string]string{"message-id": "<a@b>"}}))
}

// TestAttachmentContentType checks the attachment content type formatting
func TestAttachmentContentType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		att      attachmentData
		expected string
	}{
		{"type with name", attachmentData{contentType: "text/plain", name: testFileName}, `text/plain; name=a.txt`},
		{"keeps parameters", attachmentData{contentType: "text/plain; charset=utf-8", name: testFileName}, `text/plain; charset=utf-8; name=a.txt`},
		{"invalid type falls back", attachmentData{contentType: "not a type", name: testFileName}, `application/octet-stream; name=a.txt`},
		{"no name", attachmentData{contentType: "image/png"}, `image/png`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, attachmentContentType(test.att))
		})
	}
}

// TestFormatMediaTypeOr checks the fallback for invalid media types
func TestFormatMediaTypeOr(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "fallback", formatMediaTypeOr("bad type", nil, "fallback"))
	assert.Equal(t, "inline", formatMediaTypeOr("inline", nil, "fallback"))
}

// failingWriter is an io.Writer that always fails
type failingWriter struct{}

// Write always returns an error
func (failingWriter) Write(_ []byte) (int, error) {
	return 0, io.ErrClosedPipe
}

// TestWriteHelpersPropagateErrors checks that part creation and write errors are returned
func TestWriteHelpersPropagateErrors(t *testing.T) {
	t.Parallel()

	creators := map[string]partCreator{
		"create fails": func(_ textproto.MIMEHeader) (io.Writer, error) { return nil, io.ErrClosedPipe },
		"write fails":  func(_ textproto.MIMEHeader) (io.Writer, error) { return failingWriter{}, nil },
	}

	email := newValidEmail()
	email.HTMLContent = "<p>hi</p>"
	att := attachmentData{content: []byte("x"), contentType: "text/plain", name: testFileName}
	inline := attachmentData{content: []byte("x"), contentID: "cid", contentType: "image/png", name: "a.png"}

	for name, create := range creators {
		t.Run(name, func(t *testing.T) {
			require.Error(t, writeMixedPart(create, email, []attachmentData{att}))
			require.Error(t, writeRelatedPart(create, email, []attachmentData{inline}))
			require.Error(t, writeBodyPart(create, email))
			require.Error(t, writeTextPart(create, mimeTypePlain, "x"))
			require.Error(t, writeAttachmentPart(create, att))
		})
	}
}

// limitWriter fails once more than remaining bytes are written
type limitWriter struct {
	written   int
	remaining int
}

// Write writes up to the remaining budget, then fails
func (w *limitWriter) Write(p []byte) (int, error) {
	if len(p) > w.remaining {
		n := w.remaining
		w.remaining = 0
		w.written += n
		return n, io.ErrShortWrite
	}
	w.remaining -= len(p)
	w.written += len(p)
	return len(p), nil
}

// TestWriteMixedPartFailsAtEveryByte fails the output at every possible byte and
// checks the error is always returned
func TestWriteMixedPartFailsAtEveryByte(t *testing.T) {
	t.Parallel()

	email := newValidEmail()
	email.HTMLContent = "<p>hi</p>"
	attachments := []attachmentData{
		{content: bytes.Repeat([]byte("regular "), 30), contentType: "text/plain", name: testFileName},
		{content: []byte("inline"), contentID: "cid", contentType: "image/png", name: "a.png"},
	}

	// Measure the full size first
	full := &limitWriter{remaining: 1 << 20}
	require.NoError(t, writeMixedPart(func(_ textproto.MIMEHeader) (io.Writer, error) { return full, nil }, email, attachments))

	for limit := range full.written {
		w := &limitWriter{remaining: limit}
		err := writeMixedPart(func(_ textproto.MIMEHeader) (io.Writer, error) { return w, nil }, email, attachments)
		require.ErrorIs(t, err, io.ErrShortWrite, "failing after %d bytes must return the error", limit)
	}
}
