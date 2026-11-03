// Package pins calls Kubo's pin and add RPC on a loopback address.
// Pin adds that fetch from the network run as tracked jobs. This package
// does not hold an HTTP request open for that fetch.
package pins

import "errors"

// OpError is a stable pins error code. The HTTP layer maps Code to a status.
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

func invalidCID() error  { return &OpError{Code: "invalid_cid"} }
func invalidName() error { return &OpError{Code: "invalid_name"} }
