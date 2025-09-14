# CLAUDE.md - go-mail Project Guide

**Quick checklist for Claude Code when working with the go-mail package**

## 🎯 Project Overview
Multi-provider email library for Go with support for AWS SES, Mandrill, Postmark, and SMTP. Interface-based architecture with extensive testing and security scanning.

## 🏗️ Core Architecture

### Key Structs
- `MailService`: Main configuration struct managing providers and settings
- `Email`: Email data structure with attachments, templates, tracking options
- `Attachment`: File attachment with name, type, and reader

### Provider Pattern
All providers implement interfaces for testability:
```go
// Pattern used throughout
type providerInterface interface {
    SendEmail(...) error
}
```

Provider files: `aws_ses.go`, `mandrill.go`, `postmark.go`, `smtp.go`

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
1. Create `provider.go` with interface and implementation
2. Add to `ServiceProvider` enum in `config.go`
3. Update `StartUp()` method to initialize provider
4. Add case in `SendEmail()` switch statement
5. Create `provider_test.go` with mock interface
6. Add example in `examples/examples.go`

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
├── mandrill.go        # Mandrill implementation
├── postmark.go        # Postmark implementation
├── smtp.go           # SMTP implementation
├── errors.go         # Sentinel error definitions
└── examples/         # Usage examples
```

### Dependencies
- AWS SDK v2 for SES
- `gochimp` for Mandrill
- `postmark` client library
- `mailyak` for SMTP/raw email
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
