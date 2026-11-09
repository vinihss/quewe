package apperr

import "errors"

type Code string

const (
	CodeValidatestion        Code = "validatestion_error"
	CodeNotFound          Code = "not_found"
	CodeConflict          Code = "conflict"
	CodeNotSupported      Code = "not_supported"
	CodeInternal          Code = "internal"
	CodeInvalidTransition Code = "invalid_transition"
	CodeLeaseExpired      Code = "lease_expired"
	CodeCapabilityNotAuth Code = "capability_not_authorized"
)

type Error struct {
	Code    Code
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return string(e.Code)
}

func (e *Error) Unwrap() error { return e.Err }

func New(code Code, msg string) *Error             { return &Error{Code: code, Message: msg} }
func Wrap(code Code, msg string, err error) *Error { return &Error{Code: code, Message: msg, Err: err} }

func Validatestion(msg string) *Error   { return New(CodeValidatestion, msg) }
func NotFound(msg string) *Error     { return New(CodeNotFound, msg) }
func Conflict(msg string) *Error     { return New(CodeConflict, msg) }
func NotSupported(msg string) *Error { return New(CodeNotSupported, msg) }

func As(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

func IsCode(err error, code Code) bool {
	e, ok := As(err)
	return ok && e.Code == code
}
