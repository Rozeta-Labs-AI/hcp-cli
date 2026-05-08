package cli

import (
	"errors"
	"fmt"
)

const (
	exitUsage  = 2
	exitAuth   = 4
	exitAPI    = 5
	exitConfig = 4
)

type codedError struct {
	code int
	err  error
}

func (e codedError) Error() string {
	return e.err.Error()
}

func (e codedError) Unwrap() error {
	return e.err
}

func (e codedError) ExitCode() int {
	return e.code
}

func errorf(code int, format string, args ...any) error {
	return codedError{
		code: code,
		err:  fmt.Errorf(format, args...),
	}
}

func ExitCode(err error) int {
	if err == nil {
		return 0
	}

	var coded interface {
		ExitCode() int
	}
	if errors.As(err, &coded) {
		return coded.ExitCode()
	}

	return 1
}
