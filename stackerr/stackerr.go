package stackerr

import (
	"fmt"
	"runtime"
	"strings"
)

var _ error = (*StackErr)(nil)

type StackErr struct {
	strError string
}

func New() error {
	err := &StackErr{}
	err.init()

	return err
}

func (err *StackErr) init() {
	pc := make([]uintptr, 7)
	n := runtime.Callers(2, pc)
	if n == 0 {
		return
	}

	b := strings.Builder{}
	pc = pc[:n]
	frames := runtime.CallersFrames(pc)

	for {
		frame, more := frames.Next()
		b.WriteString(fmt.Sprintf("%s()\n", frame.Function))
		b.WriteString(fmt.Sprintf("	%s:%d\n", frame.File, frame.Line))

		if !more {
			break
		}
	}

	err.strError = b.String()

}

func (err *StackErr) Error() string {
	return err.strError
}
