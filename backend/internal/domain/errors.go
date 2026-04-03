package domain

import "errors"

// Sentinel domain errors. Handlers map these to HTTP status codes.
var (
	ErrNotFound        = errors.New("not found")
	ErrForbidden       = errors.New("forbidden")
	ErrUnauthorized    = errors.New("unauthorized")
	ErrConflict        = errors.New("conflict")
	ErrValidation      = errors.New("validation error")
	ErrUnsupportedMIME = errors.New("unsupported MIME type")
)
