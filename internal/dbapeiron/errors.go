package dbapeiron

import (
	"errors"
	"fmt"

	"google.golang.org/grpc/status"
)

var ErrRequiredDBUnavailable = errors.New("required db-apeiron unavailable")

func ErrRequiredUnavailable(reason string) error {
	if reason == "" {
		return ErrRequiredDBUnavailable
	}
	return fmt.Errorf("%w: %s", ErrRequiredDBUnavailable, reason)
}

func mapGRPCError(operation string, err error) error {
	if err == nil {
		return nil
	}

	if st, ok := status.FromError(err); ok {
		return fmt.Errorf("%s failed with grpc code %s: %w", operation, st.Code(), err)
	}

	return fmt.Errorf("%s failed: %w", operation, err)
}
