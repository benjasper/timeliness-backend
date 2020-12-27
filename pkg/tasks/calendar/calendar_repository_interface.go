package calendar

import "github.com/timeliness-app/timeliness-backend/pkg/users"

// RepositoryInterface is an interface for every calendar implementation e.g. Google Calendar, Microsoft Calendar,...
type RepositoryInterface interface {
	CreateCalendar() (string, error)
	NewEvent(title string, description string, blocking bool, event *Event, u *users.User) (*Event, error)
	UpdateEvent(title string, description string, blocking bool, event *Event, u *users.User) error
	AddBusyToWindow(window *TimeWindow) error
}
