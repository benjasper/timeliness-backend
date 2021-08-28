package calendar

import (
	"crypto/md5"
	"errors"
	"github.com/timeliness-app/timeliness-backend/pkg/users"
)

type MockCalendarRepository struct {
	Events       []*Event
	EventsToSync []*Event
	User         *users.User
}

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

	return nil, -1, nil
}

func (r *MockCalendarRepository) CreateCalendar() (string, error) {
	return "test", nil
}

func (r *MockCalendarRepository) GetAllCalendarsOfInterest() (map[string]*Calendar, error) {
	panic("implement me")
}

func (r *MockCalendarRepository) NewEvent(event *Event) (*Event, error) {
	id := ([]byte)(event.Date.Start.String() + event.Title)

	calendarEvent := CalendarEvent{
		CalendarEventID: string(md5.New().Sum(id)),
		CalendarType:    "mock-calendar",
		UserID:          r.User.ID,
	}

	event.CalendarEvents = append(event.CalendarEvents, calendarEvent)

	r.Events = append(r.Events, event)
	return event, nil
}

func (r *MockCalendarRepository) UpdateEvent(event *Event) error {
	calendarEvent := event.CalendarEvents.FindByUserID(r.User.ID.Hex())

	if calendarEvent == nil {
		return errors.New("calendar event not found")
	}

	event, i, err := r.findEventByID(calendarEvent.CalendarEventID)
	if err != nil {
		return err
	}

	r.Events[i] = event

	return nil
}

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

func (r *MockCalendarRepository) AddBusyToWindow(window *TimeWindow) error {
	for _, event := range r.Events {
		window.AddToBusy(event.Date)
	}

	return nil
}

func (r *MockCalendarRepository) WatchCalendar(calendarID string, user *users.User) (*users.User, error) {
	return nil, nil
}

func (r *MockCalendarRepository) SyncEvents(calendarID string, user *users.User, eventChannel *chan *Event, errorChannel *chan error, userChannel *chan *users.User) {
	defer close(*eventChannel)
	defer close(*errorChannel)
	defer close(*userChannel)

	for _, event := range r.EventsToSync {
		*eventChannel <- event
	}

	*userChannel <- r.User
}
