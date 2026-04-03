package domain

// DomainError is a typed domain error with a stable machine-readable code.
// Handlers map these codes to HTTP status codes.
type DomainError struct {
	code    string
	message string
}

func (e *DomainError) Error() string { return e.message }
func (e *DomainError) Code() string  { return e.code }

// Sentinel domain errors. Use errors.Is(err, domain.ErrNotFound) for matching.
var (
	ErrNotFound        = &DomainError{"not_found", "not found"}
	ErrForbidden       = &DomainError{"forbidden", "forbidden"}
	ErrUnauthorized    = &DomainError{"unauthorized", "unauthorized"}
	ErrConflict        = &DomainError{"conflict", "conflict"}
	ErrValidation      = &DomainError{"validation_error", "validation error"}
	ErrUnsupportedMIME = &DomainError{"unsupported_mime", "unsupported MIME type"}
)
