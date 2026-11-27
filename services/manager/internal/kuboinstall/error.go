package kuboinstall

// Codes the HTTP layer and the page both understand. A job's error field
// is one of these, never a filesystem path or a Kubo log line.
const (
	CodeOffline      = "offline"
	CodeVerify       = "verify_failed"
	CodeUnsupported  = "unsupported_platform"
	CodeExternal     = "external_binary"
	CodeAlready      = "already_installed"
	CodeNotInstalled = "not_installed"
	CodeUpToDate     = "up_to_date"
	CodeBusy         = "busy"
	CodeStop         = "stop_failed"
	CodeMigrate      = "migrate_failed"
	CodeStart        = "start_failed"
	CodeRollback     = "rollback_failed"
	CodeNotFound     = "not_found"
)

// Error is a stable code. The string form is the code alone so a log line
// cannot grow a path by formatting the error.
type Error struct {
	Code string
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.Code
}

func errCode(code string) *Error {
	return &Error{Code: code}
}
