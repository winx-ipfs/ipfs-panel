package logs

import "errors"

// Error is a refusal from the Kubo log client or from query parsing.
type Error struct {
	Code   string
	Detail string
	State  string
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.Code
}

// CodeOf returns the code of an Error, or "" when err is something else.
func CodeOf(err error) string {
	var op *Error
	if errors.As(err, &op) {
		return op.Code
	}
	return ""
}
