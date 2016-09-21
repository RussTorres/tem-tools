package service

import (
	"fmt"
	"net/smtp"
	"strings"
	"time"

	"imagecatcher/config"
	"imagecatcher/logger"
)

// MessageNotifier responsible with sending a notification in case of an error
type MessageNotifier interface {
	// SendMessage sends the message. Typically the messages are throttled so that
	// the it doesn't fill the receiver box but if force is set to true it tries to send it immediately
	SendMessage(message string, force bool)
}

// emailNotifier email error notifier
type emailNotifier struct {
	messageChannel             chan string
	instanceName               string
	senderUserName             string
	senderEmail                string
	senderPassword             string
	smtpHostname               string
	smtpPort                   int
	recipients                 []string
	lastEmailSent              time.Time
	minTimeBetweenEmailsInMins float64
}

// NewEmailNotifier creates an instance of an email error notifier using
// settings defined in the given configuration
func NewEmailNotifier(instanceName string, config config.Config) MessageNotifier {
	var emailrecipients = make([]string, 0)
	recipients := config["ERROR_NOTIFICATION_RECIPIENTS"]
	switch v := recipients.(type) {
	case string:
		emailrecipients = append(emailrecipients, v)
	case []interface{}:
		for _, recipient := range v {
			emailrecipients = append(emailrecipients, recipient.(string))
		}
	}

	notifier := &emailNotifier{
		messageChannel:             make(chan string, 1),
		instanceName:               instanceName,
		senderUserName:             config.GetStringProperty("MAIL_USERNAME", ""),
		senderPassword:             config.GetStringProperty("MAIL_PASSWORD", ""),
		senderEmail:                config.GetStringProperty("MAIL_SENDER", ""),
		smtpHostname:               config.GetStringProperty("MAIL_SERVER", ""),
		smtpPort:                   config.GetIntProperty("MAIL_PORT", 0),
		recipients:                 emailrecipients,
		minTimeBetweenEmailsInMins: config.GetFloat64Property("MIN_TIME_BETWEEN_EMAIL_NOTIFS_IN_MINS", 10),
	}
	go notifier.waitLoop()
	return notifier
}

// SendMessage - MessageNotifier method
func (n *emailNotifier) SendMessage(message string, force bool) {
	if force {
		n.sendEmail(message)
		return
	}
	select {
	case n.messageChannel <- message:
		return
	default:
		logger.Infof("Message channel is full - dropping %v", message)
		return
	}
}

func (n *emailNotifier) waitLoop() {
	timeBetweenEmails := time.Duration(n.minTimeBetweenEmailsInMins) * time.Minute
	for {
		select {
		case msg := <-n.messageChannel:
			if time.Since(n.lastEmailSent).Minutes() < n.minTimeBetweenEmailsInMins {
				return
			}
			n.sendEmail(msg)
			time.Sleep(timeBetweenEmails)
		}
	}
}

func (n *emailNotifier) sendEmail(message string) {
	var auth smtp.Auth
	if n.senderPassword != "" {
		auth = smtp.PlainAuth("", n.senderUserName, n.senderPassword, n.smtpHostname)
	}

	msg := []byte("To: " + strings.Join(n.recipients, ", ") + "\r\n" +
		"Subject: " + fmt.Sprintf("Message from Image Catcher running at %s", n.instanceName) + "\r\n" +
		"\r\n" +
		message + "\r\n")
	var smtpAddr string
	if n.smtpPort > 0 {
		smtpAddr = fmt.Sprintf("%s:%d", n.smtpHostname, n.smtpPort)
	} else {
		smtpAddr = n.smtpHostname
	}
	if smtpErr := smtp.SendMail(smtpAddr, auth, n.senderEmail, n.recipients, msg); smtpErr != nil {
		logger.Errorf("Error while trying to send the email notification to %v regarding %v: %s", n.recipients, message, smtpErr)
		return
	}
	n.lastEmailSent = time.Now()
}
