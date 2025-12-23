package github

import (
	"fmt"
	"time"
)

// =============================================================================
// GitHub API Errors
// =============================================================================
// These error types implement the domain error interfaces from common.errors
// to enable proper error handling in the presentation layer.

// AuthenticationError indicates authentication failed (401).
type AuthenticationError struct {
	Message string
}

func (e *AuthenticationError) Error() string {
	return e.Message
}

// ShouldSilenceUsage returns true because authentication errors
// don't indicate incorrect command usage.
func (e *AuthenticationError) ShouldSilenceUsage() bool {
	return true
}

// UserMessage returns the user-friendly error message.
func (e *AuthenticationError) UserMessage() string {
	return e.Message
}

// NotFoundError indicates the resource was not found (404).
// This often happens when accessing private repos without proper authentication.
type NotFoundError struct {
	Message string
}

func (e *NotFoundError) Error() string {
	return e.Message
}

// ShouldSilenceUsage returns true because 404 errors on private repos
// don't indicate incorrect command usage - the user needs authentication.
func (e *NotFoundError) ShouldSilenceUsage() bool {
	return true
}

// UserMessage returns the user-friendly error message with recovery hints.
func (e *NotFoundError) UserMessage() string {
	return e.Message
}

// RecoveryHint returns actionable steps to fix the issue.
func (e *NotFoundError) RecoveryHint() string {
	return `To set up GitHub authentication:
  1. Create a Personal Access Token at https://github.com/settings/tokens
  2. Run: devnet-builder config set github-token <your-token>`
}

// RateLimitError indicates rate limiting (403/429).
type RateLimitError struct {
	Limit     int
	Remaining int
	Reset     time.Time
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("GitHub API rate limit exceeded. Reset at %s", e.Reset.Format(time.RFC1123))
}

// ShouldSilenceUsage returns true because rate limit errors
// don't indicate incorrect command usage.
func (e *RateLimitError) ShouldSilenceUsage() bool {
	return true
}

// UserMessage returns the user-friendly error message.
func (e *RateLimitError) UserMessage() string {
	return e.Error()
}

// RecoveryHint returns actionable steps to fix the issue.
func (e *RateLimitError) RecoveryHint() string {
	if !e.Reset.IsZero() {
		return fmt.Sprintf("Wait until %s or configure a GitHub token for higher rate limits.", e.Reset.Format(time.RFC1123))
	}
	return "Configure a GitHub token for higher rate limits, or wait and try again."
}

// StaleDataWarning indicates stale cached data is being used.
// This is not a fatal error - the operation can proceed with cached data.
type StaleDataWarning struct {
	Message string
}

func (e *StaleDataWarning) Error() string {
	return e.Message
}

// NetworkError indicates a network connectivity issue.
type NetworkError struct {
	Message string
	Cause   error
}

func (e *NetworkError) Error() string {
	return e.Message
}

func (e *NetworkError) Unwrap() error {
	return e.Cause
}

// ShouldSilenceUsage returns true because network errors
// don't indicate incorrect command usage.
func (e *NetworkError) ShouldSilenceUsage() bool {
	return true
}

// UserMessage returns the user-friendly error message.
func (e *NetworkError) UserMessage() string {
	return e.Message
}

// RecoveryHint returns actionable steps to fix the issue.
func (e *NetworkError) RecoveryHint() string {
	return "Check your internet connection and try again."
}
