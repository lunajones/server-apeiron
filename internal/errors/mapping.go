package errors

import (
	stderrors "errors"

	"server-apeiron/internal/result"
)

func ToResultCode(err error) result.Code {
	switch {
	case err == nil:
		return result.CodeOK
	case stderrors.Is(err, ErrDependencyUnavailable):
		return result.CodeDependencyFailed
	case stderrors.Is(err, ErrStartupValidation):
		return result.CodeStartupValidation
	case stderrors.Is(err, ErrEntityNotFound):
		return result.CodeNotFound
	default:
		return result.CodeInternal
	}
}
