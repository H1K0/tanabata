package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"tanabata/backend/internal/domain"
)

// errorBody is the JSON shape returned for all error responses.
type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func respondJSON(c *gin.Context, status int, data any) {
	c.JSON(status, data)
}

// respondError maps a domain error to the appropriate HTTP status and writes
// a JSON error body. Unknown errors become 500.
func respondError(c *gin.Context, err error) {
	var de *domain.DomainError
	if errors.As(err, &de) {
		c.JSON(domainStatus(de), errorBody{Code: de.Code(), Message: de.Error()})
		return
	}
	c.JSON(http.StatusInternalServerError, errorBody{
		Code:    "internal_error",
		Message: "internal server error",
	})
}

// domainStatus maps a DomainError sentinel to its HTTP status code per the
// error mapping table in docs/GO_PROJECT_STRUCTURE.md.
func domainStatus(de *domain.DomainError) int {
	switch de {
	case domain.ErrNotFound:
		return http.StatusNotFound
	case domain.ErrForbidden:
		return http.StatusForbidden
	case domain.ErrUnauthorized:
		return http.StatusUnauthorized
	case domain.ErrConflict:
		return http.StatusConflict
	case domain.ErrValidation:
		return http.StatusBadRequest
	case domain.ErrUnsupportedMIME:
		return http.StatusUnsupportedMediaType
	default:
		return http.StatusInternalServerError
	}
}
