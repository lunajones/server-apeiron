package logging

import (
	"log"

	"server-apeiron/internal/config"
)

type Logger struct {
	component string
	err       error
}

var Log = Logger{}

func Initialize(config.LoggerConfig) {}

func BootstrapError(message string, err error) {
	if err != nil {
		log.Printf("%s: %v", message, err)
		return
	}
	log.Print(message)
}

func WithComponent(component string) Logger {
	return Logger{component: component}
}

func (l Logger) Error() Logger {
	return l
}

func (l Logger) Warn() Logger {
	return l
}

func (l Logger) Info() Logger {
	return l
}

func (l Logger) Debug() Logger {
	return l
}

func (l Logger) Err(err error) Logger {
	l.err = err
	return l
}

func (l Logger) Str(string, string) Logger {
	return l
}

func (l Logger) Int(string, int) Logger {
	return l
}

func (l Logger) Int64(string, int64) Logger {
	return l
}

func (l Logger) Uint64(string, uint64) Logger {
	return l
}

func (l Logger) Float64(string, float64) Logger {
	return l
}

func (l Logger) Bool(string, bool) Logger {
	return l
}

func (l Logger) Msg(message string) {
	if l.component != "" {
		message = "[" + l.component + "] " + message
	}
	if l.err != nil {
		log.Printf("%s: %v", message, l.err)
		return
	}
	log.Print(message)
}

func (l Logger) Msgf(format string, args ...any) {
	log.Printf(format, args...)
}
