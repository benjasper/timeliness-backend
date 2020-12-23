package calendar

import (
	"context"
	"errors"
	"github.com/timeliness-app/timeliness-backend/pkg/logger"
	"github.com/timeliness-app/timeliness-backend/pkg/users"
	"google.golang.org/api/option"
	"io/ioutil"
	"log"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	gcalendar "google.golang.org/api/calendar/v3"
)

var ErrorInvalidToken = errors.New("google token is invalid")

type CalendarRepository struct {
	Config  *oauth2.Config
	Logger  logger.Interface
	Service *gcalendar.Service
}

func ReadGoogleConfig() (*oauth2.Config, error) {
	b, err := ioutil.ReadFile("./keys/credentials.json")
	if err != nil {
		log.Fatal(err)
		return nil, err
	}

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, gcalendar.CalendarReadonlyScope, gcalendar.CalendarScope)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}

	return config, nil
}

func NewCalendarRepository(u *users.User) (*CalendarRepository, error) {
	newRepo := CalendarRepository{}

	config, _ := ReadGoogleConfig()
	newRepo.Config = config

	if u.GoogleCalendarConnection.Token.AccessToken == "" || u.GoogleCalendarConnection.Token.Expiry.Before(time.Now()) {
		return nil, ErrorInvalidToken
	}

	client := newRepo.Config.Client(context.Background(), &u.GoogleCalendarConnection.Token)

	srv, err := gcalendar.NewService(context.TODO(), option.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}

	newRepo.Service = srv

	return &newRepo, nil
}

func (c *CalendarRepository) CreateCalendar() (string, error) {
	// calendarId := "refm50ua0bukpdmp52a84cgshk@group.gcalendar.google.com"
	newCalendar := gcalendar.Calendar{
		Summary: "Tasks",
	}
	cal, err := c.Service.Calendars.Insert(&newCalendar).Do()
	if err != nil {
		return "", err
	}

	return cal.Id, nil
}

func (c *CalendarRepository) AddBusytimesToWindow(window *TimeWindow) error {
	calList, err := c.Service.CalendarList.List().Do()
	if err != nil {
		return err
	}

	var items = make([]*gcalendar.FreeBusyRequestItem, len(calList.Items))
	for _, cal := range calList.Items {
		items = append(items, &gcalendar.FreeBusyRequestItem{Id: cal.Id})
	}

	response, err := c.Service.Freebusy.Query(&gcalendar.FreeBusyRequest{
		TimeMin: window.Start.Format(time.RFC3339),
		TimeMax: window.End.Format(time.RFC3339),
		Items:   items}).Do()
	if err != nil {
		return err
	}

	for _, v := range response.Calendars {
		for _, period := range v.Busy {
			start, err := time.Parse(time.RFC3339, period.Start)
			if err != nil {
				return err
			}

			end, err := time.Parse(time.RFC3339, period.End)
			if err != nil {
				return err
			}

			window.AddToBusy(Timespan{Start: start, End: end})
		}
	}

	return nil
}
