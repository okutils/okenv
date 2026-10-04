package runner

import (
	"errors"
	"io"
	"os/exec"

	"github.com/okutils/okenv/internal/failure"
)

type (
	BeforeHook  func(index int) error
	IgnoredHook func(index int, operationError error) error
	Hooks       struct {
		Before  BeforeHook
		Ignored IgnoredHook
	}
)

func Execute(executionPlan *Plan, stdin io.Reader, stdout, stderr io.Writer, hooks Hooks) error {
	for index, command := range executionPlan.Commands {
		wrap := func(kind failure.Kind, operationError error) error {
			return &failure.Error{Kind: kind, Script: executionPlan.Script, Command: index + 1, Err: operationError}
		}
		if hooks.Before != nil {
			if operationError := hooks.Before(index); operationError != nil {
				return wrap(failure.Own, operationError)
			}
		}
		process := exec.Command(command.Command, command.Args...)
		process.Dir = executionPlan.Cwd
		process.Env = executionPlan.environment
		process.Stdin = stdin
		process.Stdout = stdout
		process.Stderr = stderr
		if operationError := process.Run(); operationError != nil {
			var exit *exec.ExitError
			if !errors.As(operationError, &exit) {
				return wrap(failure.Start, operationError)
			}
			if exit.ExitCode() < 0 || !command.IgnoreError {
				return wrap(failure.Exit, operationError)
			}
			if hooks.Ignored != nil {
				if hookError := hooks.Ignored(index, operationError); hookError != nil {
					return wrap(failure.Own, hookError)
				}
			}
		}
	}
	return nil
}
