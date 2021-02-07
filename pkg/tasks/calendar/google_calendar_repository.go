package calendar

import (
	"context"
	"errors"
	"github.com/timeliness-app/timeliness-backend/internal/google"
	"github.com/timeliness-app/timeliness-backend/pkg/logger"
	"github.com/timeliness-app/timeliness-backend/pkg/users"
	"google.golang.org/api/option"
	"time"

	"golang.org/x/oauth2"
	gcalendar "google.golang.org/api/calendar/v3"
)

// ErrorInvalidToken is an error thrown if the google token is invalid
var ErrorInvalidToken = errors.New("google token is invalid")

// GoogleCalendarRepository provides function for easily editing the users google calendar
type GoogleCalendarRepository struct {
	Config  *oauth2.Config
	Logger  logger.Interface
	Service *gcalendar.Service
	ctx     context.Context
	user    *users.User
}

// NewGoogleCalendarRepository constructs a GoogleCalendarRepository
func NewGoogleCalendarRepository(ctx context.Context, u *users.User) (*GoogleCalendarRepository, error) {
	newRepo := GoogleCalendarRepository{}

	newRepo.ctx = ctx

	config, _ := google.ReadGoogleConfig()
	newRepo.Config = config

	if u.GoogleCalendarConnection.Token.AccessToken == "" {
		return nil, ErrorInvalidToken
	}

	if u.GoogleCalendarConnection.Token.Expiry.Before(time.Now()) {
		source := newRepo.Config.TokenSource(ctx, &u.GoogleCalendarConnection.Token)
		newToken, err := source.Token()
		if err != nil {
			return nil, err
		}
		u.GoogleCalendarConnection.Token = *newToken
	}

	client := newRepo.Config.Client(ctx, &u.GoogleCalendarConnection.Token)

	srv, err := gcalendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}

	newRepo.Service = srv
	newRepo.user = u

	return &newRepo, nil
}

// CreateCalendar creates a calendar and returns its id
func (c *GoogleCalendarRepository) CreateCalendar() (string, error) {
	// calendarId := "refm50ua0bukpdmp52a84cgshk@group.gcalendar.google.com"
	newCalendar := gcalendar.Calendar{
		Summary: "Timeliness tasks",
	}
	cal, err := c.Service.Calendars.Insert(&newCalendar).Do()
	if err != nil {
		return "", err
	}

	return cal.Id, nil
}

// GetAllCalendarsOfInterest retrieves all Calendars from Google Calendar
func (c *GoogleCalendarRepository) GetAllCalendarsOfInterest() (map[string]*Calendar, error) {
	var calendars = make(map[string]*Calendar)
	calList, err := c.Service.CalendarList.List().Do()
	if err != nil {
		return calendars, err
	}

	for _, cal := range calList.Items {
		if c.user.GoogleCalendarConnection.TaskCalendar.CalendarID == cal.Id {
			continue
		}
		calendars[cal.Id] = &Calendar{CalendarID: cal.Id, Name: cal.Summary}
	}
	return calendars, err
}

// NewEvent creates a new Event in Google Calendar
func (c *GoogleCalendarRepository) NewEvent(title string, description string, blocking bool, event *Event) (*Event, error) {
	googleEvent := createGoogleEvent(title, description, blocking, event)

	createdEvent, err := c.Service.Events.Insert(c.user.GoogleCalendarConnection.TaskCalendar.CalendarID, googleEvent).Do()
	if err != nil {
		return nil, err
	}

	event.CalendarEventID = createdEvent.Id
	event.CalendarType = CalendarTypeGoogleCalendar

	return event, nil
}

// UpdateEvent updates an existing Google Calendar event
func (c *GoogleCalendarRepository) UpdateEvent(title string, description string, blocking bool, event *Event) error {
	googleEvent := createGoogleEvent(title, description, blocking, event)

	_, err := c.Service.Events.
		Update(c.user.GoogleCalendarConnection.TaskCalendar.CalendarID, event.CalendarEventID, googleEvent).Do()
	if err != nil {
		return err
	}

	return nil
}

func createGoogleEvent(title string, description string, blocking bool, event *Event) *gcalendar.Event {
	start := gcalendar.EventDateTime{
		DateTime: event.Date.Start.Format(time.RFC3339),
	}

	end := gcalendar.EventDateTime{
		DateTime: event.Date.End.Format(time.RFC3339),
	}

	transparency := "opaque"
	if !blocking {
		transparency = "transparent"
	}

	source := gcalendar.EventSource{Title: "Timeliness", Url: "https://timeliness.app"}

	googleEvent := gcalendar.Event{
		Start:        &start,
		End:          &end,
		Summary:      title,
		Description:  description,
		Transparency: transparency,
		Source:       &source,
	}

	return &googleEvent
}

// AddBusyToWindow reads times from a window and fills it with busy timeslots
func (c *GoogleCalendarRepository) AddBusyToWindow(window *TimeWindow) error {
	calList := c.user.GoogleCalendarConnection.CalendarsOfInterest
	calList = append(calList, c.user.GoogleCalendarConnection.TaskCalendar)

	var items = make([]*gcalendar.FreeBusyRequestItem, len(calList))
	for _, cal := range calList {
		items = append(items, &gcalendar.FreeBusyRequestItem{Id: cal.CalendarID})
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

// DeleteEvent deletes a single Event
func (c *GoogleCalendarRepository) DeleteEvent(event *Event) error {
	err := c.Service.Events.Delete(c.user.GoogleCalendarConnection.TaskCalendar.CalendarID, event.CalendarEventID).Do()
	if err != nil {
		return err
	}

	return nil
}
