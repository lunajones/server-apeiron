package errors

import "errors"

var (
	ErrInvalidState      = errors.New("invalid state")
	ErrInvalidTransition = errors.New("invalid transition")
	ErrEntityNotFound    = errors.New("entity not found")
)
