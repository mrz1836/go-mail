package gomail

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mailgun/mailgun-go/v5"
	"github.com/mattbaird/gochimp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestContainsServiceProvider will check the containsServiceProvider() method
func TestContainsServiceProvider(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		providers []ServiceProvider
		provider  ServiceProvider
		expected  bool
	}{
		{"provider found in single provider list", []ServiceProvider{AwsSes}, AwsSes, true},
		{"provider found in multiple provider list", []ServiceProvider{Mandrill, AwsSes, SMTP, Postmark}, AwsSes, true},
		{"provider not found in different provider list", []ServiceProvider{Mandrill}, AwsSes, false},
		{"provider not found in empty list", []ServiceProvider{}, AwsSes, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, containsServiceProvider(test.providers, test.provider))
		})
	}
}

// TestServiceProvider_String will test the String() method
func TestServiceProvider_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		provider ServiceProvider
		expected string
	}{
		{"aws ses", AwsSes, "AwsSes"},
		{"mandrill", Mandrill, "Mandrill"},
		{"postmark", Postmark, "Postmark"},
		{"smtp", SMTP, "SMTP"},
		{"sendgrid", SendGrid, "SendGrid"},
		{"resend", Resend, "Resend"},
		{"mailgun", Mailgun, "Mailgun"},
		{"unknown provider", ServiceProvider(999), "Unknown"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, test.provider.String())
		})
	}
}

// TestMailService_StartUpRecipientCaps will test that StartUp() respects
// explicitly configured recipient caps and only applies the defaults when unset
func TestMailService_StartUpRecipientCaps(t *testing.T) {
	t.Parallel()

	t.Run("defaults applied when caps are unset", func(t *testing.T) {
		service := &MailService{FromUsername: testUsernameEmail, FromDomain: testDomainEmail, MandrillAPIKey: "1234567"}
		require.NoError(t, service.StartUp())
		assert.Equal(t, maxToRecipients, service.MaxToRecipients)
		assert.Equal(t, maxCcRecipients, service.MaxCcRecipients)
		assert.Equal(t, maxBccRecipients, service.MaxBccRecipients)
	})

	t.Run("configured caps are preserved", func(t *testing.T) {
		service := &MailService{
			FromDomain:       testDomainEmail,
			FromUsername:     testUsernameEmail,
			MandrillAPIKey:   "1234567",
			MaxBccRecipients: 30,
			MaxCcRecipients:  20,
			MaxToRecipients:  10,
		}
		require.NoError(t, service.StartUp())
		assert.Equal(t, 10, service.MaxToRecipients)
		assert.Equal(t, 20, service.MaxCcRecipients)
		assert.Equal(t, 30, service.MaxBccRecipients)
	})
}

// TestMailService_StartUp will test the StartUp() method
func TestMailService_StartUp(t *testing.T) {
	t.Parallel()

	service := new(MailService)
	require.ErrorIs(t, service.StartUp(), ErrMissingFromUsername)

	service.FromUsername = "someone"
	require.ErrorIs(t, service.StartUp(), ErrMissingFromDomain)

	service.FromDomain = testDomainEmail
	require.ErrorIs(t, service.StartUp(), ErrNoServiceProvider)

	// Every provider loads from its credentials
	service.MandrillAPIKey = "1234567"
	service.AwsSesAccessID = "1234567"
	service.AwsSesSecretKey = "1234567"
	service.AwsSesEndpoint = "https://email.us-east-1.amazonaws.com"
	service.AwsSesConfigurationSet = "tracking"
	service.PostmarkServerToken = "1234567"
	service.SMTPHost = "smtp.example.com"
	service.SMTPPassword = "fake-password"
	service.SMTPUsername = "fake-username"
	service.SMTPPort = 465
	service.SendGridAPIKey = "1234567"
	service.ResendAPIKey = "re_1234567"
	service.MailgunAPIKey = "key-1234567"
	service.MailgunDomain = "mg.example.com"
	require.NoError(t, service.StartUp())
	assert.Equal(t, []ServiceProvider{Mandrill, AwsSes, Postmark, SMTP, SendGrid, Resend, Mailgun}, service.AvailableProviders)

	assert.IsType(t, &MandrillProvider{}, service.providers[Mandrill])
	assert.IsType(t, &PostmarkProvider{}, service.providers[Postmark])
	assert.IsType(t, &SendGridProvider{}, service.providers[SendGrid])
	assert.IsType(t, &ResendProvider{}, service.providers[Resend])

	mailgunProvider, ok := service.providers[Mailgun].(*MailgunProvider)
	require.True(t, ok)
	assert.Equal(t, "mg.example.com", mailgunProvider.domain)

	sesProvider, ok := service.providers[AwsSes].(*SESProvider)
	require.True(t, ok)
	assert.Equal(t, "tracking", sesProvider.configurationSet)

	smtpProvider, ok := service.providers[SMTP].(*SMTPProvider)
	require.True(t, ok)
	assert.Equal(t, SMTPConfig{
		Host:        "smtp.example.com",
		ImplicitTLS: true,
		LocalName:   smtpDefaultLocalName,
		Password:    "fake-password",
		Port:        465,
		Username:    "fake-username",
	}, smtpProvider.config)
}

// TestMailService_StartUpIdempotent checks StartUp can run again without duplicates
func TestMailService_StartUpIdempotent(t *testing.T) {
	t.Parallel()

	service := &MailService{FromUsername: testUsernameEmail, FromDomain: testDomainEmail, MandrillAPIKey: "key", ResendAPIKey: "re_key"}
	require.NoError(t, service.StartUp())
	first := service.providers[Mandrill]

	require.NoError(t, service.StartUp())
	assert.Equal(t, []ServiceProvider{Mandrill, Resend}, service.AvailableProviders)
	assert.Same(t, first, service.providers[Mandrill], "an existing provider is kept")
}

// TestMailService_StartUpKeepsRegisteredProvider checks a registered provider is not replaced
func TestMailService_StartUpKeepsRegisteredProvider(t *testing.T) {
	t.Parallel()

	fake := &fakeProvider{}
	service := &MailService{FromUsername: testUsernameEmail, FromDomain: testDomainEmail, SendGridAPIKey: "key"}
	require.NoError(t, service.RegisterProvider(SendGrid, fake))
	require.NoError(t, service.StartUp())

	assert.Same(t, fake, service.providers[SendGrid])
	assert.Equal(t, []ServiceProvider{SendGrid}, service.AvailableProviders)
}

// TestMailService_StartUpCustomProviderOnly checks a registered custom provider is enough to start
func TestMailService_StartUpCustomProviderOnly(t *testing.T) {
	t.Parallel()

	const custom ServiceProvider = 100
	service := &MailService{FromUsername: testUsernameEmail, FromDomain: testDomainEmail}
	require.NoError(t, service.RegisterProvider(custom, &fakeProvider{}))
	require.NoError(t, service.StartUp())
	assert.Equal(t, []ServiceProvider{custom}, service.AvailableProviders)
}

// TestMailService_StartUpAwsSesIAMRole tests that the AWS SES provider loads via
// the default credential chain when AwsSesUseIAMRole is set and no static access
// keys are supplied.
func TestMailService_StartUpAwsSesIAMRole(t *testing.T) {
	t.Parallel()

	service := &MailService{
		AwsSesRegion:     awsSesDefaultRegion,
		AwsSesUseIAMRole: true,
		FromDomain:       testDomainEmail,
		FromUsername:     testUsernameEmail,
	}
	require.NoError(t, service.StartUp())
	assert.True(t, containsServiceProvider(service.AvailableProviders, AwsSes))

	// Static keys remain optional: with neither keys nor the flag, SES is skipped
	other := &MailService{FromUsername: testUsernameEmail, FromDomain: testDomainEmail}
	require.ErrorIs(t, other.StartUp(), ErrNoServiceProvider)
	assert.False(t, containsServiceProvider(other.AvailableProviders, AwsSes))
}

// TestMailService_StartUpMandrillTimeout checks the Mandrill client timeout follows SendTimeout
func TestMailService_StartUpMandrillTimeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		timeout  time.Duration
		expected time.Duration
	}{
		{"default", 0, defaultSendTimeout},
		{"configured", 5 * time.Second, 5 * time.Second},
		{"disabled", -1, 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &MailService{FromUsername: testUsernameEmail, FromDomain: testDomainEmail, MandrillAPIKey: "key", SendTimeout: test.timeout}
			require.NoError(t, service.StartUp())

			provider, ok := service.providers[Mandrill].(*MandrillProvider)
			require.True(t, ok)
			api, ok := provider.client.(*gochimp.MandrillAPI)
			require.True(t, ok)
			assert.Equal(t, test.expected, api.Timeout)
		})
	}
}

// TestMailService_StartUpMailgunAPIBase checks the Mailgun client uses the configured API base
func TestMailService_StartUpMailgunAPIBase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		apiBase  string
		expected string
	}{
		{"default us region", "", mailgun.APIBaseUS},
		{"eu region", mailgun.APIBaseEU, mailgun.APIBaseEU},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &MailService{FromUsername: testUsernameEmail, FromDomain: testDomainEmail, MailgunAPIKey: "key", MailgunAPIBase: test.apiBase}
			require.NoError(t, service.StartUp())

			provider, ok := service.providers[Mailgun].(*MailgunProvider)
			require.True(t, ok)
			client, ok := provider.client.(*mailgun.Client)
			require.True(t, ok)
			assert.Equal(t, test.expected, client.APIBase())
			assert.Empty(t, provider.domain, "the sending domain defaults to the from address domain")
		})
	}

	t.Run("api base with a version is rejected", func(t *testing.T) {
		service := &MailService{FromUsername: testUsernameEmail, FromDomain: testDomainEmail, MailgunAPIKey: "key", MailgunAPIBase: "https://api.mailgun.net/v3"}
		err := service.StartUp()
		require.ErrorContains(t, err, "invalid MailgunAPIBase")
		assert.False(t, service.hasProvider(Mailgun))
	})
}

// TestMailService_RegisterProvider checks provider registration
func TestMailService_RegisterProvider(t *testing.T) {
	t.Parallel()

	service := new(MailService)
	require.ErrorIs(t, service.RegisterProvider(SMTP, nil), ErrNilProvider)
	assert.Empty(t, service.AvailableProviders)

	first, second := &fakeProvider{}, &fakeProvider{}
	require.NoError(t, service.RegisterProvider(SMTP, first))
	require.NoError(t, service.RegisterProvider(SMTP, second))
	assert.Equal(t, []ServiceProvider{SMTP}, service.AvailableProviders)
	assert.Same(t, second, service.providers[SMTP], "registering again replaces the provider")
}

// TestMailService_Defaults checks the timeout, size limit and logger defaults
func TestMailService_Defaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		timeout         time.Duration
		size            int64
		expectedTimeout time.Duration
		expectedSize    int64
	}{
		{"defaults", 0, 0, defaultSendTimeout, defaultMaxAttachmentSize},
		{"disabled", -1, -1, 0, 0},
		{"configured", time.Second, 10, time.Second, 10},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &MailService{SendTimeout: test.timeout, MaxAttachmentSize: test.size}
			assert.Equal(t, test.expectedTimeout, service.sendTimeout())
			assert.Equal(t, test.expectedSize, service.maxAttachmentSize())
		})
	}

	t.Run("logger", func(t *testing.T) {
		assert.Same(t, slog.Default(), new(MailService).logger())
		custom := slog.New(slog.DiscardHandler)
		assert.Same(t, custom, (&MailService{Logger: custom}).logger())
	})
}

// newSecretService returns a service with every secret set
func newSecretService() MailService {
	return MailService{
		AwsSesAccessID:      "AKIAEXAMPLE",
		AwsSesSecretKey:     "aws-secret",
		FromDomain:          testDomainEmail,
		MailgunAPIKey:       "mailgun-secret",
		MandrillAPIKey:      "mandrill-secret",
		PostmarkServerToken: "postmark-secret",
		ResendAPIKey:        "resend-secret",
		SMTPPassword:        "smtp-secret",
		SendGridAPIKey:      "sendgrid-secret",
	}
}

// secretValues are the secrets set by newSecretService
func secretValues() []string {
	return []string{"aws-secret", "mailgun-secret", "mandrill-secret", "postmark-secret", "resend-secret", "smtp-secret", "sendgrid-secret"}
}

// TestMailService_RedactsSecrets checks secrets never appear in JSON or fmt output
func TestMailService_RedactsSecrets(t *testing.T) {
	t.Parallel()

	service := newSecretService()
	require.NoError(t, service.RegisterProvider(Postmark, &fakeProvider{}))

	valueJSON, err := json.Marshal(service)
	require.NoError(t, err)
	pointerJSON, err := json.Marshal(&service)
	require.NoError(t, err)

	outputs := map[string]string{
		"json value":   string(valueJSON),
		"json pointer": string(pointerJSON),
		"%v value":     fmt.Sprintf("%v", service),
		"%+v pointer":  fmt.Sprintf("%+v", &service),
		"%#v value":    fmt.Sprintf("%#v", service),
		"sprint":       fmt.Sprint(&service),
	}
	for name, output := range outputs {
		t.Run(name, func(t *testing.T) {
			for _, secret := range secretValues() {
				assert.NotContains(t, output, secret)
			}
			assert.Contains(t, output, redactedValue)
			assert.Contains(t, output, "AKIAEXAMPLE", "non-secret fields are kept")
		})
	}

	t.Run("json keeps the fields", func(t *testing.T) {
		var decoded map[string]any
		require.NoError(t, json.Unmarshal(valueJSON, &decoded))
		assert.Equal(t, redactedValue, decoded["sendgrid_api_key"])
		assert.Equal(t, testDomainEmail, decoded["from_domain"])
		assert.NotContains(t, decoded, "Logger")
	})

	t.Run("original is unchanged", func(t *testing.T) {
		assert.Equal(t, "sendgrid-secret", service.SendGridAPIKey)
	})

	t.Run("empty secrets stay empty", func(t *testing.T) {
		output, marshalErr := json.Marshal(MailService{})
		require.NoError(t, marshalErr)
		assert.NotContains(t, string(output), redactedValue)
	})

	t.Run("unmarshal still reads secrets", func(t *testing.T) {
		var loaded MailService
		require.NoError(t, json.Unmarshal([]byte(`{"sendgrid_api_key":"from-config"}`), &loaded))
		assert.Equal(t, "from-config", loaded.SendGridAPIKey)
	})
}

// TestMailService_LoadAwsSesClientEndpoint checks the custom SES endpoint
func TestMailService_LoadAwsSesClientEndpoint(t *testing.T) {
	t.Parallel()

	service := &MailService{AwsSesAccessID: "id", AwsSesSecretKey: "secret", AwsSesEndpoint: "http://localhost:4566"}
	client, err := service.loadAwsSesClient(true)
	require.NoError(t, err)
	require.NotNil(t, client.Options().BaseEndpoint)
	assert.Equal(t, "http://localhost:4566", *client.Options().BaseEndpoint)
	assert.Equal(t, awsSesDefaultRegion, client.Options().Region)

	// The endpoint is copied, so later config changes do not affect the client
	service.AwsSesEndpoint = "changed"
	assert.Equal(t, "http://localhost:4566", *client.Options().BaseEndpoint)
}

// TestMailService_StartUpAwsSesConfigError checks an AWS config failure is returned
func TestMailService_StartUpAwsSesConfigError(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "config")
	require.NoError(t, os.WriteFile(configFile, []byte("[default]\nregion = us-east-1\n"), 0o600))
	t.Setenv("AWS_CONFIG_FILE", configFile)
	t.Setenv("AWS_PROFILE", "go-mail-missing-profile")

	service := &MailService{
		AwsSesUseIAMRole: true,
		FromDomain:       testDomainEmail,
		FromUsername:     testUsernameEmail,
		MandrillAPIKey:   "key",
	}
	err := service.StartUp()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load AWS config")
	assert.False(t, service.hasProvider(AwsSes))
}
