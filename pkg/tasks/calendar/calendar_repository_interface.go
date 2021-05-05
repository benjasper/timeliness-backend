package calendar

import "github.com/timeliness-app/timeliness-backend/pkg/users"

// RepositoryInterface is an interface for every calendar implementation e.g. Google Calendar, Microsoft Calendar,...
type RepositoryInterface interface {
	CreateCalendar() (string, error)
	GetAllCalendarsOfInterest() (map[string]*Calendar, error)
	NewEvent(event *Event) (*Event, error)
	UpdateEvent(event *Event) error
	DeleteEvent(event *Event) error
	AddBusyToWindow(window *TimeWindow) error
	WatchCalendar(calendarID string) (*users.User, error)
	SyncEvents(calendarID string, eventChannel *chan *Event, errorChannel *chan error, userChannel *chan *users.User)
}
