package email

import (
	"context"
	sendinblue "github.com/sendinblue/APIv3-go-library/lib"
	"strconv"
)

// Mailer is the interface email services can implement
type Mailer interface {
	SendEmail(ctx context.Context, mail *Email) error
}

// Email is a struct that contains information to send an email
type Email struct {
	ReceiverName    string
	ReceiverAddress string
	Template        string
	Parameters      map[string]interface{}
}

// SendInBlueService is an implementation of Mailer
type SendInBlueService struct {
	mailer *sendinblue.APIClient
}

// ReplyToName the reply to name for all emails
const ReplyToName = "Timeliness"

// ReplyToEmail the reply to email for all emails
const ReplyToEmail = "hello@timeliness.app"

// NewSendInBlueService constructs a new SendInBlueService
func NewSendInBlueService(apiKey string) *SendInBlueService {
	service := SendInBlueService{}

	cfg := sendinblue.NewConfiguration()

	cfg.AddDefaultHeader("api-key", apiKey)

	service.mailer = sendinblue.NewAPIClient(cfg)

	return &service
}

// SendEmail sends an email
func (s *SendInBlueService) SendEmail(ctx context.Context, mail *Email) error {
	templateID, err := strconv.Atoi(mail.Template)
	if err != nil {
		return err
	}

	params := interface{}(mail.Parameters)

	_, _, err = s.mailer.TransactionalEmailsApi.SendTransacEmail(ctx, sendinblue.SendSmtpEmail{
		TemplateId: int64(templateID),
		To: []sendinblue.SendSmtpEmailTo{
			{
				Email: mail.ReceiverAddress,
				Name:  mail.ReceiverName,
			},
		},
		ReplyTo: &sendinblue.SendSmtpEmailReplyTo{
			Name:  ReplyToName,
			Email: ReplyToEmail,
		},
		Params: &params,
	})
	if err != nil {
		return err
	}

	return nil
}
