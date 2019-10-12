package gomail

import (
	"fmt"
	"testing"
)

// TestMailService_NewEmail tests the method NewEmail()
func TestMailService_NewEmail(t *testing.T) {
	mail := new(MailService)

	mail.AutoText = true
	mail.FromUsername = "no-reply"
	mail.FromName = "No Reply"
	mail.FromDomain = "example.com"
	mail.Important = true

	email := mail.NewEmail()

	if email.FromAddress != mail.FromUsername+"@"+mail.FromDomain {
		t.Fatal("FromAddress is invalid")
	}

	if email.ReplyToAddress != email.FromAddress {
		t.Fatal("ReplyToAddress is invalid")
	}

	if !email.AutoText {
		t.Fatal("AutoText is invalid")
	}

	if !email.Important {
		t.Fatal("Important is invalid")
	}

	if email.FromName != mail.FromName {
		t.Fatal("FromName is invalid")
	}
}

// ExampleMailService_NewEmail example using the NewEmail()
func ExampleMailService_NewEmail() {
	mail := new(MailService)
	mail.FromUsername = "no-reply"
	mail.FromName = "No Reply"
	mail.FromDomain = "example.com"

	email := mail.NewEmail()
	fmt.Printf("new email with from address: %s", email.FromAddress)
	// output: new email with from address: no-reply@example.com
}

// BenchmarkMailService_NewEmail runs benchmark on NewEmail()
func BenchmarkMailService_NewEmail(b *testing.B) {
	mail := new(MailService)
	mail.FromUsername = "no-reply"
	mail.FromName = "No Reply"
	mail.FromDomain = "example.com"
	for i := 0; i < b.N; i++ {
		_ = mail.NewEmail()
	}
}

// TestEmail_AddAttachment tests the method AddAttachment()
func TestEmail_AddAttachment(t *testing.T) {
	mail := new(MailService)
	mail.FromUsername = "no-reply"
	mail.FromName = "No Reply"
	mail.FromDomain = "example.com"

	email := mail.NewEmail()
	email.AddAttachment("testName", "testType", nil)
	email.AddAttachment("testName2", "testType2", nil)

	if len(email.Attachments) != 2 {
		t.Fatalf("expected 2 attachments, got: %d", len(email.Attachments))
	}

	if email.Attachments[0].FileName != "testName" || email.Attachments[0].FileType != "testType" {
		t.Fatalf("expected value was wrong, got: %s", email.Attachments[0].FileName)
	}

	if email.Attachments[1].FileName != "testName2" || email.Attachments[1].FileType != "testType2" {
		t.Fatalf("expected value was wrong, got: %s", email.Attachments[0].FileName)
	}
}

// ExampleEmail_AddAttachment example using the AddAttachment()
func ExampleEmail_AddAttachment() {
	mail := new(MailService)
	mail.FromUsername = "no-reply"
	mail.FromName = "No Reply"
	mail.FromDomain = "example.com"

	email := mail.NewEmail()
	email.AddAttachment("testName", "testType", nil)

	fmt.Printf("attachment: %s", email.Attachments[0].FileName)
	// output: attachment: testName
}

// BenchmarkEmail_AddAttachment runs benchmark on AddAttachment()
func BenchmarkEmail_AddAttachment(b *testing.B) {
	mail := new(MailService)
	mail.FromUsername = "no-reply"
	mail.FromName = "No Reply"
	mail.FromDomain = "example.com"
	email := mail.NewEmail()
	for i := 0; i < b.N; i++ {
		email.AddAttachment("testName", "testType", nil)
	}
}
