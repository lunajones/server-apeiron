package mappers

import apeironv1 "db-apeiron/gen/apeiron/v1"

func OperationSucceeded(result *apeironv1.OperationResult) bool {
	return result != nil && result.Success
}
