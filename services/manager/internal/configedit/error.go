package configedit

import "errors"

// Error is a stable config-editor failure. Status is the HTTP status the
// handler should write. Detail is optional and is safe to show: it is never
// the document and never a secret.
type Error struct {
	Code   string
	Status int
	Detail string
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

func codeDetail(status int, name, detail string) *Error {
	return &Error{Code: name, Status: status, Detail: detail}
}

// CodeOf returns the stable code, or "" when err is not an Error.
func CodeOf(err error) string {
	var edit *Error
	if errors.As(err, &edit) && edit != nil {
		return edit.Code
	}
	return ""
}
