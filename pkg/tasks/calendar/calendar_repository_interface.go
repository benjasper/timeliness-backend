package calendar

type RepositoryInterface interface {
	CreateCalendar() (string, error)
	AddBusyToWindow(window *TimeWindow) error
}
