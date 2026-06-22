package result

type Result struct {
	Success bool
	Code    Code
	Message string
}

func OK(message string) Result {
	return Result{Success: true, Code: CodeOK, Message: message}
}

func Fail(code Code, message string) Result {
	return Result{Success: false, Code: code, Message: message}
}
