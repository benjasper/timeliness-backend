package calendar

import (
	"github.com/timeliness-app/timeliness-backend/pkg/date"
	"github.com/timeliness-app/timeliness-backend/pkg/users"
)

// RepositoryInterface is an interface for every calendar implementation e.g. Google Calendar, Microsoft Calendar,...
type RepositoryInterface interface {
	CreateCalendar() (string, error)
	GetAllCalendarsOfInterest() (map[string]*Calendar, error)
	NewEvent(event *Event) (*Event, error)

	// UpdateEvent updates an event in a calendar, make sure to persist changes to the event before calling this method
	UpdateEvent(event *Event) error

	// DeleteEvent deletes an event in a calendar, make sure to persist the deletion of the event before calling this method
	DeleteEvent(event *Event) error
	AddBusyToWindow(window *date.TimeWindow) error
	WatchCalendar(calendarID string, user *users.User) (*users.User, error)
	StopWatchingCalendar(calendarID string, user *users.User) (*users.User, error)
	SyncEvents(calendarID string, user *users.User, eventChannel *chan *Event, errorChannel *chan error, userChannel *chan *users.User)
}
