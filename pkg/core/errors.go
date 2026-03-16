package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

var (
	// Transport errors
	ErrVarMissing = errors.New("variable missing")
	ErrVarType    = errors.New("invalid variable type")

	// Auth errors
	ErrUnauthorized = errors.New("unauthorized")           // Middleware error
	ErrInvalidToken = errors.New("failed to verify token") // Provider error

	// Storage errors
	ErrObjectAlreadyExists = errors.New("object already exists")
)

type ObjectNotFoundError struct {
	key string
}

func (p ObjectNotFoundError) Error() string {
	return fmt.Sprintf("failed to locate object: %s", p.key)
}

func NewObjectNotFoundError(key string) *ObjectNotFoundError {
	return &ObjectNotFoundError{
		key: key,
	}
}

type ProviderError struct {
	Reason     string
	Provider   *Provider
	StatusCode int
}

func (p ProviderError) Error() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s: ", p.Reason))
	if p.Provider.Hostname != "" {
		sb.WriteString(fmt.Sprintf("hostname=%s, ", p.Provider.Hostname))
	}
	if p.Provider.Namespace != "" {
		sb.WriteString(fmt.Sprintf("namespace=%s, ", p.Provider.Namespace))
	}
	if p.Provider.Name != "" {
		sb.WriteString(fmt.Sprintf("name=%s, ", p.Provider.Name))
	}
	if p.Provider.Version != "" {
		sb.WriteString(fmt.Sprintf("version=%s, ", p.Provider.Version))
	}
	if p.Provider.OS != "" {
		sb.WriteString(fmt.Sprintf("os=%s, ", p.Provider.OS))
	}
	if p.Provider.Arch != "" {
		sb.WriteString(fmt.Sprintf("arch=%s, ", p.Provider.Arch))
	}
	message := sb.String()
	message = strings.TrimSuffix(message, ", ")
	return message
}

// GenericError returns the HTTP status code for module-agnostic boring-registry errors
func GenericError(err error) int {
	if errors.Is(err, ErrVarMissing) {
		return http.StatusBadRequest
	} else if errors.Is(err, ErrInvalidToken) || errors.Is(err, ErrUnauthorized) {
		return http.StatusUnauthorized
	} else if errors.Is(err, ErrObjectAlreadyExists) {
		return http.StatusConflict
	}

	// Default error
	return http.StatusInternalServerError
}

// HandleErrorResponse handles the HTTP response for errors
func HandleErrorResponse(err error, w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	_ = json.NewEncoder(w).Encode(struct {
		Errors []string `json:"errors"`
	}{
		Errors: []string{
			err.Error(),
		},
	})
}
