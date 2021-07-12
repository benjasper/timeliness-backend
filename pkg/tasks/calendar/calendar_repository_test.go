package calendar

import (
	"crypto/md5"
	"github.com/timeliness-app/timeliness-backend/pkg/users"
)

type MockCalendarRepository struct {
	Events []*Event
}

func (r *MockCalendarRepository) findEventByID(ID string) (*Event, int, error) {
	for i, event := range r.Events {
		if event.CalendarEventID != ID {
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
	event.CalendarEventID = string(md5.New().Sum(id))
	event.CalendarType = "mock-calendar"

	r.Events = append(r.Events, event)
	return event, nil
}

func (r *MockCalendarRepository) UpdateEvent(event *Event) error {
	event, i, err := r.findEventByID(event.CalendarEventID)
	if err != nil {
		return err
	}

	r.Events[i] = event

	return nil
}

func (r *MockCalendarRepository) DeleteEvent(event *Event) error {
	_, i, err := r.findEventByID(event.CalendarEventID)
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

func (r *MockCalendarRepository) SyncEvents(calendarID string, eventChannel *chan *Event, errorChannel *chan error, userChannel *chan *users.User) {
	return
}
