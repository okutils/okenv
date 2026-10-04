package runner

import (
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/okutils/okenv/internal/parser"
)

type Context struct {
	Config string `json:"config"`
	OS     string `json:"os"`
	Arch   string `json:"arch"`
	Branch string `json:"branch"`
}
type Plan struct {
	environment []string
	Context     Context          `json:"context"`
	Script      string           `json:"script"`
	Cwd         string           `json:"cwd"`
	Env         []string         `json:"env"`
	Commands    []parser.Command `json:"commands"`
}

func Select(script parser.Script, operatingSystem, architecture string) (parser.Branch, int, bool) {
	for index, override := range script.Overrides {
		if override.IsMatch(operatingSystem, architecture) {
			return override.Clone(), index, true
		}
	}
	return script.Clone(), -1, script.HasExecution() || len(script.Overrides) == 0
}
func Build(config *parser.Config, input, name, operatingSystem, architecture string) (*Plan, error) {
	executionPlan, operationError := build(config, input, name, operatingSystem, architecture)
	if operationError != nil {
		return nil, fmt.Errorf("%q: %w", input, operationError)
	}
	return executionPlan, nil
}
func build(config *parser.Config, input, name, operatingSystem, architecture string) (*Plan, error) {
	script, ok := config.Scripts[name]
	if !ok {
		return nil, fmt.Errorf("script %q does not exist", name)
	}
	selectedBranch, index, isAvailable := Select(script, operatingSystem, architecture)
	if !isAvailable {
		return nil, fmt.Errorf("script %q is unavailable on %s/%s", name, operatingSystem, architecture)
	}
	path := parser.MapPath("scripts", name)
	branch := "default"
	if index >= 0 {
		path += fmt.Sprintf(".overrides[%d]", index)
		branch = "override"
	}
	absolutePath, operationError := filepath.Abs(input)
	if operationError != nil {
		return nil, fmt.Errorf("resolve config path: %w", operationError)
	}
	executionPlan := &Plan{
		Context:  Context{Config: absolutePath, OS: operatingSystem, Arch: architecture, Branch: branch},
		Script:   name,
		Env:      []string{},
		Commands: []parser.Command{},
	}
	commands := selectedBranch.Commands
	isSingle := selectedBranch.Command != nil
	if isSingle {
		shouldIgnore := false
		if selectedBranch.IgnoreError != nil {
			shouldIgnore = *selectedBranch.IgnoreError
		}
		commands = []parser.Command{
			{Command: *selectedBranch.Command, Args: selectedBranch.Args, IgnoreError: shouldIgnore},
		}
	}
	shouldCheckNUL := operatingSystem == "windows" || operatingSystem == "linux" || operatingSystem == "darwin"
	check := func(value, path, what string) error {
		if shouldCheckNUL && strings.ContainsRune(value, 0) {
			return fmt.Errorf("%s: %s contains NUL after environment expansion", path, what)
		}
		return nil
	}
	checkExecution := func(value, path, what string) error {
		if len(commands) == 0 {
			return nil
		}
		return check(value, path, what)
	}
	for _, layer := range []struct {
		environment map[string]parser.EnvDefinition
		path        string
	}{{environment: config.Env, path: "env"}, {environment: selectedBranch.Env, path: path + ".env"}} {
		for _, entryName := range slices.Sorted(maps.Keys(layer.environment)) {
			definition := layer.environment[entryName]
			value := definition.Value
			environmentPath := parser.MapPath(layer.path, entryName)
			valuePath := environmentPath + ".value"
			for index, override := range definition.Overrides {
				if override.IsMatch(operatingSystem, architecture) {
					value = override.Value
					valuePath = fmt.Sprintf("%s.overrides[%d].value", environmentPath, index)
					break
				}
			}
			if value == nil {
				continue
			}
			expanded := os.ExpandEnv(*value)
			if operationError := check(entryName, environmentPath, "environment name"); operationError != nil {
				return nil, operationError
			}
			if operationError := check(expanded, valuePath, "environment value"); operationError != nil {
				return nil, operationError
			}
			executionPlan.Env = append(executionPlan.Env, entryName+"="+expanded)
		}
	}
	executionPlan.Cwd = filepath.Dir(absolutePath)
	if selectedBranch.Cwd != nil {
		executionPlan.Cwd = os.ExpandEnv(*selectedBranch.Cwd)
		if operationError := checkExecution(
			executionPlan.Cwd,
			path+".cwd",
			"working directory",
		); operationError != nil {
			return nil, operationError
		}
		if executionPlan.Cwd != "" && !filepath.IsAbs(executionPlan.Cwd) {
			executionPlan.Cwd = filepath.Join(filepath.Dir(absolutePath), executionPlan.Cwd)
		}
	}
	if operationError := checkExecution(executionPlan.Cwd, path+".cwd", "working directory"); operationError != nil {
		return nil, operationError
	}
	for index, command := range commands {
		commandPath := path
		if !isSingle {
			commandPath += fmt.Sprintf(".commands[%d]", index)
		}
		command.Command = os.ExpandEnv(command.Command)
		if command.Command == "" {
			return nil, fmt.Errorf("%s.command: command is empty after environment expansion", commandPath)
		}
		if operationError := checkExecution(command.Command, commandPath+".command", "command"); operationError != nil {
			return nil, operationError
		}
		arguments := make([]string, len(command.Args))
		for argumentIndex, argument := range command.Args {
			arguments[argumentIndex] = os.ExpandEnv(argument)
			if operationError := checkExecution(
				arguments[argumentIndex],
				fmt.Sprintf("%s.args[%d]", commandPath, argumentIndex),
				"argument",
			); operationError != nil {
				return nil, operationError
			}
		}
		command.Args = arguments
		executionPlan.Commands = append(executionPlan.Commands, command)
	}
	// Let os/exec prepare PWD before applying explicit configuration, then
	// normalize duplicates and platform-specific entries using the same API.
	process := &exec.Cmd{Dir: executionPlan.Cwd}
	process.Env = append(process.Environ(), executionPlan.Env...)
	executionPlan.environment = process.Environ()
	for _, layer := range []struct {
		options parser.Options
		path    string
	}{
		{options: config.Options, path: "options"},
		{options: selectedBranch.Options, path: path + ".options"},
	} {
		if err := checkEnvironment(layer.options, layer.path, executionPlan.environment); err != nil {
			return nil, err
		}
	}
	return executionPlan, nil
}

func checkEnvironment(options parser.Options, path string, environment []string) error {
	values := make(map[string]string, len(environment))
	normalize := func(name string) string {
		if runtime.GOOS == "windows" {
			return strings.ToLower(name)
		}
		return name
	}
	for _, entry := range environment {
		// os/exec permits Windows drive entries such as =C:=C:\work.
		separator := strings.IndexByte(entry, '=')
		if separator == 0 {
			separator = strings.IndexByte(entry[1:], '=') + 1
		}
		if separator >= 0 {
			values[normalize(entry[:separator])] = entry[separator+1:]
		}
	}
	for _, name := range slices.Sorted(maps.Keys(options.CheckEnv)) {
		rule := options.CheckEnv[name]
		rulePath := parser.MapPath(path+".checkEnv", name)
		value, ok := values[normalize(name)]
		if rule.IsRequired {
			if !ok {
				return fmt.Errorf("%s.required: environment variable is missing", rulePath)
			}
			if value == "" {
				return fmt.Errorf("%s.required: environment variable is empty", rulePath)
			}
		}
		if !ok {
			continue
		}
		if rule.Enum != nil && !slices.Contains(rule.Enum, value) {
			return fmt.Errorf("%s.enum: environment value is not in enum", rulePath)
		}
		if rule.CompiledPattern != nil && !rule.CompiledPattern.MatchString(value) {
			return fmt.Errorf("%s.pattern: environment value does not fully match pattern", rulePath)
		}
	}
	return nil
}
