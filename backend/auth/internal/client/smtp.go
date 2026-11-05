package client

import (
	"net/smtp"
	"os"
)

// SMTPClient sends emails from the service account
type SMTPClient interface {
	// SendOTP mails the email verification code to to
	SendOTP(to, otp string) error
	// SendAuthenticatorChanged tells the owner of to that a new authenticator replaced the old one
	SendAuthenticatorChanged(to string) error
}

type smtpClient struct{}

func NewSMTPClient() SMTPClient {
	return smtpClient{}
}

func (smtpClient) SendOTP(to, otp string) error {
	headers := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";"
	message := "Subject: " + appName() + " : Verify your email\n" + headers + "\n\n" + otp + "\ncode is valid for 5 minutes"
	return send(to, message)
}

func (smtpClient) SendAuthenticatorChanged(to string) error {
	headers := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";"
	message := "Subject: " + appName() + " : Your authenticator was changed\n" + headers + "\n\n" +
		"A new authenticator was just set up for your account. If this wasn't you, secure your email account, " +
		"then use \"Lost your phone?\" when you log in to set up your own again."
	return send(to, message)
}

func appName() string {
	name := os.Getenv("APP_NAME")
	if name == "" {
		return "Howwasread"
	}
	return name
}

func send(to, message string) error {
	from := os.Getenv("FROM_EMAIL")
	auth := smtp.PlainAuth("", from, os.Getenv("FROM_EMAIL_PASSWORD"), os.Getenv("FROM_EMAIL_SMTP"))
	message = "From: " + appName() + " <" + from + ">\nTo: " + to + "\n" + message
	return smtp.SendMail(os.Getenv("SMTP_ADDR"), auth, from, []string{to}, []byte(message))
}
