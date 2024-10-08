package tasks

// Filter is a model for the rest api filter
type Filter struct {
	Field    string
	Value    interface{}
	Operator string
}

// ConcatFilter is a model for the rest api where multiple filters are concatenated
type ConcatFilter struct {
	Filters  []Filter
	Operator string
}
