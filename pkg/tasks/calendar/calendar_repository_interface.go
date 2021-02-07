package calendar

// RepositoryInterface is an interface for every calendar implementation e.g. Google Calendar, Microsoft Calendar,...
type RepositoryInterface interface {
	CreateCalendar() (string, error)
	GetAllCalendarsOfInterest() (map[string]*Calendar, error)
	NewEvent(title string, description string, blocking bool, event *Event) (*Event, error)
	UpdateEvent(title string, description string, blocking bool, event *Event) error
	AddBusyToWindow(window *TimeWindow) error
}
