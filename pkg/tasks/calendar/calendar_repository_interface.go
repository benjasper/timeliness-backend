package calendar

import (
	"github.com/pkg/errors"
	"github.com/timeliness-app/timeliness-backend/pkg/date"
	"github.com/timeliness-app/timeliness-backend/pkg/users"
	"time"
)

// ErrNonSyncable is an error that is returned when a calendar doesn't support syncing
var ErrNonSyncable = errors.New("non_syncable_calendar")

// RepositoryInterface is an interface for every calendar implementation e.g. Google Calendar, Microsoft Calendar,...
type RepositoryInterface interface {
	GetAllCalendarsOfInterest() (map[string]*Calendar, error)
	NewEvent(event *Event, taskID string, title string, description string, withReminder bool) (*Event, error)
	TestTaskCalendarExistence(*users.User) (*users.User, error)

	// UpdateEvent updates an event in a calendar, make sure to persist changes to the event before calling this method
	UpdateEvent(event *Event, taskID string, title string, description string, withReminder bool) error

	// DeleteEvent deletes an event in a calendar, make sure to persist the deletion of the event before calling this method
	DeleteEvent(event *Event) error
	AddBusyToWindow(window *date.TimeWindow, start time.Time, end time.Time) error
	WatchCalendar(calendarID string, user *users.User) (*users.User, error)
	StopWatchingCalendar(calendarID string, user *users.User) (*users.User, error)
	SyncEvents(calendarID string, user *users.User, eventChannel *chan *Event, errorChannel *chan error, userChannel *chan *users.User)
}
