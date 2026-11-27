package peering

import "errors"

// Error is a stable peering failure. Status is the HTTP status to write.
type Error struct {
	Code   string
	Status int
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.Code
}

func code(status int, name string) *Error {
	return &Error{Code: name, Status: status}
}

// CodeOf returns the stable code, or "" when err is not an Error.
func CodeOf(err error) string {
	var pe *Error
	if errors.As(err, &pe) && pe != nil {
		return pe.Code
	}
	return ""
}
