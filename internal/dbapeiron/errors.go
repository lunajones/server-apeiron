package dbapeiron

import (
	"fmt"

	"google.golang.org/grpc/status"
)

func mapGRPCError(operation string, err error) error {
	if err == nil {
		return nil
	}

	if st, ok := status.FromError(err); ok {
		return fmt.Errorf("%s failed with grpc code %s: %w", operation, st.Code(), err)
	}

	return fmt.Errorf("%s failed: %w", operation, err)
}
