package app

import "errors"

const (
	ExitSuccess    = 0
	ExitGeneral    = 1
	ExitUsage      = 2
	ExitNotFound   = 3
	ExitAuth       = 4
	ExitDependency = 5
	ExitUpstream   = 6
)

type Error struct {
	Kind    string
	Message string
	Exit    int
	Fields  map[string]any
	Err     error
}

func (e *Error) Error() string {
	return e.Message
}

func (e *Error) Unwrap() error {
	return e.Err
}

func New(kind, message string, exit int, fields map[string]any, err error) *Error {
	return &Error{
		Kind:    kind,
		Message: message,
		Exit:    exit,
		Fields:  fields,
		Err:     err,
	}
}

func Extract(err error) (*Error, bool) {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr, true
	}
	return nil, false
}
