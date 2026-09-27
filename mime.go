package gomail

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"maps"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"net/textproto"
	"slices"
	"strings"
	"time"
)

const (
	base64LineLength    = 76                         // RFC 2045 maximum encoded line length
	headerFoldLength    = 78                         // RFC 5322 recommended maximum header line length
	defaultAttachType   = "application/octet-stream" // fallback when a content type is unusable
	headerMessageID     = "Message-ID"               // header holding the unique message id
	mimeCharsetUTF8     = "utf-8"                    // charset for every text part and encoded header
	mimeTransferQP      = "quoted-printable"         // transfer encoding for text parts
	mimeTransferBase64  = "base64"                   // transfer encoding for attachments
	mimeTypeHTML        = "text/html"                // HTML body part
	mimeTypePlain       = "text/plain"               // plain-text body part
	mimeTypeAlternative = "multipart/alternative"    // plain-text and HTML versions of the body
	mimeTypeMixed       = "multipart/mixed"          // body plus regular attachments
	mimeTypeRelated     = "multipart/related"        // HTML body plus inline (cid:) attachments
)

// partCreator creates a MIME part with the given header and returns its body writer
type partCreator func(header textproto.MIMEHeader) (io.Writer, error)

// buildMIME renders an email as an RFC 5322 message with a MIME body.
//
// The Bcc header is never written, so blind-copy recipients are not disclosed
// to the other recipients; callers pass every recipient in the SMTP envelope
// (or the SES Destinations) instead. messageID is written when non-empty.
func buildMIME(email *Email, env *envelope, attachments []attachmentData, messageID string, date time.Time) ([]byte, error) {
	var buf bytes.Buffer

	writeHeader(&buf, "From", formatAddress(&env.from))
	if env.replyTo != nil {
		writeHeader(&buf, "Reply-To", formatAddress(env.replyTo))
	}
	writeHeader(&buf, "To", strings.Join(formatAddresses(env.to), ", "))
	if len(env.cc) > 0 {
		writeHeader(&buf, "Cc", strings.Join(formatAddresses(env.cc), ", "))
	}
	writeHeader(&buf, "Subject", encodeHeaderText(email.Subject))
	writeHeader(&buf, "Date", date.Format(time.RFC1123Z))
	if len(messageID) > 0 {
		writeHeader(&buf, headerMessageID, messageID)
	}
	writeHeader(&buf, "MIME-Version", "1.0")
	for _, h := range emailHeaders(email) {
		writeHeader(&buf, h.name, encodeHeaderText(h.value))
	}

	// The top-level entity's content headers are written straight after the message headers
	topLevel := func(header textproto.MIMEHeader) (io.Writer, error) {
		for _, name := range slices.Sorted(maps.Keys(header)) {
			for _, value := range header[name] {
				writeHeader(&buf, name, value)
			}
		}
		buf.WriteString("\r\n")
		return &buf, nil
	}

	if err := writeMixedPart(topLevel, email, attachments); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// writeMixedPart writes the body plus regular attachments (multipart/mixed),
// or just the body when there are no regular attachments
func writeMixedPart(create partCreator, email *Email, attachments []attachmentData) error {
	var inline, regular []attachmentData
	for _, att := range attachments {
		if att.inline() {
			inline = append(inline, att)
		} else {
			regular = append(regular, att)
		}
	}

	if len(regular) == 0 {
		return writeRelatedPart(create, email, inline)
	}

	mw, err := newNestedMultipart(create, mimeTypeMixed, nil)
	if err != nil {
		return err
	}
	if err = writeRelatedPart(mw.CreatePart, email, inline); err != nil {
		return err
	}
	for _, att := range regular {
		if err = writeAttachmentPart(mw.CreatePart, att); err != nil {
			return err
		}
	}
	return mw.Close()
}

// writeRelatedPart writes the body plus inline attachments (multipart/related),
// or just the body when there are no inline attachments
func writeRelatedPart(create partCreator, email *Email, inline []attachmentData) error {
	if len(inline) == 0 {
		return writeBodyPart(create, email)
	}

	// RFC 2387 requires the type of the root part
	rootType := mimeTypeHTML
	if len(email.PlainTextContent) > 0 && len(email.HTMLContent) > 0 {
		rootType = mimeTypeAlternative
	}

	mw, err := newNestedMultipart(create, mimeTypeRelated, map[string]string{"type": rootType})
	if err != nil {
		return err
	}
	if err = writeBodyPart(mw.CreatePart, email); err != nil {
		return err
	}
	for _, att := range inline {
		if err = writeAttachmentPart(mw.CreatePart, att); err != nil {
			return err
		}
	}
	return mw.Close()
}

// writeBodyPart writes the text and/or HTML body (multipart/alternative when both are set)
func writeBodyPart(create partCreator, email *Email) error {
	hasText, hasHTML := len(email.PlainTextContent) > 0, len(email.HTMLContent) > 0
	switch {
	case hasText && hasHTML:
		mw, err := newNestedMultipart(create, mimeTypeAlternative, nil)
		if err != nil {
			return err
		}
		if err = writeTextPart(mw.CreatePart, mimeTypePlain, email.PlainTextContent); err != nil {
			return err
		}
		if err = writeTextPart(mw.CreatePart, mimeTypeHTML, email.HTMLContent); err != nil {
			return err
		}
		return mw.Close()
	case hasHTML:
		return writeTextPart(create, mimeTypeHTML, email.HTMLContent)
	default:
		return writeTextPart(create, mimeTypePlain, email.PlainTextContent)
	}
}

// newNestedMultipart creates a multipart part and returns a writer for its sub-parts
func newNestedMultipart(create partCreator, mediaType string, params map[string]string) (*multipart.Writer, error) {
	boundary := multipart.NewWriter(io.Discard).Boundary()

	allParams := map[string]string{"boundary": boundary}
	maps.Copy(allParams, params)

	w, err := create(textproto.MIMEHeader{"Content-Type": {mime.FormatMediaType(mediaType, allParams)}})
	if err != nil {
		return nil, err
	}

	mw := multipart.NewWriter(w)
	if err = mw.SetBoundary(boundary); err != nil {
		return nil, err
	}
	return mw, nil
}

// writeTextPart writes a quoted-printable UTF-8 text part
func writeTextPart(create partCreator, mediaType, content string) error {
	w, err := create(textproto.MIMEHeader{
		"Content-Type":              {mime.FormatMediaType(mediaType, map[string]string{"charset": mimeCharsetUTF8})},
		"Content-Transfer-Encoding": {mimeTransferQP},
	})
	if err != nil {
		return err
	}

	qp := quotedprintable.NewWriter(w)
	if _, err = qp.Write([]byte(content)); err != nil {
		return err
	}
	return qp.Close()
}

// writeAttachmentPart writes a base64 encoded attachment part
func writeAttachmentPart(create partCreator, att attachmentData) error {
	disposition := "attachment"
	header := textproto.MIMEHeader{
		"Content-Transfer-Encoding": {mimeTransferBase64},
	}
	if att.inline() {
		disposition = "inline"
		header.Set("Content-ID", "<"+att.contentID+">")
	}

	var dispositionParams map[string]string
	if len(att.name) > 0 {
		dispositionParams = map[string]string{"filename": att.name}
	}
	header.Set("Content-Type", attachmentContentType(att))
	header.Set("Content-Disposition", formatMediaTypeOr(disposition, dispositionParams, disposition))

	w, err := create(header)
	if err != nil {
		return err
	}

	encoded := base64.StdEncoding.EncodeToString(att.content)
	for len(encoded) > base64LineLength {
		if _, err = io.WriteString(w, encoded[:base64LineLength]+"\r\n"); err != nil {
			return err
		}
		encoded = encoded[base64LineLength:]
	}
	_, err = io.WriteString(w, encoded+"\r\n")
	return err
}

// attachmentContentType returns the attachment Content-Type with its name parameter
func attachmentContentType(att attachmentData) string {
	mediaType, params, err := mime.ParseMediaType(att.contentType)
	if err != nil {
		mediaType, params = defaultAttachType, map[string]string{}
	}
	if len(att.name) > 0 {
		params["name"] = att.name
	}
	return formatMediaTypeOr(mediaType, params, defaultAttachType)
}

// formatMediaTypeOr formats a media type with parameters (RFC 2231 encoding
// non-ASCII values), returning fallback when the result would be invalid
func formatMediaTypeOr(mediaType string, params map[string]string, fallback string) string {
	if formatted := mime.FormatMediaType(mediaType, params); len(formatted) > 0 {
		return formatted
	}
	return fallback
}

// encodeHeaderText strips line breaks and RFC 2047 encodes non-ASCII text
func encodeHeaderText(value string) string {
	return mime.QEncoding.Encode(mimeCharsetUTF8, stripLineBreaks(value))
}

// writeHeader writes a folded header line
func writeHeader(buf *bytes.Buffer, name, value string) {
	buf.WriteString(foldHeader(name + ": " + stripLineBreaks(value)))
	buf.WriteString("\r\n")
}

// foldHeader folds a header line at whitespace so lines stay within the
// recommended length where possible (RFC 5322 section 2.2.3)
func foldHeader(line string) string {
	if len(line) <= headerFoldLength {
		return line
	}

	var b strings.Builder
	for len(line) > headerFoldLength {
		cut := foldPoint(line)
		if cut < 0 {
			break
		}
		b.WriteString(line[:cut])
		b.WriteString("\r\n")
		line = line[cut:]
	}
	b.WriteString(line)
	return b.String()
}

// foldPoint returns the index of the whitespace to fold at: the last usable one
// within the line limit, otherwise the first one after it, or -1 when there is
// none. A fold never leaves a line consisting only of whitespace.
func foldPoint(line string) int {
	best := -1
	for i := 1; i < len(line); i++ {
		if line[i] != ' ' && line[i] != '\t' {
			continue
		}
		if len(strings.TrimSpace(line[:i])) == 0 {
			continue
		}
		if i <= headerFoldLength {
			best = i
			continue
		}
		if best < 0 {
			best = i
		}
		break
	}
	return best
}

// newMessageID generates a unique RFC 5322 Message-ID for the sender's domain
func newMessageID(from *mail.Address) (string, error) {
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return "", fmt.Errorf("failed to generate message id: %w", err)
	}

	domain := addressDomain(from.Address)
	if len(domain) == 0 {
		domain = "localhost"
	}
	return "<" + hex.EncodeToString(id) + "@" + domain + ">", nil
}

// customMessageID returns the Message-ID set through Email.Headers, if any
func customMessageID(email *Email) string {
	for _, h := range emailHeaders(email) {
		if strings.EqualFold(h.name, headerMessageID) {
			return h.value
		}
	}
	return ""
}
