<div align="center">

# 📨&nbsp;&nbsp;go-mail

**Lightweight email package with multi-provider support ([ses](https://aws.amazon.com/ses/), [mailgun](https://www.mailgun.com/), [mandrill](https://mailchimp.com/features/transactional-email/), [postmark](https://postmarkapp.com/), [resend](https://resend.com/), [sendgrid](https://sendgrid.com/), [smtp](https://en.wikipedia.org/wiki/Simple_Mail_Transfer_Protocol))**

<br/>

<a href="https://github.com/mrz1836/go-mail/releases"><img src="https://img.shields.io/github/release-pre/mrz1836/go-mail?include_prereleases&style=flat-square&logo=github&color=black" alt="Release"></a>
<a href="https://golang.org/"><img src="https://img.shields.io/github/go-mod/go-version/mrz1836/go-mail?style=flat-square&logo=go&color=00ADD8" alt="Go Version"></a>
<a href="https://github.com/mrz1836/go-mail/blob/master/LICENSE"><img src="https://img.shields.io/github/license/mrz1836/go-mail?style=flat-square&color=blue" alt="License"></a>

<br/>

<table align="center" border="0">
  <tr>
    <td align="right">
       <code>CI / CD</code> &nbsp;&nbsp;
    </td>
    <td align="left">
       <a href="https://github.com/mrz1836/go-mail/actions"><img src="https://img.shields.io/github/actions/workflow/status/mrz1836/go-mail/fortress.yml?branch=master&label=build&logo=github&style=flat-square" alt="Build"></a>
       <a href="https://github.com/mrz1836/go-mail/actions"><img src="https://img.shields.io/github/last-commit/mrz1836/go-mail?style=flat-square&logo=git&logoColor=white&label=last%20update" alt="Last Commit"></a>
    </td>
    <td align="right">
       &nbsp;&nbsp;&nbsp;&nbsp; <code>Quality</code> &nbsp;&nbsp;
    </td>
    <td align="left">
       <a href="https://codecov.io/gh/mrz1836/go-mail"><img src="https://codecov.io/gh/mrz1836/go-mail/branch/master/graph/badge.svg?style=flat-square" alt="Coverage"></a>
    </td>
  </tr>

  <tr>
    <td align="right">
       <code>Security</code> &nbsp;&nbsp;
    </td>
    <td align="left">
       <a href="https://scorecard.dev/viewer/?uri=github.com/mrz1836/go-mail"><img src="https://api.scorecard.dev/projects/github.com/mrz1836/go-mail/badge?style=flat-square" alt="Scorecard"></a>
       <a href=".github/SECURITY.md"><img src="https://img.shields.io/badge/policy-active-success?style=flat-square&logo=security&logoColor=white" alt="Security"></a>
    </td>
    <td align="right">
       &nbsp;&nbsp;&nbsp;&nbsp; <code>Community</code> &nbsp;&nbsp;
    </td>
    <td align="left">
       <a href="https://github.com/mrz1836/go-mail/graphs/contributors"><img src="https://img.shields.io/github/contributors/mrz1836/go-mail?style=flat-square&color=orange" alt="Contributors"></a>
       <a href="https://mrz1818.com/"><img src="https://img.shields.io/badge/donate-bitcoin-ff9900?style=flat-square&logo=bitcoin" alt="Bitcoin"></a>
    </td>
  </tr>
</table>

</div>

<br/>
<br/>

<div align="center">

### <code>Project Navigation</code>

</div>

<table align="center">
  <tr>
    <td align="center" width="33%">
       🚀&nbsp;<a href="#installation"><code>Installation</code></a>
    </td>
    <td align="center" width="33%">
       🧪&nbsp;<a href="#examples--tests"><code>Examples&nbsp;&&nbsp;Tests</code></a>
    </td>
    <td align="center" width="33%">
       📚&nbsp;<a href="#documentation"><code>Documentation</code></a>
    </td>
  </tr>
  <tr>
    <td align="center">
       🤝&nbsp;<a href="#contributing"><code>Contributing</code></a>
    </td>
    <td align="center">
      🛠️&nbsp;<a href="#code-standards"><code>Code&nbsp;Standards</code></a>
    </td>
    <td align="center">
      ⚡&nbsp;<a href="#benchmarks"><code>Benchmarks</code></a>
    </td>
  </tr>
  <tr>
    <td align="center">
      🤖&nbsp;<a href="#-ai-usage--assistant-guidelines"><code>AI&nbsp;Usage</code></a>
    </td>
    <td align="center">
       ⚖️&nbsp;<a href="#license"><code>License</code></a>
    </td>
    <td align="center">
       👥&nbsp;<a href="#maintainers"><code>Maintainers</code></a>
    </td>
  </tr>
</table>
<br/>

## Installation

**go-mail** requires a [supported release of Go](https://golang.org/doc/devel/release.html#policy).
```shell script
go get github.com/mrz1836/go-mail
```

<br/>

## Documentation
View the generated [documentation](https://pkg.go.dev/github.com/mrz1836/go-mail)

### Quick Start
```go
package main

import (
    "context"
    "log"

    gomail "github.com/mrz1836/go-mail"
)

func main() {
    // Configure the sender and at least one provider
    mail := &gomail.MailService{
        FromName:       "No Reply",
        FromUsername:   "no-reply",
        FromDomain:     "example.com",
        SendGridAPIKey: "SG.xxxx",
    }
    if err := mail.StartUp(); err != nil {
        log.Fatal(err)
    }

    // Create and send an email
    email := mail.NewEmail()
    email.Subject = "Welcome!"
    email.HTMLContent = "<p>Thanks for signing up.</p>"
    email.PlainTextContent = "Thanks for signing up."
    email.Recipients = []string{"Jane Doe <jane@example.com>"}

    result, err := mail.Send(context.Background(), email, gomail.SendGrid)
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("sent via %s: %s", result.Provider, result.MessageID)
}
```

More complete examples (every provider, attachments, templates, failover and
provider options) are in [examples/examples.go](examples/examples.go).

### Features
- Supports multiple service providers _(below)_, plus your own through the `Provider` interface
- Failover across providers, with the provider's message id returned from `Send`
- Provider-specific features through typed options _(ie: Postmark message streams, SendGrid templates, Mailgun test mode)_
- AWS SES via static keys or the default credential chain _(IAM role)_
- [SMTP](https://en.wikipedia.org/wiki/Simple_Mail_Transfer_Protocol) with STARTTLS or implicit TLS, optional authentication, and timeouts
- Plain-text and HTML content
- Recipients with display names (`Jane Doe <jane@example.com>`); duplicates across `To`, `CC` and `BCC` are removed
- Multiple file attachments, plus inline (`cid:`) images
- Custom headers, including one-click `List-Unsubscribe` _(required by Gmail and Yahoo for bulk senders)_
- Tags, metadata, scheduled sending and idempotency keys _(provider dependant)_
- Open & click tracking _(provider dependant)_
- Inject css into html content
- Basic template support
- Max restrictions on `To`, `CC` and `BCC`
- Secrets are redacted when the configuration is logged or marshaled to JSON
- `BCC` recipients are never written into the message headers, and header values cannot inject new headers

<details>
<summary><strong><code>Supported Service Providers</code></strong></summary>
<br/>

- [AWS SES](https://docs.aws.amazon.com/ses/) _(tags & metadata become message tags; tracking via a configuration set)_
- [Mailgun](https://documentation.mailgun.com/) _(up to 10 tags; metadata becomes user variables; US or EU region)_
- [Mandrill](https://mandrillapp.com/api/docs/) _(recipients are not preserved: each `To` recipient only sees their own address)_
- [Postmark](https://postmarkapp.com/developer) _(one tag per email: multiple tags are joined with a comma)_
- [Resend](https://resend.com/docs) _(open & click tracking configured per domain)_
- [SendGrid](https://docs.sendgrid.com/) _(native open & click tracking)_
- [SMTP](https://en.wikipedia.org/wiki/Simple_Mail_Transfer_Protocol)

| Feature                   | AWS SES | Mailgun | Mandrill | Postmark | Resend | SendGrid | SMTP |
|---------------------------|:-------:|:-------:|:--------:|:--------:|:------:|:--------:|:----:|
| Tags                      |    ✓    |    ✓    |    ✓     |    ✓     |   ✓    |    ✓     |      |
| Metadata                  |    ✓    |    ✓    |    ✓     |    ✓     |   ✓    |    ✓     |      |
| Open / click tracking     |         |    ✓    |    ✓     |    ✓     |        |    ✓     |      |
| Scheduled send (`SendAt`) |         |    ✓    |    ✓     |          |   ✓    |    ✓     |      |
| Idempotency key           |         |         |          |          |   ✓    |          |      |
| Auto text                 |         |         |    ✓     |          |   ✓    |          |      |
| View content link         |         |         |    ✓     |          |        |          |      |

When an email uses a feature its provider does not support, go-mail logs a
warning (or returns `ErrUnsupportedFeature` when `StrictFeatures` is set). An
unsupported `SendAt` is always an error, so a scheduled email is never sent early.

Mailgun detects an attachment's content type from its file name and uses the
file name as the content id of an inline image, so inline attachments are sent
named by their `ContentID`: use a content id with an extension (ie: `logo.png`,
referenced as `cid:logo.png`).
</details>

<details>
<summary><strong><code>Sending, Failover & Results</code></strong></summary>
<br/>

```go
// Send returns the provider that accepted the email and its message id
result, err := mail.Send(ctx, email, gomail.Postmark, gomail.SendGrid) // tries Postmark, then SendGrid
if err != nil {
    return err
}
log.Printf("sent via %s: %s", result.Provider, result.MessageID)

// SendEmail is still available when only the error matters
err = mail.SendEmail(ctx, email, gomail.SMTP)
```

Every send is bounded by `SendTimeout` (default one minute) and honors the
context. Attachments added with a reader are buffered on the first send, so the
same email can be retried or failed over.
</details>

<details>
<summary><strong><code>Provider-Specific Features</code></strong></summary>
<br/>

Each provider has a typed option that edits its native request right before it
is sent, so anything the provider SDK supports is available. Options for other
providers are ignored.

```go
email.With(
    gomail.PostmarkOption(func(e *postmark.Email) { e.MessageStream = "broadcast" }),
    gomail.SendGridOption(func(m *mail.SGMailV3) { m.SetTemplateID("d-123") }),
    gomail.ResendOption(func(r *resend.SendEmailRequest) { r.TopicId = "topic_123" }),
    gomail.MailgunOption(func(m *mailgun.PlainMessage) { m.SetRequireTLS(true) }),
    gomail.MandrillOption(func(m *gochimp.Message) { m.Subaccount = "tenant-1" }),
    gomail.SESOption(func(in *ses.SendRawEmailInput) { in.FromArn = aws.String(arn) }),
)
```
</details>

<details>
<summary><strong><code>Custom Providers & Clients</code></strong></summary>
<br/>

Register any `Provider` (a new service, a built-in provider with your own
client, or a fake in tests). A registered provider is kept by `StartUp`.

```go
// A custom provider
const SparkPost gomail.ServiceProvider = 100
err := mail.RegisterProvider(SparkPost, mySparkPostProvider)

// A built-in provider with a custom client (ie: SendGrid EU data residency)
client := sendgrid.NewSendClient(apiKey)
client.Request, _ = sendgrid.SetDataResidency(client.Request, "eu")
err = mail.RegisterProvider(gomail.SendGrid, gomail.NewSendGridProvider(client))

// A fake provider in your tests
err = mail.RegisterProvider(gomail.SMTP, fakeProvider)
```
</details>

<details>
<summary><strong><code>Templates</code></strong></summary>
<br/>

```go
htmlTemplate, _ := email.ParseHTMLTemplate("welcome.html") // {{.Styles}} is replaced with email.CSS, which is inlined
textTemplate, _ := email.ParseTextTemplate("welcome.txt")  // text/template: no HTML escaping
err := email.ApplyTemplates(htmlTemplate, textTemplate, data)
```
</details>

<details>
<summary><strong><code>Configuration</code></strong></summary>
<br/>

A provider is loaded by `StartUp` for every service whose credentials are set.

| Field                                                         | Description                                                                                  |
|---------------------------------------------------------------|----------------------------------------------------------------------------------------------|
| `FromName`, `FromUsername`, `FromDomain`                      | Default sender (`FromUsername` and `FromDomain` are required)                                |
| `AwsSesAccessID`, `AwsSesSecretKey`                           | AWS SES static credentials                                                                   |
| `AwsSesUseIAMRole`                                            | Load AWS SES from the default credential chain instead of static keys                       |
| `AwsSesRegion`, `AwsSesEndpoint`, `AwsSesConfigurationSet`    | AWS SES region (default `us-east-1`), custom endpoint, and configuration set                 |
| `MailgunAPIKey`, `MailgunDomain`                              | Mailgun credentials and sending domain (defaults to the domain of the from address)          |
| `MailgunAPIBase`                                              | Mailgun API base URL (default US region; `mailgun.APIBaseEU` for the EU region)              |
| `MandrillAPIKey`, `PostmarkServerToken`                       | Mandrill and Postmark credentials                                                            |
| `ResendAPIKey`, `SendGridAPIKey`                              | Resend and SendGrid credentials                                                              |
| `SMTPHost`, `SMTPPort`, `SMTPUsername`, `SMTPPassword`        | SMTP server (port defaults to `587`; leave the username empty for a relay without auth)      |
| `SMTPImplicitTLS`                                             | Connect with TLS from the start (always on for port `465`)                                   |
| `AutoText`, `Important`, `TrackClicks`, `TrackOpens`          | Defaults copied to every email created by `NewEmail`                                         |
| `EmailCSS`                                                    | Default CSS copied to every email (used by `ParseHTMLTemplate`)                              |
| `MaxToRecipients`, `MaxCcRecipients`, `MaxBccRecipients`      | Recipient limits (default `50` each)                                                         |
| `MaxAttachmentSize`                                           | Total attachment bytes per email (default 40 MiB, negative for no limit)                     |
| `SendTimeout`                                                 | Maximum time for one provider send (default one minute, negative for no timeout)            |
| `StrictFeatures`                                              | Return `ErrUnsupportedFeature` instead of logging a warning for unsupported features          |
| `Logger`                                                      | `*slog.Logger` for warnings (default `slog.Default()`)                                       |
</details>

<details>
<summary><strong><code>SMTP</code></strong></summary>
<br/>

- Every send opens a new connection that honors the context deadline and cancellation
- STARTTLS is used whenever the server offers it; set `SMTPImplicitTLS` (or port `465`) for SMTPS
- Server certificates are verified; credentials are never sent over an unencrypted connection (except to localhost)
- `PLAIN` authentication is used, with `LOGIN` as a fallback for servers that only offer `LOGIN` (ie: Microsoft 365)
- Use `NewSMTPProvider` with `RegisterProvider` for a custom `tls.Config` or EHLO name
</details>

<details>
<summary><strong><code>Development Setup (Getting Started)</code></strong></summary>
<br/>

Install [MAGE-X](https://github.com/mrz1836/mage-x) build tool for development:

```bash
# Install MAGE-X for development and building
go install github.com/mrz1836/mage-x/cmd/magex@latest
magex update:install
```
</details>

<details>
<summary><strong><code>Library Deployment</code></strong></summary>
<br/>

This project uses [goreleaser](https://github.com/goreleaser/goreleaser) for streamlined binary and library deployment to GitHub. To get started, install it via:

```bash
brew install goreleaser
```

The release process is defined in the [.goreleaser.yml](.goreleaser.yml) configuration file.

Then create and push a new Git tag using:

```bash
magex version:bump bump=patch push=true branch=master
```

This process ensures consistent, repeatable releases with properly versioned artifacts and citation metadata.

</details>

<details>
<summary><strong><code>Build Commands</code></strong></summary>
<br/>

View all build commands

```bash script
magex help
```

</details>

<details>
<summary><strong><code>GitHub Workflows</code></strong></summary>
<br/>

All workflows are driven by modular configuration in [`.github/env/`](.github/env/README.md) — no YAML editing required.

**[View all workflows and the control center →](.github/docs/workflows.md)**

</details>

<details>
<summary><strong><code>Updating Dependencies</code></strong></summary>
<br/>

To update all dependencies (Go modules, linters, and related tools), run:

```bash
magex deps:update
```

This command ensures all dependencies are brought up to date in a single step, including Go modules and any managed tools. It is the recommended way to keep your development environment and CI in sync with the latest versions.

</details>

<br/>

## Examples & Tests
All unit tests and fuzz tests run via [GitHub Actions](https://github.com/mrz1836/go-mail/actions) and use [Go version 1.26.x](https://go.dev/doc/go1.26). View the [configuration file](.github/workflows/fortress.yml).

Run all tests (fast):

```bash script
magex test
```

Run all tests with race detector (slower):
```bash script
magex test:race
```

Run the fuzz tests:
```bash script
magex test:fuzz
```

<br/>

## Benchmarks
Run the Go benchmarks:
```shell script
magex bench
```

<br/>

## Code Standards
Read more about this Go project's [code standards](.github/CODE_STANDARDS.md).

<br/>

## 🤖 AI Usage & Assistant Guidelines
Read the [AI Usage & Assistant Guidelines](.github/tech-conventions/ai-compliance.md) for details on how AI is used in this project and how to interact with AI assistants.

<br/>

## Maintainers
| [<img src="https://github.com/mrz1836.png" height="50" alt="MrZ" />](https://github.com/mrz1836) |
|:------------------------------------------------------------------------------------------------:|
|                                [MrZ](https://github.com/mrz1836)                                 |

<br/>

## Contributing
View the [contributing guidelines](.github/CONTRIBUTING.md) and please follow the [code of conduct](.github/CODE_OF_CONDUCT.md).

### How can I help?
All kinds of contributions are welcome :raised_hands:!
The most basic way to show your support is to star :star2: the project, or to raise issues :speech_balloon:.
You can also support this project by [becoming a sponsor on GitHub](https://github.com/sponsors/mrz1836) :clap:
or by making a [**bitcoin donation**](https://mrz1818.com/?tab=tips&utm_source=github&utm_medium=sponsor-link&utm_campaign=go-mail&utm_term=go-mail&utm_content=go-mail) to ensure this journey continues indefinitely! :rocket:


[![Stars](https://img.shields.io/github/stars/mrz1836/go-mail?label=Please%20like%20us&style=social)](https://github.com/mrz1836/go-mail/stargazers)

<br/>

## License

[![License](https://img.shields.io/github/license/mrz1836/go-mail.svg?style=flat)](LICENSE)
