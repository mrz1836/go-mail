package gomail

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestContainsServiceProvider will check the containsServiceProvider() method
func TestContainsServiceProvider(t *testing.T) {
	t.Parallel()

	// Create the list of tests
	tests := []struct {
		name      string
		providers []ServiceProvider
		provider  ServiceProvider
		expected  bool
	}{
		{"provider found in single provider list", []ServiceProvider{AwsSes}, AwsSes, true},
		{"provider found in multiple provider list", []ServiceProvider{Mandrill, AwsSes, SMTP, Postmark}, AwsSes, true},
		{"provider found in two provider list", []ServiceProvider{Mandrill, AwsSes}, AwsSes, true},
		{"provider not found in different provider list", []ServiceProvider{Mandrill}, AwsSes, false},
		{"provider not found in single different provider", []ServiceProvider{SMTP}, AwsSes, false},
		{"provider not found in empty list", []ServiceProvider{}, AwsSes, false},
	}

	// Loop tests
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			found := containsServiceProvider(test.providers, test.provider)
			assert.Equal(t, test.expected, found)
		})
	}
}

// TestMailService_StartUp will test the StartUp() method
func TestMailService_StartUp(t *testing.T) {
	t.Parallel()

	service := new(MailService)
	err := service.StartUp()

	// No username
	require.Error(t, err)
	assert.Equal(t, "missing required field: from_username", err.Error())

	// No domain
	service.FromUsername = "someone"
	err = service.StartUp()
	require.Error(t, err)
	assert.Equal(t, "missing required field: from_domain", err.Error())

	// No providers
	service.FromDomain = "example.com"
	err = service.StartUp()
	require.Error(t, err)
	assert.Equal(t, "attempted to startup the email service provider(s) however there's no available service provider", err.Error())

	// Add Mandrill api key
	service.MandrillAPIKey = "1234567"
	err = service.StartUp()
	require.NoError(t, err)

	// Add AWS credentials
	service.AwsSesAccessID = "1234567"
	service.AwsSesSecretKey = "1234567"
	service.AwsSesEndpoint = awsSesDefaultEndpoint
	service.AwsSesRegion = awsSesDefaultRegion
	err = service.StartUp()
	require.NoError(t, err)

	// Add postmark credentials
	service.PostmarkServerToken = "1234567"
	err = service.StartUp()
	require.NoError(t, err)

	// Add SMTP
	service.SMTPHost = "example.com"
	service.SMTPPassword = "fake-password"
	service.SMTPUsername = "fake-username"
	service.SMTPPort = 25
	err = service.StartUp()
	require.NoError(t, err)
}
