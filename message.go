package gomail

import (
	"fmt"
	"io"
	"maps"
	"mime"
	"net/http"
	"net/mail"
	"path/filepath"
	"slices"
	"strings"
)

const (
	tagMaxLength   = 256    // maximum length SES and Resend allow for a tag name or value
	tagValueMarker = "true" // value applied to every plain tag (go-mail tags have no value)
)

// envelope is the parsed, de-duplicated set of addresses for an email
//
// DO NOT CHANGE ORDER - Optimized for memory (maligned)
type envelope struct {
	bcc     []*mail.Address
	cc      []*mail.Address
	replyTo *mail.Address
	to      []*mail.Address
	from    mail.Address
}

// parseEnvelope parses every address on the email (RFC 5322, so "Name <addr>"
// is accepted) and removes duplicate recipients; an address listed more than
// once is kept only in the first list it appears in (to, then cc, then bcc)
func parseEnvelope(email *Email) (*envelope, error) {
	from, err := parseFromAddress(email)
	if err != nil {
		return nil, err
	}

	env := &envelope{from: from}
	seen := make(map[string]struct{}, len(email.Recipients)+len(email.RecipientsCc)+len(email.RecipientsBcc))
	if env.to, err = parseRecipientList(email.Recipients, seen); err != nil {
		return nil, err
	}
	if env.cc, err = parseRecipientList(email.RecipientsCc, seen); err != nil {
		return nil, err
	}
	if env.bcc, err = parseRecipientList(email.RecipientsBcc, seen); err != nil {
		return nil, err
	}

	if len(email.ReplyToAddress) > 0 {
		if env.replyTo, err = mail.ParseAddress(email.ReplyToAddress); err != nil {
			return nil, fmt.Errorf("%w: %q: %w", ErrInvalidReplyToAddress, email.ReplyToAddress, err)
		}
	}

	return env, nil
}

// parseFromAddress parses the sender; FromName takes precedence over a display
// name embedded in FromAddress
func parseFromAddress(email *Email) (mail.Address, error) {
	addr, err := mail.ParseAddress(email.FromAddress)
	if err != nil {
		return mail.Address{}, fmt.Errorf("%w: %q: %w", ErrInvalidFromAddress, email.FromAddress, err)
	}

	name := stripLineBreaks(email.FromName)
	if len(name) == 0 {
		name = addr.Name
	}
	return mail.Address{Name: name, Address: addr.Address}, nil
}

// parseRecipientList parses a list of recipients, skipping any address already in seen
func parseRecipientList(recipients []string, seen map[string]struct{}) ([]*mail.Address, error) {
	list := make([]*mail.Address, 0, len(recipients))
	for _, recipient := range recipients {
		addr, err := mail.ParseAddress(recipient)
		if err != nil {
			return nil, fmt.Errorf("%w: %q: %w", ErrInvalidRecipient, recipient, err)
		}

		key := strings.ToLower(addr.Address)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		list = append(list, addr)
	}
	return list, nil
}

// allRecipients returns the bare addresses of every to, cc and bcc recipient
func (e *envelope) allRecipients() []string {
	addrs := make([]string, 0, len(e.to)+len(e.cc)+len(e.bcc))
	addrs = append(addrs, bareAddresses(e.to)...)
	addrs = append(addrs, bareAddresses(e.cc)...)
	return append(addrs, bareAddresses(e.bcc)...)
}

// bareAddresses returns the addresses without display names
func bareAddresses(list []*mail.Address) []string {
	addrs := make([]string, 0, len(list))
	for _, addr := range list {
		addrs = append(addrs, addr.Address)
	}
	return addrs
}

// formatAddresses returns the RFC 5322 formatted addresses
func formatAddresses(list []*mail.Address) []string {
	addrs := make([]string, 0, len(list))
	for _, addr := range list {
		addrs = append(addrs, formatAddress(addr))
	}
	return addrs
}

// formatAddress returns the RFC 5322 form of an address: a bare address when
// there is no display name, otherwise a quoted (or RFC 2047 encoded) name
// followed by the address in angle brackets
func formatAddress(addr *mail.Address) string {
	if len(addr.Name) == 0 {
		return addr.Address
	}
	return addr.String()
}

// addressDomain returns the domain part of an address
func addressDomain(address string) string {
	if at := strings.LastIndexByte(address, '@'); at >= 0 {
		return address[at+1:]
	}
	return ""
}

// stripLineBreaks replaces CR and LF with spaces so a value can never break
// out of an email header
func stripLineBreaks(value string) string {
	if !strings.ContainsAny(value, "\r\n") {
		return value
	}
	return strings.Join(strings.FieldsFunc(value, func(r rune) bool { return r == '\r' || r == '\n' }), " ")
}

// header is a single email header field
type header struct {
	name  string
	value string
}

// isReservedHeader reports whether go-mail sets the header itself from the
// Email fields, so it cannot be overridden through Email.Headers
func isReservedHeader(name string) bool {
	switch strings.ToLower(name) {
	case "bcc", "cc", "content-transfer-encoding", "content-type", "date", "from",
		"mime-version", "reply-to", "return-path", "sender", "subject", "to":
		return true
	default:
		return false
	}
}

// isValidHeaderName reports whether name is a valid RFC 5322 field name
func isValidHeaderName(name string) bool {
	if len(name) == 0 {
		return false
	}
	for i := 0; i < len(name); i++ {
		if c := name[i]; c < '!' || c > '~' || c == ':' {
			return false
		}
	}
	return true
}

// validateHeaders checks the custom headers on the email
func validateHeaders(headers map[string]string) error {
	for _, name := range slices.Sorted(maps.Keys(headers)) {
		if !isValidHeaderName(name) {
			return fmt.Errorf("%w: invalid header name %q", ErrInvalidHeader, name)
		}
		if isReservedHeader(name) {
			return fmt.Errorf("%w: %q is set from the email fields and cannot be overridden", ErrInvalidHeader, name)
		}
		if strings.ContainsAny(headers[name], "\r\n\x00") {
			return fmt.Errorf("%w: value of %q contains a line break", ErrInvalidHeader, name)
		}
	}
	return nil
}

// validateMetadata checks the metadata on the email
func validateMetadata(metadata map[string]string) error {
	for key, value := range metadata {
		if len(key) == 0 || len(value) == 0 {
			return fmt.Errorf("%w: keys and values must not be empty (key %q)", ErrInvalidMetadata, key)
		}
	}
	return nil
}

// validateAttachments checks that every attachment has content and a safe name and content id
func validateAttachments(attachments []Attachment) error {
	for _, att := range attachments {
		if att.Content == nil && att.FileReader == nil {
			return fmt.Errorf("%w: %q has no content", ErrInvalidAttachment, att.FileName)
		}
		if strings.ContainsAny(att.FileName, "\r\n\x00") || strings.ContainsAny(att.FileType, "\r\n\x00") {
			return fmt.Errorf("%w: %q contains a line break", ErrInvalidAttachment, att.FileName)
		}
		if strings.ContainsAny(att.ContentID, "\r\n\x00<> ") {
			return fmt.Errorf("%w: content id %q contains invalid characters", ErrInvalidAttachment, att.ContentID)
		}
	}
	return nil
}

// emailHeaders returns the headers to add to the email: the importance headers
// (when Important is set) plus the custom headers, which win on a name clash.
// The result is sorted by name so the output is deterministic.
func emailHeaders(email *Email) []header {
	merged := make(map[string]header, len(email.Headers)+3)
	if email.Important {
		merged[strings.ToLower(headerXPriority)] = header{name: headerXPriority, value: headerXPriorityValue}
		merged[strings.ToLower(headerXMSMailPriority)] = header{name: headerXMSMailPriority, value: headerHighValue}
		merged[strings.ToLower(headerImportance)] = header{name: headerImportance, value: headerHighValue}
	}
	for _, name := range slices.Sorted(maps.Keys(email.Headers)) {
		merged[strings.ToLower(name)] = header{name: name, value: email.Headers[name]}
	}

	headers := make([]header, 0, len(merged))
	for _, key := range slices.Sorted(maps.Keys(merged)) {
		headers = append(headers, merged[key])
	}
	return headers
}

// headerMap returns emailHeaders as a map, or nil when there are none
func headerMap(email *Email) map[string]string {
	headers := emailHeaders(email)
	if len(headers) == 0 {
		return nil
	}
	m := make(map[string]string, len(headers))
	for _, h := range headers {
		m[h.name] = h.value
	}
	return m
}

// attachmentData is an attachment with its content fully loaded
//
// DO NOT CHANGE ORDER - Optimized for memory (maligned)
type attachmentData struct {
	content     []byte
	contentID   string
	contentType string
	name        string
}

// inline reports whether the attachment is embedded in the HTML body (cid:)
func (a *attachmentData) inline() bool {
	return len(a.contentID) > 0
}

// readAttachments loads the content of every attachment. Attachments that only
// have a FileReader are read here (MailService.Send buffers them into Content
// first, so an email can be retried or sent through another provider).
func readAttachments(email *Email) ([]attachmentData, error) {
	list := make([]attachmentData, 0, len(email.Attachments))
	for _, att := range email.Attachments {
		content := att.Content
		if content == nil {
			if att.FileReader == nil {
				return nil, fmt.Errorf("%w: %q has no content", ErrInvalidAttachment, att.FileName)
			}
			var err error
			if content, err = io.ReadAll(att.FileReader); err != nil {
				return nil, fmt.Errorf("failed to read attachment %q: %w", att.FileName, err)
			}
		}

		contentType := att.FileType
		if len(contentType) == 0 {
			contentType = detectContentType(att.FileName, content)
		}

		list = append(list, attachmentData{
			content:     content,
			contentID:   att.ContentID,
			contentType: contentType,
			name:        att.FileName,
		})
	}
	return list, nil
}

// detectContentType guesses a MIME type from the file extension, falling back
// to sniffing the content
func detectContentType(name string, content []byte) string {
	if byExt := mime.TypeByExtension(filepath.Ext(name)); len(byExt) > 0 {
		return byExt
	}
	return http.DetectContentType(content)
}

// bufferAttachments reads every reader-only attachment into Attachment.Content
// so the email can be sent more than once, enforcing a total size limit
// (limit <= 0 disables the limit)
func bufferAttachments(email *Email, limit int64) error {
	var total int64
	for i := range email.Attachments {
		att := &email.Attachments[i]
		content := att.Content
		if content == nil {
			if att.FileReader == nil {
				return fmt.Errorf("%w: %q has no content", ErrInvalidAttachment, att.FileName)
			}

			reader := att.FileReader
			if limit > 0 {
				reader = io.LimitReader(reader, limit-total+1)
			}

			var err error
			if content, err = io.ReadAll(reader); err != nil {
				return fmt.Errorf("failed to read attachment %q: %w", att.FileName, err)
			}
		}

		total += int64(len(content))
		if limit > 0 && total > limit {
			return fmt.Errorf("%w: limit is %d bytes", ErrAttachmentsTooLarge, limit)
		}
		att.Content = content
	}
	return nil
}

// tagPair is a name/value tag (SES message tags, Resend tags)
type tagPair struct {
	name  string
	value string
}

// sanitizeTagPart converts a value into a valid SES/Resend tag name or value:
// only ASCII letters, numbers, underscores and dashes (max 256 chars) are
// allowed, so any other character is replaced with an underscore
func sanitizeTagPart(value string) string {
	sanitized := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			return r
		}
		return '_'
	}, value)
	if len(sanitized) > tagMaxLength {
		sanitized = sanitized[:tagMaxLength]
	}
	return sanitized
}

// nameValueTags converts go-mail tags (value "true") and metadata (key/value)
// into sanitized name/value tags. Empty and duplicate names are dropped;
// metadata wins over a tag with the same name.
func nameValueTags(tags []string, metadata map[string]string) []tagPair {
	if len(tags) == 0 && len(metadata) == 0 {
		return nil
	}

	pairs := make([]tagPair, 0, len(tags)+len(metadata))
	index := make(map[string]int, len(tags)+len(metadata))
	add := func(name, value string, override bool) {
		if len(name) == 0 {
			return
		}
		if i, ok := index[name]; ok {
			if override {
				pairs[i].value = value
			}
			return
		}
		index[name] = len(pairs)
		pairs = append(pairs, tagPair{name: name, value: value})
	}

	for _, tag := range tags {
		add(sanitizeTagPart(tag), tagValueMarker, false)
	}
	for _, key := range slices.Sorted(maps.Keys(metadata)) {
		add(sanitizeTagPart(key), sanitizeTagPart(metadata[key]), true)
	}
	return pairs
}
