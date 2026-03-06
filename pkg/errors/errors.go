// Package errors provides unified error types for the truth-market platform.
//
// AppError is the base application error carrying a machine-readable code and a
// human-readable message. Sentinel errors are provided for common failure modes
// (not found, unauthorized, etc.) and can be checked with [errors.Is].
//
// Example usage:
//
//	if errors.IsNotFound(err) {
//	    // handle missing resource
//	}
//
//	return errors.Wrap(err, "BAD_REQUEST", "email already registered")
package errors

import (
	"errors"
	"fmt"
)

// AppError is the base application error type. It carries a machine-readable
// Code suitable for API responses and a human-readable Message for logging and
// display. The optional Err field allows wrapping an underlying cause so that
// [errors.Unwrap] and [errors.Is] work as expected.
type AppError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Err     error  `json:"-"`
}

// Error returns a formatted string containing the error code and message.
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying error, if any. This enables use with
// [errors.Is] and [errors.As] from the standard library.
func (e *AppError) Unwrap() error {
	return e.Err
}

// Is reports whether target matches this AppError. Two AppErrors are considered
// equal when they share the same Code, which allows sentinel errors to be
// matched regardless of the wrapped cause.
func (e *AppError) Is(target error) bool {
	var t *AppError
	if errors.As(target, &t) {
		return e.Code == t.Code
	}
	return false
}

// New creates a new AppError with the given code and message.
func New(code, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
	}
}

// Wrap creates a new AppError that wraps an existing error with additional
// context. The wrapped error is accessible via [errors.Unwrap].
func Wrap(err error, code, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// Sentinel errors for common failure modes. Use the corresponding Is* helpers
// or [errors.Is] to check for these in calling code.
var (
	// ErrNotFound indicates the requested resource does not exist.
	ErrNotFound = New("NOT_FOUND", "resource not found")

	// ErrUnauthorized indicates that authentication is required.
	ErrUnauthorized = New("UNAUTHORIZED", "authentication required")

	// ErrForbidden indicates the caller lacks sufficient permissions.
	ErrForbidden = New("FORBIDDEN", "insufficient permissions")

	// ErrBadRequest indicates the request is malformed or contains invalid data.
	ErrBadRequest = New("BAD_REQUEST", "invalid request")

	// ErrConflict indicates a conflict with the current state of a resource
	// (e.g. duplicate creation).
	ErrConflict = New("CONFLICT", "resource conflict")

	// ErrInternalError indicates an unexpected server-side failure.
	ErrInternalError = New("INTERNAL_ERROR", "internal server error")

	// ErrInsufficientBalance indicates the user does not have enough funds to
	// complete the requested operation.
	ErrInsufficientBalance = New("INSUFFICIENT_BALANCE", "insufficient balance")

	// ErrMarketClosed indicates the market is not in a state that accepts
	// new orders.
	ErrMarketClosed = New("MARKET_CLOSED", "market is not open for trading")

	// ErrInvalidPrice indicates the supplied price falls outside the valid
	// range of [0.01, 0.99].
	ErrInvalidPrice = New("INVALID_PRICE", "price must be between 0.01 and 0.99")
)

// codeFromError extracts the Code from err if it is an *AppError, returning an
// empty string otherwise.
func codeFromError(err error) string {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return ""
}

// IsNotFound reports whether err (or any error in its chain) has the
// NOT_FOUND code.
func IsNotFound(err error) bool {
	return codeFromError(err) == ErrNotFound.Code
}

// IsUnauthorized reports whether err (or any error in its chain) has the
// UNAUTHORIZED code.
func IsUnauthorized(err error) bool {
	return codeFromError(err) == ErrUnauthorized.Code
}

// IsForbidden reports whether err (or any error in its chain) has the
// FORBIDDEN code.
func IsForbidden(err error) bool {
	return codeFromError(err) == ErrForbidden.Code
}

// IsBadRequest reports whether err (or any error in its chain) has the
// BAD_REQUEST code.
func IsBadRequest(err error) bool {
	return codeFromError(err) == ErrBadRequest.Code
}

// IsConflict reports whether err (or any error in its chain) has the
// CONFLICT code.
func IsConflict(err error) bool {
	return codeFromError(err) == ErrConflict.Code
}

// IsInternalError reports whether err (or any error in its chain) has the
// INTERNAL_ERROR code.
func IsInternalError(err error) bool {
	return codeFromError(err) == ErrInternalError.Code
}

// IsInsufficientBalance reports whether err (or any error in its chain) has
// the INSUFFICIENT_BALANCE code.
func IsInsufficientBalance(err error) bool {
	return codeFromError(err) == ErrInsufficientBalance.Code
}

// IsMarketClosed reports whether err (or any error in its chain) has the
// MARKET_CLOSED code.
func IsMarketClosed(err error) bool {
	return codeFromError(err) == ErrMarketClosed.Code
}

// IsInvalidPrice reports whether err (or any error in its chain) has the
// INVALID_PRICE code.
func IsInvalidPrice(err error) bool {
	return codeFromError(err) == ErrInvalidPrice.Code
}
