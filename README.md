# go-mail
**go-mail** is a lightweight email package with multi-provider support

| | | | | | | |
|-|-|-|-|-|-|-|
| ![License](https://img.shields.io/github/license/mrz1836/go-mail.svg?style=flat&p=1) | [![Report](https://goreportcard.com/badge/github.com/mrz1836/go-mail?style=flat&p=1)](https://goreportcard.com/report/github.com/mrz1836/go-mail)  | [![Codacy Badge](https://api.codacy.com/project/badge/Grade/0b377a0d1dde4b6ba189545aa7ee2e17)](https://www.codacy.com/app/mrz1818/go-mail?utm_source=github.com&amp;utm_medium=referral&amp;utm_content=mrz1836/go-mail&amp;utm_campaign=Badge_Grade) |  [![Build Status](https://travis-ci.com/mrz1836/go-mail.svg?branch=master)](https://travis-ci.com/mrz1836/go-mail)   |  [![standard-readme compliant](https://img.shields.io/badge/standard--readme-OK-green.svg?style=flat)](https://github.com/RichardLitt/standard-readme) | [![Release](https://img.shields.io/github/release-pre/mrz1836/go-mail.svg?style=flat)](https://github.com/mrz1836/go-mail/releases) | [![GoDoc](https://godoc.org/github.com/mrz1836/go-mail?status.svg&style=flat)](https://godoc.org/github.com/mrz1836/go-mail) |

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

**go-mail** requires a [supported release of Go](https://golang.org/doc/devel/release.html#policy) and [dep](https://github.com/golang/dep).
```bash
$ go get -u github.com/mrz1836/go-mail
```

Updating dependencies in **go-mail**:
```bash
$ cd ../go-mail
$ dep ensure -update -v
```

### Package Dependencies
- keighl's [postmark](https://github.com/keighl/postmark)
- mattbaird's [gochimp](https://github.com/mattbaird/gochimp)

## Documentation
You can view the generated [documentation here](https://godoc.org/github.com/mrz1836/go-mail).

### Supported Service Providers
- [Mandrill](https://mandrillapp.com/api/docs/)
- [Postmark](https://postmarkapp.com/developer)
- [AWS SES](https://docs.aws.amazon.com/ses/)

### Features
- Uses MrZ's [go-logger](https://github.com/mrz1836/go-logger) for either local or remote logging via [LogEntries](https://logentries.com/)
- Supports multiple service providers

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
)

func main() {

}
```

## Maintainers

[@MrZ1836](https://github.com/mrz1836)

## Contributing

This project uses keighl's [postmark](https://github.com/keighl/postmark) package.

This project uses mattbaird's [gochimp](https://github.com/mattbaird/gochimp) package.

View the [contributing guidelines](CONTRIBUTING.md) and follow the [code of conduct](CODE_OF_CONDUCT.md).

Support the development of this project 🙏

[![Donate](https://img.shields.io/badge/donate-bitcoin-brightgreen.svg)](https://mrz1818.com/?tab=tips&af=go-mail)

## License

![License](https://img.shields.io/github/license/mrz1836/go-mail.svg?style=flat&p=1)
