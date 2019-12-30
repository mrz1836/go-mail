# go-mail
**go-mail** is a lightweight email package with multi-provider support (ses, mandrill, postmark)

[![Go](https://img.shields.io/github/go-mod/go-version/mrz1836/go-mail)](https://golang.org/)
[![Build Status](https://travis-ci.com/mrz1836/go-mail.svg?branch=master)](https://travis-ci.com/mrz1836/go-mail)
[![Report](https://goreportcard.com/badge/github.com/mrz1836/go-mail?style=flat&v=1)](https://goreportcard.com/report/github.com/mrz1836/go-mail)
[![Codacy Badge](https://api.codacy.com/project/badge/Grade/52bfcc447ee24c29a2a3fb65c53a4de3)](https://www.codacy.com/app/mrz1818/go-mail?utm_source=github.com&amp;utm_medium=referral&amp;utm_content=mrz1836/go-mail&amp;utm_campaign=Badge_Grade)
[![Release](https://img.shields.io/github/release-pre/mrz1836/go-mail.svg?style=flat&v=1)](https://github.com/mrz1836/go-mail/releases)
[![standard-readme compliant](https://img.shields.io/badge/standard--readme-OK-green.svg?style=flat)](https://github.com/RichardLitt/standard-readme)
[![GoDoc](https://godoc.org/github.com/mrz1836/go-mail?status.svg&style=flat)](https://godoc.org/github.com/mrz1836/go-mail)

## Table of Contents
- [Installation](#installation)
- [Documentation](#documentation)
- [Examples & Tests](#examples--tests)
- [Benchmarks](#benchmarks)
- [Code Standards](#code-standards)
- [Usage](#usage)
- [Maintainers](#maintainers)
- [Contributing](#contributing)
- [License](#license)

## Installation

**go-mail** requires a [supported release of Go](https://golang.org/doc/devel/release.html#policy).
```bash
$ go get -u github.com/mrz1836/go-mail
```

### Package Dependencies
- domodwyer's [mailyak](https://github.com/domodwyer/mailyak)
- keighl's [postmark](https://github.com/mrz1836/postmark)
- mattbaird's [gochimp](https://github.com/mattbaird/gochimp)
- sourcegraph's [go-ses](https://github.com/sourcegraph/go-ses)
- aymerick's [douceur](https://github.com/aymerick/douceur)

## Documentation
You can view the generated [documentation here](https://godoc.org/github.com/mrz1836/go-mail).

### Features
- Supports multiple service providers _(below)_
- Support basic [SMTP](https://en.wikipedia.org/wiki/Simple_Mail_Transfer_Protocol)
- Plain-text and HTML content
- Multiple file attachments
- Open & click tracking _(provider dependant)_
- Inject css into html content
- Basic template support
- Max restrictions on To, CC and BCC

### Supported Service Providers
- [AWS SES](https://docs.aws.amazon.com/ses/)
- [Mandrill](https://mandrillapp.com/api/docs/)
- [Postmark](https://postmarkapp.com/developer)
- [SMTP](https://en.wikipedia.org/wiki/Simple_Mail_Transfer_Protocol)

## Examples & Tests
All unit tests and [examples](examples/examples.go) run via [Travis CI](https://travis-ci.com/mrz1836/go-mail) and uses [Go version 1.13.x](https://golang.org/doc/go1.13). View the [deployment configuration file](.travis.yml).

Run all tests (including integration tests)
```bash
$ cd ../go-mail
$ go test ./... -v
```

Run tests (excluding integration tests)
```bash
$ cd ../go-mail
$ go test ./... -v -test.short
```

View and run the examples:
```bash
$ cd ../go-mail/examples
$ go run examples.go
```

## Benchmarks
Run the Go benchmarks:
```bash
$ cd ../go-mail
$ go test -bench . -benchmem
```

## Code Standards
Read more about this Go project's [code standards](CODE_STANDARDS.md).

## Usage
View the [examples](examples/examples.go)

Basic implementation:
```golang
package main

import (
	"github.com/mrz1836/go-mail"
)

func main() {

	// Config
	mail := new(gomail.MailService)
	mail.FromName = "No Reply"
	mail.FromUsername = "no-reply"
	mail.FromDomain = "example.com"

	// Provider
	mail.MandrillAPIKey = "1234567"

	// Start the service
	_ = mail.StartUp()

	// Create and send a basic email
	email := mail.NewEmail()
	email.HTMLContent = "<html><body>This is a <b>go-mail</b> test email using <i>HTML</i></body></html>"
	email.Recipients = []string{"jack@example.com"}
	email.Subject = "testing go-mail package - test message"

	_ = mail.SendEmail(email, gomail.Mandrill)
}
```

## Maintainers

[@MrZ](https://github.com/mrz1836)

## Contributing

This project uses:
- domodwyer's [mailyak](https://github.com/domodwyer/mailyak) package
- keighl's [postmark](https://github.com/mrz1836/postmark) package
- mattbaird's [gochimp](https://github.com/mattbaird/gochimp) package
- sourcegraph's [go-ses](https://github.com/sourcegraph/go-ses) package
- aymerick's [douceur](https://github.com/aymerick/douceur) package

View the [contributing guidelines](CONTRIBUTING.md) and follow the [code of conduct](CODE_OF_CONDUCT.md).

Support the development of this project 🙏

[![Donate](https://img.shields.io/badge/donate-bitcoin-brightgreen.svg)](https://mrz1818.com/?tab=tips&af=go-mail)

## License

![License](https://img.shields.io/github/license/mrz1836/go-mail.svg?style=flat&v=1)
