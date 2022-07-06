package auth

import "errors"

// Middleware errors.
var (
	ErrUnauthorized = errors.New("unauthorized")
)

// Provider errors.
var (
	ErrInvalidToken = errors.New("failed to verify token")
)
