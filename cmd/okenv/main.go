package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"runtime/debug"

	"github.com/okutils/okenv/internal/constants"
	"github.com/okutils/okenv/internal/failure"
	"github.com/okutils/okenv/internal/parser"
	"github.com/okutils/okenv/internal/runner"
)

func main() {
	operationError := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	if operationError == nil {
		return
	}
	code := 125
	var executionFailure *failure.Error
	if errors.As(operationError, &executionFailure) {
		switch executionFailure.Kind {
		case failure.Start:
			code = 126
			if errors.Is(operationError, exec.ErrNotFound) {
				code = 127
			}
		case failure.Exit:
			var exitError *exec.ExitError
			if errors.As(operationError, &exitError) {
				code = exitError.ExitCode()
				if code < 0 {
					code = 1
				}
			}
		}
	}
	_, _ = fmt.Fprintf(os.Stderr, "okenv: %v\n", operationError)
	os.Exit(code)
}

func run(arguments []string, stdin io.Reader, stdout, stderr io.Writer) error {
	var config, name, format string
	var isList, isDryRun, isVerbose, isHelp, isVersion bool
	flagSet := flag.NewFlagSet("okenv", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)
	flagSet.StringVar(&config, "config", constants.ConfigName, "JSON configuration path")
	flagSet.StringVar(&name, "run", constants.DefaultScript, "exact script name")
	flagSet.StringVar(&format, "format", "text", "list or preview format: text|json")
	flagSet.BoolVar(&isList, "list", false, "list scripts")
	flagSet.BoolVar(&isList, "l", false, "alias for --list")
	flagSet.BoolVar(&isDryRun, "dry-run", false, "preview without executing commands")
	flagSet.BoolVar(&isVerbose, "verbose", false, "show execution details on stderr (may expose sensitive values)")
	flagSet.BoolVar(&isHelp, "help", false, "show help")
	flagSet.BoolVar(&isHelp, "h", false, "alias for --help")
	flagSet.BoolVar(&isVersion, "version", false, "show version")
	usage := func() []byte {
		var buffer bytes.Buffer
		buffer.WriteString("Usage: okenv [options]\n\nOptions:\n")
		flagSet.SetOutput(&buffer)
		flagSet.PrintDefaults()
		flagSet.SetOutput(io.Discard)
		return bytes.ReplaceAll(buffer.Bytes(), []byte("\n  -"), []byte("\n  --"))
	}
	argumentError := func(operationError error) error { return fmt.Errorf("%w\n%s", operationError, usage()) }
	if operationError := flagSet.Parse(arguments); operationError != nil {
		return argumentError(operationError)
	}
	if flagSet.NArg() > 0 {
		return argumentError(fmt.Errorf("unexpected positional arguments: %q", flagSet.Args()))
	}
	specifiedOptions := map[string]bool{}
	flagSet.Visit(func(option *flag.Flag) { specifiedOptions[option.Name] = true })
	if isList && (specifiedOptions["run"] || isDryRun || isVerbose) || isDryRun && isVerbose {
		return argumentError(fmt.Errorf("conflicting operation options"))
	}
	if (isHelp || isVersion) &&
		(isHelp && isVersion || isList || isDryRun || isVerbose || specifiedOptions["run"] || specifiedOptions["config"] || specifiedOptions["format"]) {
		return argumentError(fmt.Errorf("help and version must be used independently"))
	}
	if format != "text" && format != "json" {
		return argumentError(fmt.Errorf("unsupported format %q", format))
	}
	if specifiedOptions["format"] && !isList && !isDryRun {
		return argumentError(fmt.Errorf("--format requires --list or --dry-run"))
	}
	if isHelp {
		buffer := []byte("okenv runs named scripts with JSON environment configuration.\n\n")
		buffer = append(buffer, usage()...)
		buffer = append(
			buffer,
			[]byte(
				"\nWithout --config, only ./config.okenv.json is opened; no parent search or fallback.\nWithout --run, the exact script name default is selected.\n--list conflicts with --run, --dry-run and --verbose.\n--verbose conflicts with --dry-run, writes text to stderr, and may expose sensitive values.\n--format applies only to lists and previews. Help and version must stand alone.\nBoolean options default to false. Positional arguments are not supported.\n\nExamples:\n  okenv --run dev\n  okenv --config config.okenv.production.json --run build\n  okenv --list\n  okenv --run dev --dry-run --format json\n  okenv --run dev --verbose\n",
			)...)
		return write(stdout, buffer)
	}
	if isVersion {
		versionValue := "unknown"
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
			versionValue = info.Main.Version
		}
		return write(
			stdout,
			fmt.Appendf(nil, "okenv %s (%s, %s/%s)\n", versionValue, runtime.Version(), runtime.GOOS, runtime.GOARCH),
		)
	}
	configuration, operationError := parser.Load(config)
	if operationError != nil {
		return operationError
	}
	if isList {
		body, operationError := listBody(configuration, runtime.GOOS, runtime.GOARCH, format)
		if operationError != nil {
			return operationError
		}
		return write(stdout, body)
	}
	executionPlan, operationError := runner.Build(configuration, config, name, runtime.GOOS, runtime.GOARCH)
	if operationError != nil {
		return operationError
	}
	if isDryRun {
		body, operationError := planBody(executionPlan, format)
		if operationError != nil {
			return operationError
		}
		if operationError = write(
			stderr,
			[]byte(Warning+"Preview only. No commands will be executed.\n"),
		); operationError != nil {
			return operationError
		}
		return write(stdout, body)
	}
	hooks := runner.Hooks{Ignored: func(index int, operationError error) error {
		return write(
			stderr,
			fmt.Appendf(
				nil,
				"okenv: script %q, command %d: ignored failure: %v\n",
				executionPlan.Script,
				index+1,
				operationError,
			),
		)
	}}

	if isVerbose {
		if operationError := write(
			stderr,
			[]byte(Warning+"Each command will be attempted after its details are printed.\n"),
		); operationError != nil {
			return operationError
		}
		if operationError := write(stderr, commonBody(executionPlan)); operationError != nil {
			return operationError
		}
		hooks.Before = func(index int) error { return write(stderr, commandBody(executionPlan, index)) }
	}
	return runner.Execute(executionPlan, stdin, stdout, stderr, hooks)
}
