package email

import (
	"fmt"
	"net/smtp"
	"os"
)

func SendMail(recipient string, recipient_password string, api_key string, api_secret string) {
	from := "admin@example.com"
	to := recipient
	subject := "Welcome Onboard"
	body := fmt.Sprintf(`
	You can login into our website using this credentials:
	Email: %s
	Passowrd: %s
	API Key: %s
	API Secret: %s 
	`, recipient, recipient_password, api_key, api_secret)

	host, hostExists := os.LookupEnv("MAIL_HOST")
	port, portExists := os.LookupEnv("MAIL_PORT")
	username, usernameExists := os.LookupEnv("MAIL_USERNAME")
	password, passwordExists := os.LookupEnv("MAIL_PASSWORD")

	// If one of the is not exists, then do nothing
	if !hostExists || !portExists || !usernameExists || !passwordExists {
		return
	}

	mime := "MIME-version: 1.0;\nContent-Type: text/plain; charset=\"UTF-8\";\n\n"
	msg := []byte("From: " + from + "\n" +
		"To: " + to + "\n" +
		"Subject: " + subject + "\n" +
		mime + "\n" +
		body)

	auth := smtp.PlainAuth("", username, password, host)

	err := smtp.SendMail(host+":"+port, auth, from, []string{to}, msg)
	if err != nil {
		fmt.Println("Error sending email:", err)
		return
	}

	fmt.Println("Email sent successfully")

}
