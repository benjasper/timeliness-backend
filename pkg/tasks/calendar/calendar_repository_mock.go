package calendar

import (
	"crypto/md5"
	"github.com/pkg/errors"
	"github.com/timeliness-app/timeliness-backend/pkg/date"
	"github.com/timeliness-app/timeliness-backend/pkg/users"
	"time"
)

// MockCalendarRepository is a calendar repository for testing
type MockCalendarRepository struct {
	Events       []*Event
	EventsToSync []*Event
	User         *users.User
}

// NewMockCalendarRepository builds a new MockCalendarRepository
func NewMockCalendarRepository(user *users.User) MockCalendarRepository {
	return MockCalendarRepository{
		User: user,
	}
}

func (r *MockCalendarRepository) findEventByID(ID string) (*Event, int, error) {
	for i, event := range r.Events {
		calendarEvent := event.CalendarEvents.FindByCalendarID(ID)
		if calendarEvent == nil {
			continue
		}

		if calendarEvent.CalendarEventID != ID {
			continue
		}
		return event, i, nil
	}

	return nil, -1, errors.New("event does not exist")
}

// TestTaskCalendarExistence creates a test calendar
func (r *MockCalendarRepository) TestTaskCalendarExistence(u *users.User) (*users.User, error) {
	return u, nil
}

// GetAllCalendarsOfInterest is not implemented yet
func (r *MockCalendarRepository) GetAllCalendarsOfInterest() (map[string]*Calendar, error) {
	panic("implement me")
}

// NewEvent adds a new event
func (r *MockCalendarRepository) NewEvent(event *Event, taskID string, title string, description string, withReminder bool) (*Event, error) {
	id := ([]byte)(event.Date.Start.String() + title)

	calendarEvent := PersistedEvent{
		CalendarEventID: string(md5.New().Sum(id)),
		CalendarType:    "mock-calendar",
		UserID:          r.User.ID,
	}

	event.CalendarEvents = append(event.CalendarEvents, calendarEvent)

	r.Events = append(r.Events, event)
	return event, nil
}

// UpdateEvent updates an existing event
func (r *MockCalendarRepository) UpdateEvent(event *Event, taskID string, title string, description string, withReminder bool) error {
	calendarEvent := event.CalendarEvents.FindByUserID(r.User.ID.Hex())

	if calendarEvent == nil {
		return errors.New("calendar event not found")
	}

	_, i, err := r.findEventByID(calendarEvent.CalendarEventID)
	if err != nil {
		return err
	}

	r.Events[i] = event

	return nil
}

// DeleteEvent deletes an Event
func (r *MockCalendarRepository) DeleteEvent(event *Event) error {
	calendarEvent := event.CalendarEvents.FindByUserID(r.User.ID.Hex())

	if calendarEvent == nil {
		return errors.New("calendar event not found")
	}

	_, i, err := r.findEventByID(calendarEvent.CalendarEventID)
	if err != nil {
		return err
	}

	r.Events = append(r.Events[:i], r.Events[i+1:]...)
	return nil
}

// AddBusyToWindow adds busy times
func (r *MockCalendarRepository) AddBusyToWindow(window *date.TimeWindow, start time.Time, end time.Time) error {
	for _, event := range r.Events {
		window.AddToBusy(event.Date)
	}

	return nil
}

// WatchCalendar is not implemented yet
func (r *MockCalendarRepository) WatchCalendar(calendarID string, user *users.User) (*users.User, error) {
	return nil, nil
}

// StopWatchingCalendar is not implemented yet
func (r *MockCalendarRepository) StopWatchingCalendar(calendarID string, user *users.User) (*users.User, error) {
	return nil, nil
}

// SyncEvents returns the events in EventsToSync
func (r *MockCalendarRepository) SyncEvents(calendarID string, user *users.User, eventChannel *chan *Event, errorChannel *chan error, userChannel *chan *users.User) {
	defer close(*eventChannel)
	defer close(*errorChannel)
	defer close(*userChannel)

	for _, event := range r.EventsToSync {
		*eventChannel <- event
	}

	*userChannel <- r.User
}
