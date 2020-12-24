package calendar

import (
	"context"
	"github.com/google/uuid"
	"github.com/timeliness-app/timeliness-backend/pkg/tasks/calendar/google"
	"github.com/timeliness-app/timeliness-backend/pkg/users"
	"golang.org/x/oauth2"
)

type CalendarService struct {
}

func (c *CalendarService) GetGoogleToken(context context.Context, u *users.User, authCode string) (*oauth2.Token, error) {
	config, _ := google.ReadGoogleConfig()

	tok, err := config.Exchange(context, authCode, oauth2.AccessTypeOffline)
	if err != nil {
		return nil, err
	}
	return tok, nil
}

func (c *CalendarService) GetGoogleAuthURL(u *users.User) (string, string, error) {
	config, err := google.ReadGoogleConfig()
	if err != nil {
		return "", "", err
	}

	stateToken := uuid.New().String()

	u.GoogleCalendarConnection.StateToken = stateToken
	url := config.AuthCodeURL(stateToken, oauth2.AccessTypeOffline)

	return url, stateToken, nil
}

func (c *CalendarService) SuggestTimeslot(u *users.User, window *TimeWindow) (*[]Timespan, error) {
	calendarRepository, err := NewCalendarRepository(u)
	if err != nil {
		return nil, err
	}

	err = calendarRepository.AddBusytimesToWindow(window)
	if err != nil {
		return nil, err
	}

	free := window.ComputeFree()
	return &free, nil
}
