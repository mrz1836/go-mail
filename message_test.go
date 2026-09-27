package gomail

import (
	"bytes"
	"io"
	"net/mail"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseEnvelope checks address parsing and de-duplication
func TestParseEnvelope(t *testing.T) {
	t.Parallel()

	t.Run("parses names and removes duplicates", func(t *testing.T) {
		email := newValidEmail()
		email.FromName = "Sender"
		email.Recipients = []string{"To <TO@example.com>", "to@example.com", "other@example.com"}
		email.RecipientsCc = []string{"to@example.com", testCcAddress}
		email.RecipientsBcc = []string{testCcAddress, testBccAddress}
		email.ReplyToAddress = "Reply <reply@example.com>"

		env, err := parseEnvelope(email)
		require.NoError(t, err)
		assert.Equal(t, mail.Address{Name: "Sender", Address: "no-reply@example.com"}, env.from)
		assert.Equal(t, []string{"TO@example.com", "other@example.com"}, bareAddresses(env.to))
		assert.Equal(t, "To", env.to[0].Name)
		assert.Equal(t, []string{testCcAddress}, bareAddresses(env.cc))
		assert.Equal(t, []string{testBccAddress}, bareAddresses(env.bcc))
		assert.Equal(t, "reply@example.com", env.replyTo.Address)
		assert.Equal(t, []string{"TO@example.com", "other@example.com", testCcAddress, testBccAddress}, env.allRecipients())
	})

	t.Run("from name from the address", func(t *testing.T) {
		email := newValidEmail()
		email.FromAddress = "Embedded Name <sender@example.com>"
		env, err := parseEnvelope(email)
		require.NoError(t, err)
		assert.Equal(t, mail.Address{Name: "Embedded Name", Address: "sender@example.com"}, env.from)
	})

	t.Run("from name line breaks are removed", func(t *testing.T) {
		email := newValidEmail()
		email.FromName = "Evil\r\nBcc: x@example.com"
		env, err := parseEnvelope(email)
		require.NoError(t, err)
		assert.Equal(t, "Evil Bcc: x@example.com", env.from.Name)
	})

	tests := []struct {
		name     string
		setup    func(email *Email)
		expected error
	}{
		{"empty from", func(email *Email) { email.FromAddress = "" }, ErrInvalidFromAddress},
		{"from without domain", func(email *Email) { email.FromAddress = "user@" }, ErrInvalidFromAddress},
		{"from without at", func(email *Email) { email.FromAddress = "user" }, ErrInvalidFromAddress},
		{"empty recipient", func(email *Email) { email.Recipients = []string{""} }, ErrInvalidRecipient},
		{"two recipients in one string", func(email *Email) { email.Recipients = []string{"a@example.com, b@example.com"} }, ErrInvalidRecipient},
		{"header injection in recipient", func(email *Email) { email.Recipients = []string{"a@example.com\r\nBcc: b@example.com"} }, ErrInvalidRecipient},
		{"bad cc", func(email *Email) { email.RecipientsCc = []string{"nope"} }, ErrInvalidRecipient},
		{"bad bcc", func(email *Email) { email.RecipientsBcc = []string{"nope"} }, ErrInvalidRecipient},
		{"bad reply-to", func(email *Email) { email.ReplyToAddress = "nope" }, ErrInvalidReplyToAddress},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			email := newValidEmail()
			test.setup(email)
			_, err := parseEnvelope(email)
			require.ErrorIs(t, err, test.expected)
		})
	}
}

// TestFormatAddress checks RFC 5322 address formatting
func TestFormatAddress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		addr     mail.Address
		expected string
	}{
		{"bare address", mail.Address{Address: testAddressA}, testAddressA},
		{"simple name", mail.Address{Name: "Bob", Address: testAddressA}, `"Bob" <a@example.com>`},
		{"name with comma is quoted", mail.Address{Name: "Acme, Inc.", Address: testAddressA}, `"Acme, Inc." <a@example.com>`},
		{"name with quote is escaped", mail.Address{Name: `Bob "B"`, Address: testAddressA}, `"Bob \"B\"" <a@example.com>`},
		{"non-ascii name is encoded", mail.Address{Name: "Jörg", Address: testAddressA}, "=?utf-8?q?J=C3=B6rg?= <a@example.com>"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			formatted := formatAddress(&test.addr)
			assert.Equal(t, test.expected, formatted)

			parsed, err := mail.ParseAddress(formatted)
			require.NoError(t, err)
			assert.Equal(t, test.addr, *parsed)
		})
	}
}

// TestAddressDomain checks the domain extraction
func TestAddressDomain(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "example.com", addressDomain(testAddressA))
	assert.Equal(t, "example.com", addressDomain(`"a@b"@example.com`))
	assert.Empty(t, addressDomain("nodomain"))
}

// TestStripLineBreaks checks CR and LF removal
func TestStripLineBreaks(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "plain", stripLineBreaks("plain"))
	assert.Equal(t, "a b c", stripLineBreaks("a\r\nb\nc"))
	assert.Equal(t, "a", stripLineBreaks("\r\na\r\n"))
}

// TestValidateHeaders checks custom header validation
func TestValidateHeaders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		headers map[string]string
		valid   bool
	}{
		{"nil", nil, true},
		{"custom headers", map[string]string{"X-Campaign": "a", "List-Unsubscribe": "<https://x>", "Message-ID": "<a@b>"}, true},
		{"empty name", map[string]string{"": "a"}, false},
		{"name with space", map[string]string{"X Bad": "a"}, false},
		{"name with colon", map[string]string{"X:Bad": "a"}, false},
		{"name with non-ascii", map[string]string{"X-Ü": "a"}, false},
		{"reserved to", map[string]string{"to": testAddressA}, false},
		{"reserved bcc", map[string]string{"BCC": testAddressA}, false},
		{"reserved from", map[string]string{"From": testAddressA}, false},
		{"reserved content type", map[string]string{"Content-Type": "text/plain"}, false},
		{"value with line break", map[string]string{"X-Bad": "a\r\nBcc: b@example.com"}, false},
		{"value with nul", map[string]string{"X-Bad": "a\x00"}, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateHeaders(test.headers)
			if test.valid {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, ErrInvalidHeader)
		})
	}
}

// TestValidateMetadata checks metadata validation
func TestValidateMetadata(t *testing.T) {
	t.Parallel()

	require.NoError(t, validateMetadata(nil))
	require.NoError(t, validateMetadata(map[string]string{"k": "v"}))
	require.ErrorIs(t, validateMetadata(map[string]string{"": "v"}), ErrInvalidMetadata)
	require.ErrorIs(t, validateMetadata(map[string]string{"k": ""}), ErrInvalidMetadata)
}

// TestValidateAttachments checks attachment validation
func TestValidateAttachments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		att   Attachment
		valid bool
	}{
		{"content", Attachment{FileName: testFileName, Content: []byte("a")}, true},
		{"empty content", Attachment{FileName: testFileName, Content: []byte{}}, true},
		{"reader", Attachment{FileName: testFileName, FileReader: strings.NewReader("a")}, true},
		{"inline", Attachment{FileName: "a.png", Content: []byte("a"), ContentID: "logo@example"}, true},
		{"no content", Attachment{FileName: testFileName}, false},
		{"name with line break", Attachment{FileName: "a\r\n.txt", Content: []byte("a")}, false},
		{"type with line break", Attachment{FileName: testFileName, FileType: "text/plain\r\nX: 1", Content: []byte("a")}, false},
		{"content id with bracket", Attachment{FileName: "a.png", Content: []byte("a"), ContentID: "<logo>"}, false},
		{"content id with space", Attachment{FileName: "a.png", Content: []byte("a"), ContentID: "a b"}, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateAttachments([]Attachment{test.att})
			if test.valid {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, ErrInvalidAttachment)
		})
	}
}

// TestEmailHeaders checks the merged, sorted header list
func TestEmailHeaders(t *testing.T) {
	t.Parallel()

	t.Run("none", func(t *testing.T) {
		assert.Empty(t, emailHeaders(&Email{}))
		assert.Nil(t, headerMap(&Email{}))
	})

	t.Run("importance and custom headers are sorted", func(t *testing.T) {
		email := &Email{Important: true, Headers: map[string]string{"X-A": "1", "importance": "Low"}}
		assert.Equal(t, []header{
			{name: "importance", value: "Low"},
			{name: "X-A", value: "1"},
			{name: headerXMSMailPriority, value: headerHighValue},
			{name: headerXPriority, value: headerXPriorityValue},
		}, emailHeaders(email))
		assert.Equal(t, map[string]string{
			"importance":          "Low",
			"X-A":                 "1",
			headerXMSMailPriority: headerHighValue,
			headerXPriority:       headerXPriorityValue,
		}, headerMap(email))
	})

	t.Run("case-insensitive duplicates are deterministic", func(t *testing.T) {
		email := &Email{Headers: map[string]string{"X-Tag": "upper", "x-tag": "lower"}}
		for range 20 {
			assert.Equal(t, []header{{name: "x-tag", value: "lower"}}, emailHeaders(email))
		}
	})
}

// TestReadAttachments checks attachment loading and content type detection
func TestReadAttachments(t *testing.T) {
	t.Parallel()

	email := &Email{}
	email.AddAttachmentBytes("report.pdf", "", []byte("%PDF-1.4"))
	email.AddAttachment("notes", "", strings.NewReader("plain text"))
	email.AddInlineAttachment("logo.png", "image/png", "logo", []byte("png"))

	attachments, err := readAttachments(email)
	require.NoError(t, err)
	require.Len(t, attachments, 3)

	assert.Equal(t, "application/pdf", attachments[0].contentType)
	assert.False(t, attachments[0].inline())
	assert.Equal(t, "plain text", string(attachments[1].content))
	assert.Equal(t, "text/plain; charset=utf-8", attachments[1].contentType)
	assert.True(t, attachments[2].inline())
	assert.Equal(t, "logo", attachments[2].contentID)

	t.Run("reader error", func(t *testing.T) {
		bad := &Email{}
		bad.AddAttachment(testFileName, "text/plain", errReader{})
		_, err := readAttachments(bad)
		require.ErrorIs(t, err, io.ErrUnexpectedEOF)
	})

	t.Run("no content", func(t *testing.T) {
		_, err := readAttachments(&Email{Attachments: []Attachment{{FileName: testFileName}}})
		require.ErrorIs(t, err, ErrInvalidAttachment)
	})
}

// TestBufferAttachments checks reader attachments are buffered for reuse
func TestBufferAttachments(t *testing.T) {
	t.Parallel()

	t.Run("reader is buffered once", func(t *testing.T) {
		email := &Email{}
		email.AddAttachment(testFileName, "text/plain", strings.NewReader("hello"))

		require.NoError(t, bufferAttachments(email, 0))
		assert.Equal(t, []byte("hello"), email.Attachments[0].Content)

		// A second call keeps the buffered content (the reader is now drained)
		require.NoError(t, bufferAttachments(email, 0))
		assert.Equal(t, []byte("hello"), email.Attachments[0].Content)
	})

	t.Run("total size limit", func(t *testing.T) {
		email := &Email{}
		email.AddAttachmentBytes("a.bin", "", bytes.Repeat([]byte("a"), 6))
		email.AddAttachment("b.bin", "", bytes.NewReader(bytes.Repeat([]byte("b"), 6)))

		err := bufferAttachments(email, 10)
		require.ErrorIs(t, err, ErrAttachmentsTooLarge)
	})

	t.Run("exactly the limit", func(t *testing.T) {
		email := &Email{}
		email.AddAttachment("a.bin", "", bytes.NewReader(bytes.Repeat([]byte("a"), 10)))
		require.NoError(t, bufferAttachments(email, 10))
	})

	t.Run("content counts toward the limit", func(t *testing.T) {
		email := &Email{}
		email.AddAttachmentBytes("a.bin", "", bytes.Repeat([]byte("a"), 11))
		require.ErrorIs(t, bufferAttachments(email, 10), ErrAttachmentsTooLarge)
	})

	t.Run("reader error", func(t *testing.T) {
		email := &Email{}
		email.AddAttachment(testFileName, "text/plain", errReader{})
		require.ErrorIs(t, bufferAttachments(email, 0), io.ErrUnexpectedEOF)
	})

	t.Run("no content", func(t *testing.T) {
		email := &Email{Attachments: []Attachment{{FileName: testFileName}}}
		require.ErrorIs(t, bufferAttachments(email, 0), ErrInvalidAttachment)
	})
}

// TestSanitizeTagPart checks tag sanitization
func TestSanitizeTagPart(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "admin_alert", sanitizeTagPart("admin alert"))
	assert.Equal(t, "Signup-2_", sanitizeTagPart("Signup-2!"))
	assert.Equal(t, "caf_", sanitizeTagPart("café"))
	assert.Len(t, sanitizeTagPart(strings.Repeat("a", 300)), tagMaxLength)
	assert.Empty(t, sanitizeTagPart(""))
}

// TestNameValueTags checks the tag and metadata conversion
func TestNameValueTags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		tags     []string
		metadata map[string]string
		expected []tagPair
	}{
		{"nothing", nil, nil, nil},
		{"tags", []string{"a", "b c"}, nil, []tagPair{{"a", tagValueMarker}, {"b_c", tagValueMarker}}},
		{"empty and duplicate tags dropped", []string{"", "a", "a"}, nil, []tagPair{{"a", tagValueMarker}}},
		{"metadata sorted", nil, map[string]string{"z": "1", "a": "2 3"}, []tagPair{{"a", "2_3"}, {"z", "1"}}},
		{"metadata wins over a tag", []string{"k", "x"}, map[string]string{"k": "v"}, []tagPair{{"k", "v"}, {"x", tagValueMarker}}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, nameValueTags(test.tags, test.metadata))
		})
	}
}
