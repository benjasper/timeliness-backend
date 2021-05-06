package calendar

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/timeliness-app/timeliness-backend/internal/google"
	"github.com/timeliness-app/timeliness-backend/pkg/auth/encryption"
	"github.com/timeliness-app/timeliness-backend/pkg/communication"
	"github.com/timeliness-app/timeliness-backend/pkg/logger"
	"github.com/timeliness-app/timeliness-backend/pkg/users"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	"os"
	"time"

	"golang.org/x/oauth2"
	gcalendar "google.golang.org/api/calendar/v3"
)

// GoogleCalendarRepository provides function for easily editing the users google calendar
type GoogleCalendarRepository struct {
	Config     *oauth2.Config
	Logger     logger.Interface
	Service    *gcalendar.Service
	ctx        context.Context
	user       *users.User
	apiBaseURL string
}

// NewGoogleCalendarRepository constructs a GoogleCalendarRepository
func NewGoogleCalendarRepository(ctx context.Context, u *users.User) (*GoogleCalendarRepository, error) {
	newRepo := GoogleCalendarRepository{}

	newRepo.ctx = ctx

	config, _ := google.ReadGoogleConfig()
	newRepo.Config = config

	if u.GoogleCalendarConnection.Token.AccessToken == "" {
		return nil, communication.ErrCalendarAuthInvalid
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

	newRepo.apiBaseURL = "http://localhost"
	envBaseURL, ok := os.LookupEnv("BASE_URL")
	if ok {
		newRepo.apiBaseURL = envBaseURL
	}

	return &newRepo, nil
}

func checkForInvalidTokenError(err error) error {
	if e, ok := err.(*googleapi.Error); ok {
		if e.Code == 401 {
			return communication.ErrCalendarAuthInvalid
		}
	}

	return err
}

func checkForIsGone(err error) error {
	if e, ok := err.(*googleapi.Error); ok {
		if e.Code == 410 {
			return nil
		}
	}

	return err
}

// CreateCalendar creates a calendar and returns its id
func (c *GoogleCalendarRepository) CreateCalendar() (string, error) {
	// calendarId := "refm50ua0bukpdmp52a84cgshk@group.gcalendar.google.com"
	newCalendar := gcalendar.Calendar{
		Summary: "Timeliness tasks",
	}
	cal, err := c.Service.Calendars.Insert(&newCalendar).Do()
	if err != nil {
		return "", checkForInvalidTokenError(err)
	}

	return cal.Id, nil
}

// GetAllCalendarsOfInterest retrieves all Calendars from Google Calendar
func (c *GoogleCalendarRepository) GetAllCalendarsOfInterest() (map[string]*Calendar, error) {
	var calendars = make(map[string]*Calendar)
	calList, err := c.Service.CalendarList.List().Do()
	if err != nil {
		return calendars, checkForInvalidTokenError(err)
	}

	for _, cal := range calList.Items {
		if c.user.GoogleCalendarConnection.TaskCalendarID == cal.Id {
			continue
		}
		calendars[cal.Id] = &Calendar{CalendarID: cal.Id, Name: cal.Summary}
	}
	return calendars, err
}

// NewEvent creates a new Event in Google Calendar
func (c *GoogleCalendarRepository) NewEvent(event *Event) (*Event, error) {
	googleEvent := createGoogleEvent(event)

	createdEvent, err := c.Service.Events.Insert(c.user.GoogleCalendarConnection.TaskCalendarID, googleEvent).Do()
	if err != nil {
		return nil, checkForInvalidTokenError(err)
	}

	event.CalendarEventID = createdEvent.Id
	event.CalendarType = CalendarTypeGoogleCalendar

	return event, nil
}

// UpdateEvent updates an existing Google Calendar event
func (c *GoogleCalendarRepository) UpdateEvent(event *Event) error {
	googleEvent := createGoogleEvent(event)

	_, err := c.Service.Events.
		Update(c.user.GoogleCalendarConnection.TaskCalendarID, event.CalendarEventID, googleEvent).Do()
	if err != nil {
		return checkForInvalidTokenError(err)
	}

	return nil
}

// WatchCalendar activates notifications for
func (c *GoogleCalendarRepository) WatchCalendar(calendarID string, user *users.User) (*users.User, error) {
	expiration := time.Now().Add(time.Hour * 336)
	channel := gcalendar.Channel{
		Id:         uuid.New().String(),
		Address:    fmt.Sprintf("%s/v1/calendar/google/notifications", c.apiBaseURL),
		Expiration: expiration.Unix(),
		Token:      encryption.Encrypt(user.ID.Hex()),
	}

	index := findSyncByID(user.GoogleCalendarConnection, calendarID)
	if index == -1 {
		return nil, errors.New("calendar id could not be found in calendars of interest")
	}

	// TODO: Stop notifications if we are close to end
	// TODO: rerequest watch if that is the case
	if user.GoogleCalendarConnection.CalendarsOfInterest[index].Expiration.After(time.Now()) {
		return user, nil
	}

	response, err := c.Service.Events.Watch(calendarID, &channel).Do()
	if err != nil {
		return nil, err
	}

	user.GoogleCalendarConnection.CalendarsOfInterest[index].SyncResourceID = response.ResourceId
	user.GoogleCalendarConnection.CalendarsOfInterest[index].Expiration = expiration

	return user, nil
}

func findSyncByID(connection users.GoogleCalendarConnection, ID string) int {
	for i, sync := range connection.CalendarsOfInterest {
		if sync.CalendarID == ID {
			return i
		}
	}

	return -1
}

func googleEventToEvent(event *gcalendar.Event) (*Event, error) {
	blocking := false
	if event.Transparency == "opaque" {
		blocking = true
	}

	start, err := time.Parse(time.RFC3339, event.Start.DateTime)
	if err != nil {
		return nil, err
	}

	end, err := time.Parse(time.RFC3339, event.End.DateTime)
	if err != nil {
		return nil, err
	}

	return &Event{
		CalendarEventID: event.Id,
		Title:           event.Summary,
		Description:     event.Description,
		Blocking:        blocking,
		CalendarType:    CalendarTypeGoogleCalendar,
		Date: Timespan{
			Start: start,
			End:   end,
		},
	}, nil
}

// SyncEvents syncs the calendar events of a single calendar
func (c *GoogleCalendarRepository) SyncEvents(calendarID string,
	eventChannel *chan *Event,
	errorChannel *chan error,
	userChannel *chan *users.User) {

	now := time.Now().Format(time.RFC3339)
	request := c.Service.Events.List(calendarID).SingleEvents(true)

	user := c.user
	syncIndex := findSyncByID(user.GoogleCalendarConnection, calendarID)

	if user.GoogleCalendarConnection.CalendarsOfInterest[syncIndex].SyncToken != "" {
		request = request.SyncToken(user.GoogleCalendarConnection.CalendarsOfInterest[syncIndex].SyncToken)
	} else {
		request = request.TimeMin(now)
		// TODO max time restriction
	}

	defer close(*eventChannel)
	defer close(*errorChannel)
	defer close(*userChannel)

	for {
		response, err := request.Do()
		if err != nil {
			*errorChannel <- err
			return
		}

		for _, item := range response.Items {
			event, err := googleEventToEvent(item)
			if err != nil {
				*errorChannel <- err
				return
			}

			*eventChannel <- event
		}

		if response.NextSyncToken != "" {
			user.GoogleCalendarConnection.CalendarsOfInterest[syncIndex].SyncToken = response.NextSyncToken
			break
		}

		if len(response.Items) == 0 {
			break
		}

		if response.NextPageToken == "" {
			*errorChannel <- errors.New("neither sync token nor page token found")
			return
		}

		request = request.PageToken(response.NextPageToken)
	}

	*userChannel <- user
}

func createGoogleEvent(event *Event) *gcalendar.Event {
	start := gcalendar.EventDateTime{
		DateTime: event.Date.Start.Format(time.RFC3339),
	}

	end := gcalendar.EventDateTime{
		DateTime: event.Date.End.Format(time.RFC3339),
	}

	transparency := "opaque"
	if !event.Blocking {
		transparency = "transparent"
	}

	source := gcalendar.EventSource{Title: "Timeliness", Url: "https://timeliness.app"}

	googleEvent := gcalendar.Event{
		Start:        &start,
		End:          &end,
		Summary:      event.Title,
		Description:  event.Description,
		Transparency: transparency,
		Source:       &source,
	}

	return &googleEvent
}

// AddBusyToWindow reads times from a window and fills it with busy timeslots
func (c *GoogleCalendarRepository) AddBusyToWindow(window *TimeWindow) error {
	calList := c.user.GoogleCalendarConnection.CalendarsOfInterest

	var items = make([]*gcalendar.FreeBusyRequestItem, len(calList))
	for _, cal := range calList {
		items = append(items, &gcalendar.FreeBusyRequestItem{Id: cal.CalendarID})
	}

	response, err := c.Service.Freebusy.Query(&gcalendar.FreeBusyRequest{
		TimeMin: window.Start.Format(time.RFC3339),
		TimeMax: window.End.Format(time.RFC3339),
		Items:   items}).Do()
	if err != nil {
		return checkForInvalidTokenError(err)
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

			window.AddToBusy(Timespan{Start: start.UTC(), End: end.UTC()})
		}
	}

	return nil
}

// DeleteEvent deletes a single Event
func (c *GoogleCalendarRepository) DeleteEvent(event *Event) error {
	err := c.Service.Events.Delete(c.user.GoogleCalendarConnection.TaskCalendarID, event.CalendarEventID).Do()
	if err != nil {
		if checkForIsGone(err) == nil {
			return nil
		}

		return checkForInvalidTokenError(err)
	}

	return nil
}
