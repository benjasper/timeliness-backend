package email

import (
	"context"
	sendinblue "github.com/sendinblue/APIv3-go-library/lib"
	"strconv"
)

// Mailer is the interface email services can implement
type Mailer interface {
	SendEmail(ctx context.Context, mail *Email) error
	AddToList(ctx context.Context, email string, list string) error
	IsInList(ctx context.Context, email string, list string) bool
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

// UnconfirmedListID is the list ID for unconfirmed users
const UnconfirmedListID = "2"

// AppUsersListID is the list ID for app users
const AppUsersListID = "5"

// EarlyAccessUsersListID is the list ID for app users
const EarlyAccessUsersListID = "6"

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

// AddToList adds a user to an email list
func (s *SendInBlueService) AddToList(ctx context.Context, email string, list string) error {
	templateID, err := strconv.Atoi(list)
	if err != nil {
		return err
	}

	info, _, err := s.mailer.ContactsApi.GetContactInfo(ctx, email)
	if err != nil {
		_, _, err = s.mailer.ContactsApi.CreateContact(ctx, sendinblue.CreateContact{
			Email: email,
			ListIds: []int64{
				int64(templateID),
			},
		})
		if err != nil {
			return err
		}

		return err
	}

	isInList := false
	for _, id := range info.ListIds {
		if id == int64(templateID) {
			isInList = true
			break
		}
	}

	if !isInList {
		_, _, err = s.mailer.ContactsApi.AddContactToList(ctx, sendinblue.AddContactToList{
			Emails: []string{
				email,
			},
		}, int64(templateID))
		if err != nil {
			return err
		}
	}

	return nil
}

// IsInList checks if a user is in a list
func (s *SendInBlueService) IsInList(ctx context.Context, email string, list string) bool {
	templateID, err := strconv.Atoi(list)
	if err != nil {
		return false
	}

	info, _, err := s.mailer.ContactsApi.GetContactInfo(ctx, email)
	if err != nil {
		return false
	}

	isInList := false
	for _, id := range info.ListIds {
		if id == int64(templateID) {
			isInList = true
			break
		}
	}

	return isInList
}
