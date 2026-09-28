package failure

import "fmt"

type Error struct {
	Kind    Kind
	Script  string
	Command int
	Err     error
}

func (failure *Error) Error() string {
	if failure.Command > 0 {
		return fmt.Sprintf("script %q, command %d: %v", failure.Script, failure.Command, failure.Err)
	}
	return failure.Err.Error()
}

func (failure *Error) Unwrap() error { return failure.Err }

var _ error = (*Error)(nil)
