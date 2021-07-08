package gomail

import "testing"

// TestContainsServiceProvider will check the containsServiceProvider() method
func TestContainsServiceProvider(t *testing.T) {
	t.Parallel()

	// Create the list of tests
	var tests = []struct {
		providers []ServiceProvider
		provider  ServiceProvider
		expected  bool
	}{
		{[]ServiceProvider{AwsSes}, AwsSes, true},
		{[]ServiceProvider{Mandrill, AwsSes, SMTP, Postmark}, AwsSes, true},
		{[]ServiceProvider{Mandrill, AwsSes}, AwsSes, true},
		{[]ServiceProvider{Mandrill}, AwsSes, false},
		{[]ServiceProvider{SMTP}, AwsSes, false},
		{[]ServiceProvider{}, AwsSes, false},
	}

	// Loop tests
	for _, test := range tests {
		if found := containsServiceProvider(test.providers, test.provider); found && !test.expected {
			t.Fatalf("%s Failed: [%v] providers, [%d] provider, found but expected to fail", t.Name(), test.providers, test.provider)
		} else if !found && test.expected {
			t.Fatalf("%s Failed: [%v] providers, [%d] provider, NOT found but expected to succeed", t.Name(), test.providers, test.provider)
		}
	}
}

// TestMailService_StartUp will test the StartUp() method
func TestMailService_StartUp(t *testing.T) {
	t.Parallel()

	service := new(MailService)
	err := service.StartUp()

	// No username
	if err == nil || err.Error() != "missing required field: from_username" {
		t.Fatalf("%s Failed: expected an error for missing from name, error: %v", t.Name(), err)
	}

	// No domain
	service.FromUsername = "someone"
	err = service.StartUp()
	if err == nil || err.Error() != "missing required field: from_domain" {
		t.Fatalf("%s Failed: expected an error for missing from domain, error: %v", t.Name(), err)
	}

	// No providers
	service.FromDomain = "example.com"
	err = service.StartUp()
	if err == nil || err.Error() != "attempted to startup the email service provider(s) however there's no available service provider" {
		t.Fatalf("%s Failed: expected an error for missing a provider, error: %v", t.Name(), err)
	}

	// Add Mandrill api key
	service.MandrillAPIKey = "1234567"
	err = service.StartUp()
	if err != nil {
		t.Fatalf("%s Failed: error should not have occurred, error: %s", t.Name(), err.Error())
	}

	// Add AWS credentials
	service.AwsSesAccessID = "1234567"
	service.AwsSesSecretKey = "1234567"
	service.AwsSesEndpoint = awsSesDefaultEndpoint
	service.AwsSesRegion = awsSesDefaultRegion
	err = service.StartUp()
	if err != nil {
		t.Fatalf("%s Failed: error should not have occurred, error: %s", t.Name(), err.Error())
	}

	// Add postmark credentials
	service.PostmarkServerToken = "1234567"
	err = service.StartUp()
	if err != nil {
		t.Fatalf("%s Failed: error should not have occurred, error: %s", t.Name(), err.Error())
	}

	// Add SMTP
	service.SMTPHost = "example.com"
	service.SMTPPassword = "fake-password"
	service.SMTPUsername = "fake-username"
	service.SMTPPort = 25
	err = service.StartUp()
	if err != nil {
		t.Fatalf("%s Failed: error should not have occurred, error: %s", t.Name(), err.Error())
	}
}
