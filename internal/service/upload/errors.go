package upload

// Error is an expected upload failure carrying an HTTP status + machine code so
// the controller can map it directly (via errors.As).
type Error struct {
	Code    string
	Status  int
	Message string
}

func (e *Error) Error() string { return e.Message }

func newErr(code string, status int, msg string) *Error {
	return &Error{Code: code, Status: status, Message: msg}
}
