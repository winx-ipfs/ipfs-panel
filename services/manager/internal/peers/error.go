package peers

import "errors"

// OpError is a stable peers error code. The HTTP layer maps Code to a status.
type OpError struct {
	Code string
}

func (e *OpError) Error() string {
	if e == nil {
		return ""
	}
	return e.Code
}

// CodeOf returns the stable code, or "" when err is not an OpError.
func CodeOf(err error) string {
	var op *OpError
	if errors.As(err, &op) && op != nil {
		return op.Code
	}
	return ""
}

func invalidAddress() error {
	return &OpError{Code: "invalid_address"}
}
