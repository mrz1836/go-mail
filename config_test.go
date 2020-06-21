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
			t.Errorf("%s Failed: [%v] providers, [%d] provider, found but expected to fail", t.Name(), test.providers, test.provider)
		} else if !found && test.expected {
			t.Errorf("%s Failed: [%v] providers, [%d] provider, NOT found but expected to succeed", t.Name(), test.providers, test.provider)
		}
	}
}
