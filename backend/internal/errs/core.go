package errs

import (
	"errors"
	"net/http"
)

// Code identifies an error category within a domain.
type Code string

// httpStatuses maps error codes to their HTTP status.
var httpStatuses = map[Code]int{}

// RegisterHTTPStatus associates a Code with its HTTP status at init time.
func RegisterHTTPStatus(code Code, status int) Code {
	httpStatuses[code] = status
	return code
}

// HTTPStatus returns the HTTP status for a code, defaulting to 500.
func (c Code) HTTPStatus() int {
	if status, ok := httpStatuses[c]; ok {
		return status
	}
	return http.StatusInternalServerError
}

// Error is the structured error type used throughout the application.
type Error struct {
	Code    Code
	Message string
	Err     error
	Fields  map[string]string
	Details any
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return string(e.Code)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// HTTPStatus returns the HTTP status for this error's code.
func (e *Error) HTTPStatus() int {
	return e.Code.HTTPStatus()
}

// New creates an Error with a code and message.
func New(code Code, message string) *Error {
	return &Error{Code: code, Message: message}
}

// Wrap creates an Error wrapping an underlying error.
func Wrap(code Code, message string, err error) *Error {
	return &Error{Code: code, Message: message, Err: err}
}

// WithFields attaches field-level validation errors to an existing error.
func WithFields(err error, fields map[string]string) error {
	if len(fields) == 0 {
		return err
	}
	var typed *Error
	if errors.As(err, &typed) {
		if typed.Fields == nil {
			typed.Fields = map[string]string{}
		}
		for k, v := range fields {
			typed.Fields[k] = v
		}
		return typed
	}
	return &Error{Code: CodeValidationFields, Message: "validation failed", Err: err, Fields: fields}
}

// WithDetails attaches additional context to an existing error.
func WithDetails(err error, details any) error {
	if details == nil {
		return err
	}
	var typed *Error
	if errors.As(err, &typed) {
		typed.Details = details
		return typed
	}
	return &Error{Code: CodeInternal, Message: "unexpected error", Err: err, Details: details}
}

// From extracts an *Error from an error chain, if present.
func From(err error) (*Error, bool) {
	var typed *Error
	if errors.As(err, &typed) {
		return typed, true
	}
	return nil, false
}
