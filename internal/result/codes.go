package result

type Code string

const (
	CodeOK                 Code = "OK"
	CodeInvalidArgument    Code = "INVALID_ARGUMENT"
	CodeNotFound           Code = "NOT_FOUND"
	CodeUnavailable        Code = "UNAVAILABLE"
	CodeDeadlineExceeded   Code = "DEADLINE_EXCEEDED"
	CodeInternal           Code = "INTERNAL"
	CodeDependencyFailed   Code = "DEPENDENCY_FAILED"
	CodeStartupValidation  Code = "STARTUP_VALIDATION_FAILED"
	CodePanicRecovered     Code = "PANIC_RECOVERED"
	CodeTickBudgetExceeded Code = "TICK_BUDGET_EXCEEDED"
)
