package common

import "fmt"

// NotFoundError indicates a resource was not found.
type NotFoundError struct {
	Resource string
	ID       string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s with id '%s' not found", e.Resource, e.ID)
}

// NewNotFoundError creates a new NotFoundError.
func NewNotFoundError(resource, id string) *NotFoundError {
	return &NotFoundError{Resource: resource, ID: id}
}

// ValidationError indicates invalid input data.
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// NewValidationError creates a new ValidationError.
func NewValidationError(message string) *ValidationError {
	return &ValidationError{Message: message}
}

// UnauthorizedError indicates missing or invalid authentication.
type UnauthorizedError struct {
	Message string
}

func (e *UnauthorizedError) Error() string {
	if e.Message == "" {
		return "unauthorized"
	}
	return e.Message
}

// NewUnauthorizedError creates a new UnauthorizedError.
func NewUnauthorizedError(message string) *UnauthorizedError {
	return &UnauthorizedError{Message: message}
}

// ProviderError indicates an external provider failure.
type ProviderError struct {
	Provider string
	Message  string
}

func (e *ProviderError) Error() string {
	return fmt.Sprintf("%s provider error: %s", e.Provider, e.Message)
}

// NewProviderError creates a new ProviderError.
func NewProviderError(provider, message string) *ProviderError {
	return &ProviderError{Provider: provider, Message: message}
}
