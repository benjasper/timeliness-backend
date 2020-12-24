package calendar

import (
	"github.com/timeliness-app/timeliness-backend/pkg/users"
)

type CalendarService struct {
}

func (c *CalendarService) SuggestTimeslot(u *users.User, window *TimeWindow) (*[]Timespan, error) {
	calendarRepository, err := NewCalendarRepository(u)
	if err != nil {
		return nil, err
	}

	err = calendarRepository.AddBusyToWindow(window)
	if err != nil {
		return nil, err
	}

	free := window.ComputeFree()
	return &free, nil
}
