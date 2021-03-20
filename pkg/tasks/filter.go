package tasks

// Filter is a model for the rest api filter
type Filter struct {
	Field    string
	Value    interface{}
	Operator string
}
