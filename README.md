# go-mail
**go-mail** is a lightweight email package with multi-provider support (ses, mandrill, postmark)

[![Go](https://img.shields.io/github/go-mod/go-version/mrz1836/go-mail)](https://golang.org/)
[![Build Status](https://travis-ci.com/mrz1836/go-mail.svg?branch=master)](https://travis-ci.com/mrz1836/go-mail)
[![Report](https://goreportcard.com/badge/github.com/mrz1836/go-mail?style=flat&v=1)](https://goreportcard.com/report/github.com/mrz1836/go-mail)
[![Release](https://img.shields.io/github/release-pre/mrz1836/go-mail.svg?style=flat&v=1)](https://github.com/mrz1836/go-mail/releases)
[![GoDoc](https://godoc.org/github.com/mrz1836/go-mail?status.svg&style=flat)](https://pkg.go.dev/github.com/mrz1836/go-mail)

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

## Documentation
You can view the generated [documentation here](https://pkg.go.dev/github.com/mrz1836/go-mail).

### Features
- Supports multiple service providers _(below)_
- Support basic [SMTP](https://en.wikipedia.org/wiki/Simple_Mail_Transfer_Protocol)
- Plain-text and HTML content
- Multiple file attachments
- Open & click tracking _(provider dependant)_
- Inject css into html content
- Basic template support
- Max restrictions on To, CC and BCC

<details>
<summary><strong><code>Supported Service Providers</code></strong></summary>

- [AWS SES](https://docs.aws.amazon.com/ses/)
- [Mandrill](https://mandrillapp.com/api/docs/)
- [Postmark](https://postmarkapp.com/developer)
- [SMTP](https://en.wikipedia.org/wiki/Simple_Mail_Transfer_Protocol)
</details>

<details>
<summary><strong><code>Package Dependencies</code></strong></summary>

- domodwyer's [mailyak](https://github.com/domodwyer/mailyak)
- keighl's [postmark](https://github.com/mrz1836/postmark)
- mattbaird's [gochimp](https://github.com/mattbaird/gochimp)
- sourcegraph's [go-ses](https://github.com/sourcegraph/go-ses)
- aymerick's [douceur](https://github.com/aymerick/douceur)
</details>

<details>
<summary><strong><code>Library Deployment</code></strong></summary>

[goreleaser](https://github.com/goreleaser/goreleaser) for easy binary or library deployment to Github and can be installed via: `brew install goreleaser`.

The [.goreleaser.yml](.goreleaser.yml) file is used to configure [goreleaser](https://github.com/goreleaser/goreleaser).

Use `make release-snap` to create a snapshot version of the release, and finally `make release` to ship to production.
</details>

<details>
<summary><strong><code>Makefile Commands</code></strong></summary>

View all `makefile` commands
```bash
$ make help
```

List of all current commands:
```text
all                            Runs test, install, clean, docs
bench                          Run all benchmarks in the Go application
clean                          Remove previous builds and any test cache data
clean-mods                     Remove all the Go mod cache
coverage                       Shows the test coverage
godocs                         Sync the latest tag with GoDocs
help                           Show all make commands available
lint                           Run the Go lint application
release                        Full production release (creates release in Github)
release-test                   Full production test release (everything except deploy)
release-snap                   Test the full release (build binaries)
tag                            Generate a new tag and push (IE: make tag version=0.0.0)
tag-remove                     Remove a tag if found (IE: make tag-remove version=0.0.0)
tag-update                     Update an existing tag to current commit (IE: make tag-update version=0.0.0)
test                           Runs vet, lint and ALL tests
test-short                     Runs vet, lint and tests (excludes integration tests)
update                         Update all project dependencies
update-releaser                Update the goreleaser application
vet                            Run the Go vet application
```
</details>

## Examples & Tests
All unit tests and [examples](examples/examples.go) run via [Travis CI](https://travis-ci.com/mrz1836/go-mail) and uses [Go version 1.14.x](https://golang.org/doc/go1.14). View the [deployment configuration file](.travis.yml).

Run all tests (including integration tests)
```bash
$ make test
```

Run tests (excluding integration tests)
```bash
$ make test-short
```

View and run the examples:
```bash
$ make run-examples
```

## Benchmarks
Run the Go benchmarks:
```bash
$ make bench
```

## Code Standards
Read more about this Go project's [code standards](CODE_STANDARDS.md).

## Usage
View the [examples](examples/examples.go)

## Maintainers

| [<img src="https://github.com/mrz1836.png" height="50" alt="MrZ" />](https://github.com/mrz1836) |
|:---:|
| [MrZ](https://github.com/mrz1836) |

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
