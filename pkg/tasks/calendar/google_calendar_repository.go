package calendar

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/timeliness-app/timeliness-backend/internal/google"
	"github.com/timeliness-app/timeliness-backend/pkg/auth/encryption"
	"github.com/timeliness-app/timeliness-backend/pkg/communication"
	"github.com/timeliness-app/timeliness-backend/pkg/date"
	"github.com/timeliness-app/timeliness-backend/pkg/logger"
	"github.com/timeliness-app/timeliness-backend/pkg/users"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	"os"
	"time"

	"golang.org/x/oauth2"
	gcalendar "google.golang.org/api/calendar/v3"
)

// GoogleNotificationExpirationOffset decides how much before an expiration a sync should be renewed
const GoogleNotificationExpirationOffset = time.Hour * 24

// GoogleCalendarRepository provides function for easily editing the users google calendar
type GoogleCalendarRepository struct {
	Config     *oauth2.Config
	Logger     logger.Interface
	Service    *gcalendar.Service
	connection *users.GoogleCalendarConnection
	apiBaseURL string
	userID     primitive.ObjectID
}

// NewGoogleCalendarRepository constructs a GoogleCalendarRepository, best to use CalendarRepositoryManager for this
func NewGoogleCalendarRepository(ctx context.Context, userID primitive.ObjectID, connection *users.GoogleCalendarConnection, logger logger.Interface) (*GoogleCalendarRepository, error) {
	newRepo := GoogleCalendarRepository{}

	config, err := google.ReadGoogleConfig()
	if err != nil {
		return nil, err
	}

	newRepo.Config = config

	if connection.Token.AccessToken == "" {
		return nil, communication.ErrCalendarAuthInvalid
	}

	if connection.Token.Expiry.Before(time.Now()) {
		source := newRepo.Config.TokenSource(ctx, &connection.Token)
		newToken, err := source.Token()
		if err != nil {
			return nil, err
		}
		connection.Token = *newToken
	}

	client := newRepo.Config.Client(ctx, &connection.Token)

	srv, err := gcalendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}

	newRepo.Service = srv
	newRepo.Logger = logger
	newRepo.connection = connection
	newRepo.userID = userID

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
			return errors.Wrap(e, communication.ErrCalendarAuthInvalid.Error())
		}

		return errors.Wrap(e, "google calendar api error")
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
func (c *GoogleCalendarRepository) createCalendar() (string, error) {
	newCalendar := gcalendar.Calendar{
		Summary: "Timeliness Tasks",
	}
	cal, err := c.Service.Calendars.Insert(&newCalendar).Do()
	if err != nil {
		return "", checkForInvalidTokenError(err)
	}

	/*
		// This requires the https://www.googleapis.com/auth/calendar scope, which we don't want to include because it would give us access to all calendars

		calendarList := &gcalendar.CalendarListEntry{
			BackgroundColor: "#dbe2ff",
			ForegroundColor: "#000000",
		}

		_, err = c.Service.CalendarList.Patch(cal.Id, calendarList).ColorRgbFormat(true).Do()
		if err != nil {
			return "", checkForInvalidTokenError(err)
		}
	*/

	return cal.Id, nil
}

// TestTaskCalendarExistence checks if the task calendar still exists and creates a new one if it doesn't
func (c *GoogleCalendarRepository) TestTaskCalendarExistence(u *users.User) (*users.User, error) {
	if !c.connection.IsTaskCalendarConnection {
		return u, nil
	}

	createCalendar := false

	if c.connection.TaskCalendarID == "" {
		createCalendar = true
	} else {
		_, err := c.Service.Calendars.Get(c.connection.TaskCalendarID).Do()
		if err != nil {
			if checkForInvalidTokenError(err) == communication.ErrCalendarAuthInvalid {
				return nil, communication.ErrCalendarAuthInvalid
			}

			createCalendar = true
		}
	}

	if createCalendar {
		calendarID, err := c.createCalendar()
		if err != nil {
			return nil, err
		}

		if c.connection.TaskCalendarID != "" {
			c.connection.CalendarsOfInterest = c.connection.CalendarsOfInterest.RemoveCalendar(c.connection.TaskCalendarID)
		}

		c.connection.TaskCalendarID = calendarID

		c.connection.CalendarsOfInterest = append(c.connection.CalendarsOfInterest,
			users.GoogleCalendarSync{CalendarID: calendarID})

		for i, connection := range u.GoogleCalendarConnections {
			if connection.ID == c.connection.ID {
				u.GoogleCalendarConnections[i] = *c.connection
			}
		}
	} else {
		if !c.connection.CalendarsOfInterest.HasCalendarWithID(c.connection.TaskCalendarID) {
			c.connection.CalendarsOfInterest = append(c.connection.CalendarsOfInterest,
				users.GoogleCalendarSync{CalendarID: c.connection.TaskCalendarID})

			for i, connection := range u.GoogleCalendarConnections {
				if connection.ID == c.connection.ID {
					u.GoogleCalendarConnections[i] = *c.connection
				}
			}
		}
	}

	return u, nil
}

// GetAllCalendarsOfInterest retrieves all Calendars from Google Calendar
func (c *GoogleCalendarRepository) GetAllCalendarsOfInterest() (map[string]*Calendar, error) {
	var calendars = make(map[string]*Calendar)
	calList, err := c.Service.CalendarList.List().Do()
	if err != nil {
		return calendars, checkForInvalidTokenError(err)
	}

	for _, cal := range calList.Items {
		if c.connection.TaskCalendarID == cal.Id {
			continue
		}
		calendars[cal.Id] = &Calendar{CalendarID: cal.Id, Name: cal.Summary}
	}
	return calendars, err
}

// NewEvent creates a new Event in Google Calendar
func (c *GoogleCalendarRepository) NewEvent(event *Event, taskID string) (*Event, error) {
	googleEvent := c.createGoogleEvent(event, taskID)

	createdEvent, err := c.Service.Events.Insert(c.connection.TaskCalendarID, googleEvent).Do()
	if err != nil {
		return nil, checkForInvalidTokenError(err)
	}

	calEvent := PersistedEvent{
		CalendarEventID: createdEvent.Id,
		CalendarType:    PersistedCalendarTypeGoogleCalendar,
		UserID:          c.userID,
	}

	event.CalendarEvents = append(event.CalendarEvents, calEvent)

	return event, nil
}

// UpdateEvent updates an existing Google Calendar event
func (c *GoogleCalendarRepository) UpdateEvent(event *Event, taskID string) error {
	googleEvent := c.createGoogleEvent(event, taskID)

	calendarEvent := event.CalendarEvents.FindByUserID(c.userID.Hex())

	_, err := c.Service.Events.
		Update(c.connection.TaskCalendarID, calendarEvent.CalendarEventID, googleEvent).Do()
	if err != nil {
		return checkForInvalidTokenError(err)
	}

	return nil
}

// WatchCalendar activates notifications for
func (c *GoogleCalendarRepository) WatchCalendar(calendarID string, user *users.User) (*users.User, error) {
	channel := gcalendar.Channel{
		Id:      uuid.New().String(),
		Address: fmt.Sprintf("%s/v1/calendar/google/notifications", c.apiBaseURL),
		Token:   encryption.Encrypt(c.userID.Hex()),
		Type:    "web_hook",
	}

	index := findSyncByID(c.connection, calendarID)
	if index == -1 {
		return nil, errors.New("calendar id could not be found in calendars of interest")
	}

	if c.connection.CalendarsOfInterest[index].Expiration.After(time.Now().Add(GoogleNotificationExpirationOffset)) {
		return user, nil
	}

	if c.connection.CalendarsOfInterest[index].SyncResourceID != "" &&
		c.connection.CalendarsOfInterest[index].ChannelID != "" {
		oldChannel := gcalendar.Channel{
			Id:         c.connection.CalendarsOfInterest[index].ChannelID,
			ResourceId: c.connection.CalendarsOfInterest[index].SyncResourceID,
		}
		err := c.Service.Channels.Stop(&oldChannel).Do()
		if err != nil {
			c.Logger.Warning("Bad response on stopping a google notification channel", err)
		}
	}

	response, err := c.Service.Events.Watch(calendarID, &channel).Do()
	if err != nil {
		c.connection.CalendarsOfInterest = c.connection.CalendarsOfInterest.RemoveCalendar(calendarID)
		for i, connection := range user.GoogleCalendarConnections {
			if connection.ID == c.connection.ID {
				user.GoogleCalendarConnections[i] = *c.connection
			}
		}

		return user, err
	}

	c.connection.CalendarsOfInterest[index].SyncResourceID = response.ResourceId
	c.connection.CalendarsOfInterest[index].ChannelID = response.Id
	c.connection.CalendarsOfInterest[index].Expiration = time.Unix(0, response.Expiration*int64(time.Millisecond))

	for i, connection := range user.GoogleCalendarConnections {
		if connection.ID == c.connection.ID {
			user.GoogleCalendarConnections[i] = *c.connection
		}
	}

	return user, nil
}

// StopWatchingCalendar stops notifications for a calendar
func (c *GoogleCalendarRepository) StopWatchingCalendar(calendarID string, user *users.User) (*users.User, error) {
	index := findSyncByID(c.connection, calendarID)
	if index == -1 {
		return nil, errors.New("calendar id could not be found in calendars of interest")
	}

	if c.connection.CalendarsOfInterest[index].ChannelID == "" || c.connection.CalendarsOfInterest[index].SyncResourceID == "" {
		return user, nil
	}

	channel := gcalendar.Channel{
		Id:         c.connection.CalendarsOfInterest[index].ChannelID,
		ResourceId: c.connection.CalendarsOfInterest[index].SyncResourceID,
	}

	err := c.Service.Channels.Stop(&channel).Do()
	if err != nil {
		return nil, err
	}

	c.connection.CalendarsOfInterest[index].SyncResourceID = ""
	c.connection.CalendarsOfInterest[index].ChannelID = ""

	for i, connection := range user.GoogleCalendarConnections {
		if connection.ID == c.connection.ID {
			user.GoogleCalendarConnections[i] = *c.connection
		}
	}

	return user, nil
}

func findSyncByID(connection *users.GoogleCalendarConnection, ID string) int {
	for i, sync := range connection.CalendarsOfInterest {
		if sync.CalendarID == ID {
			return i
		}
	}

	return -1
}

func (c *GoogleCalendarRepository) googleEventToEvent(event *gcalendar.Event, loc *time.Location) (*Event, error) {
	newEvent := &Event{
		Title:       event.Summary,
		Description: event.Description,
		CalendarEvents: []PersistedEvent{
			{
				CalendarEventID: event.Id,
				CalendarType:    PersistedCalendarTypeGoogleCalendar,
				UserID:          c.userID,
			},
		},
	}

	if event.Source != nil && event.Source.Title == "Timeliness" {
		newEvent.IsOriginal = true
	}

	if event.Transparency == "" || event.Transparency == "opaque" {
		newEvent.Blocking = true
	}

	if event.Status == "cancelled" {
		newEvent.Deleted = true
		return newEvent, nil
	}

	start := time.Time{}
	end := time.Time{}
	var err error

	if event.Start.Date != "" && event.End.Date != "" && loc != nil {
		start, err = time.ParseInLocation("2006-01-02", event.Start.Date, loc)
		if err != nil {
			return nil, err
		}

		end, err = time.ParseInLocation("2006-01-02", event.End.Date, loc)
		if err != nil {
			return nil, err
		}
	} else if event.Start.DateTime != "" && event.End.DateTime != "" {
		start, err = time.Parse(time.RFC3339, event.Start.DateTime)
		if err != nil {
			return nil, err
		}

		end, err = time.Parse(time.RFC3339, event.End.DateTime)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, errors.New("neither date with location or datetime was filled in google event")
	}

	newEvent.Date = date.Timespan{
		Start: start,
		End:   end,
	}

	return newEvent, nil
}

// SyncEvents syncs the calendar events of a single calendar
func (c *GoogleCalendarRepository) SyncEvents(calendarID string, user *users.User,
	eventChannel *chan *Event,
	errorChannel *chan error,
	userChannel *chan *users.User) {

	now := time.Now()
	request := c.Service.Events.List(calendarID).SingleEvents(true)

	syncIndex := findSyncByID(c.connection, calendarID)

	if c.connection.CalendarsOfInterest[syncIndex].SyncToken != "" && c.connection.CalendarsOfInterest[syncIndex].Expiration.After(now) {
		request = request.SyncToken(c.connection.CalendarsOfInterest[syncIndex].SyncToken)
	} else {
		request = request.TimeMin(now.Format(time.RFC3339))
		request = request.TimeMax(now.Add(time.Hour * 24 * 31 * 6).Format(time.RFC3339)) // 6 months from now
	}

	defer close(*eventChannel)
	defer close(*errorChannel)
	defer close(*userChannel)

	for {
		response, err := request.Do()
		if err != nil {
			googleError, ok := err.(*googleapi.Error)
			if ok && googleError.Code == 410 {
				c.connection.CalendarsOfInterest[syncIndex].SyncToken = ""

				for i, connection := range user.GoogleCalendarConnections {
					if connection.ID == c.connection.ID {
						user.GoogleCalendarConnections[i] = *c.connection
					}
				}

				*userChannel <- user
				return
			}

			*errorChannel <- errors.WithStack(err)
			return
		}

		location, _ := time.LoadLocation(response.TimeZone)

		for _, item := range response.Items {
			if item.Status == "cancelled" {
				// TODO: Figure out what to do with deleted events
				*eventChannel <- &Event{
					CalendarEvents: []PersistedEvent{
						{
							CalendarEventID: item.Id,
							CalendarType:    PersistedCalendarTypeGoogleCalendar,
							UserID:          c.userID,
						},
					},
					Deleted: true,
				}
				continue
			}

			event, err := c.googleEventToEvent(item, location)
			if err != nil {
				*errorChannel <- errors.WithStack(err)
				return
			}

			*eventChannel <- event
		}

		if response.NextSyncToken != "" {
			c.connection.CalendarsOfInterest[syncIndex].SyncToken = response.NextSyncToken
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

func (c *GoogleCalendarRepository) createGoogleEvent(event *Event, taskID string) *gcalendar.Event {
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

	frontendURL := os.Getenv("FRONTEND_BASE_URL")

	source := gcalendar.EventSource{Title: "Timeliness", Url: fmt.Sprintf("%s/dashboard/task/%s", frontendURL, taskID)}

	googleEvent := gcalendar.Event{
		Start:        &start,
		End:          &end,
		Summary:      event.Title,
		Description:  event.Description,
		Transparency: transparency,
		Source:       &source,
		Reminders: &gcalendar.EventReminders{
			UseDefault: true,
		},
	}

	return &googleEvent
}

// AddBusyToWindow reads times from a window and fills it with busy timeslots
func (c *GoogleCalendarRepository) AddBusyToWindow(window *date.TimeWindow, start time.Time, end time.Time) error {
	calList := c.connection.CalendarsOfInterest

	var items = make([]*gcalendar.FreeBusyRequestItem, len(calList))
	for _, cal := range calList {
		items = append(items, &gcalendar.FreeBusyRequestItem{Id: cal.CalendarID})
	}

	response, err := c.Service.Freebusy.Query(&gcalendar.FreeBusyRequest{
		TimeMin: start.Format(time.RFC3339),
		TimeMax: end.Format(time.RFC3339),
		Items:   items}).Do()
	if err != nil {
		return checkForInvalidTokenError(err)
	}

	for _, v := range response.Calendars {
		for _, period := range v.Busy {
			slotStart, err := time.Parse(time.RFC3339, period.Start)
			if err != nil {
				return err
			}

			slotEnd, err := time.Parse(time.RFC3339, period.End)
			if err != nil {
				return err
			}

			window.AddToBusy(date.Timespan{Start: slotStart.UTC(), End: slotEnd.UTC()})
		}
	}

	return nil
}

// DeleteEvent deletes a single Event
func (c *GoogleCalendarRepository) DeleteEvent(event *Event) error {
	calendarEvent := event.CalendarEvents.FindByUserID(c.userID.Hex())
	if calendarEvent == nil {
		return fmt.Errorf("persisted calendar event for user %s could not be found while deleting event", c.userID.Hex())
	}

	err := c.Service.Events.Delete(c.connection.TaskCalendarID, calendarEvent.CalendarEventID).Do()
	if err != nil {
		if checkForIsGone(err) == nil {
			return nil
		}

		return checkForInvalidTokenError(err)
	}

	return nil
}
