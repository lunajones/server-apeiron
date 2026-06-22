package errors

import "errors"

var (
	ErrDependencyUnavailable = errors.New("dependency unavailable")
	ErrStartupValidation     = errors.New("startup validation failed")
	ErrShutdownTimeout       = errors.New("shutdown timeout")
)
