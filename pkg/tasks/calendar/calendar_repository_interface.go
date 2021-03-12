package calendar

// RepositoryInterface is an interface for every calendar implementation e.g. Google Calendar, Microsoft Calendar,...
type RepositoryInterface interface {
	CreateCalendar() (string, error)
	GetAllCalendarsOfInterest() (map[string]*Calendar, error)
	NewEvent(event *Event) (*Event, error)
	UpdateEvent(event *Event) error
	DeleteEvent(event *Event) error
	AddBusyToWindow(window *TimeWindow) error
}
