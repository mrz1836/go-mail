# CLAUDE.md - go-mail Project Guide

**Quick checklist for Claude Code when working with the go-mail package**

## 🎯 Project Overview
Multi-provider email library for Go with support for AWS SES, Mailgun, Mandrill, Postmark, Resend, SendGrid, and SMTP. Interface-based architecture with extensive testing and security scanning.

## 🏗️ Core Architecture

### Key Structs
- `MailService`: Main configuration struct managing providers and settings
- `Email`: Email data structure with attachments, templates, tracking options
- `Attachment`: File attachment with name, type, and content (bytes or a reader; `ContentID` makes it inline)
- `Provider`: Interface every service implements (`Send(ctx, *Email) (*SendResult, error)`)
- `SendResult`: Provider, message id, and native response returned by `MailService.Send`

### Provider Pattern
Every service is a `Provider` registered on the `MailService` (`StartUp` registers
the built-ins from their credentials; `RegisterProvider` adds custom ones):
```go
type Provider interface {
    Send(ctx context.Context, email *Email) (*SendResult, error)
}
```

Built-in providers wrap a small exported client interface (`SESClient`,
`MailgunClient`, `MandrillClient`, `PostmarkClient`, `SendGridClient`, `ResendClient`) that the
SDK client satisfies, so tests use fakes. Each also implements
`FeatureSupporter` and applies its typed `ProviderOption` (ie: `PostmarkOption`).

Shared pieces: `message.go` (address parsing, headers, attachments, tags),
`mime.go` (MIME builder used by SES and SMTP; never writes a Bcc header).

Provider files: `aws_ses.go`, `mailgun.go`, `mandrill.go`, `postmark.go`, `resend.go`, `sendgrid.go`, `smtp.go`

## 🔧 Essential Development Commands

```bash
# Primary workflow commands
magex test            # Run standard test suite
magex test:race       # Run with race detector
magex lint            # Check code quality
magex format:fix      # Format code
magex tidy            # Clean up modules

# Build and release
magex build           # Build for current platform
magex install         # Install to GOPATH/bin
magex release         # Create release

# Advanced testing
magex test:cover      # Coverage reports
magex bench           # Run benchmarks
magex test:fuzz       # Fuzz testing
```

## 🧪 Testing Patterns

### Mock Interfaces
Each provider has a mock interface in its test file:
```go
// Example pattern from existing tests
type mockProviderInterface struct {
    // Mock methods
}
```

### Test Structure
- Unit tests: `*_test.go` files
- Examples: `Example*` functions for documentation
- Benchmarks: `Benchmark*` functions
- Fuzz tests: `fuzz_test.go`

### Testing Conventions
- Use `testify/assert` and `testify/require`
- Parallel tests: `t.Parallel()`
- Constants for test data: `testDomainEmail`, `testUsernameEmail`, etc.

## 📝 Code Patterns

### Error Handling
Sentinel errors defined in `errors.go`:
```go
var (
    ErrMissingSubject = errors.New("email is missing a subject")
    // Use these instead of creating new errors
)
```

### Memory Optimization
Structs use memory-aligned field ordering (DO NOT CHANGE ORDER comments)

### Interface Usage
Always create interfaces for external dependencies to enable mocking:
```go
type serviceInterface interface {
    Method() error
}
```

## 🚀 Common Tasks

### Adding Email Provider
1. Create `<provider>.go` with a client interface, a `<Name>Provider` type (`Send`, `SupportsFeature`) and a `<Name>Option` type
2. Add to `ServiceProvider` enum (and `String()`) in `config.go`
3. Add a loader to `loadProviders()` in `config.go`
4. Implement `ServiceProvider()` for the option in `provider.go`
5. Create `<provider>_test.go` with a fake client
6. Add example in `examples/examples.go` and a row in the README feature table

### Modifying Email Struct
- Check memory alignment comments
- Update validation in `validateEmail()`
- Add to template processing if needed
- Update all provider implementations

### Provider-Specific Features
Each provider has different capabilities - check existing warnings for unsupported features:
```go
if email.TrackClicks {
    log.Printf("warning: track clicks not supported by this provider")
}
```

## ⚡ Quick Reference

### Project Structure
```
├── email.go           # Core Email struct and methods
├── config.go          # MailService and provider setup
├── aws_ses.go         # AWS SES implementation
├── mailgun.go         # Mailgun implementation
├── mandrill.go        # Mandrill implementation
├── postmark.go        # Postmark implementation
├── resend.go          # Resend implementation
├── sendgrid.go        # SendGrid implementation
├── smtp.go           # SMTP implementation
├── errors.go         # Sentinel error definitions
└── examples/         # Usage examples
```

### Dependencies
- AWS SDK v2 for SES
- `mailgun-go/v5` for Mailgun
- `gochimp` for Mandrill
- `postmark` client library
- `resend-go/v4` for Resend
- `sendgrid-go` for SendGrid
- Standard library (`net/smtp`, `mime/multipart`) for SMTP and raw MIME messages
- `douceur/inliner` for CSS inlining

## ⚠️ Critical Guidelines

### Before Every Commit
1. `magex test` - All tests must pass
2. `magex lint` - Code must pass linting
3. `magex format:fix` - Code must be formatted

### Code Standards
- Follow existing patterns exactly
- Use interfaces for all external dependencies
- Add comprehensive tests with mocks
- Document public methods and structs
- Handle errors properly with sentinel errors
- Check provider capabilities and warn when unsupported

### Security
- Never log or expose API keys/credentials
- Use environment variables for sensitive config
- Follow security best practices in AGENTS.md

## 📚 Key Files to Reference
- `AGENTS.md` - Comprehensive development guidelines
- `examples/examples.go` - Usage patterns for all providers
- `*_test.go` files - Testing patterns and mock implementations
- `errors.go` - Available error types

## 🔍 Debugging Tips
- Check `AvailableProviders` slice to see which providers loaded
- Provider-specific errors are wrapped with context
- Use `-v` flag with tests for verbose output
- Check examples directory for working configurations
