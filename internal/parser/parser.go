package parser

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"regexp/syntax"
	"strconv"
	"strings"
)

var _ json.Unmarshaler = (*Command)(nil)

func Load(path string) (*Config, error) {
	configFile, operationError := os.Open(path)
	if operationError != nil {
		return nil, fmt.Errorf("%q: open config: %w", path, operationError)
	}
	data, operationError := io.ReadAll(configFile)
	_ = configFile.Close()
	if operationError != nil {
		return nil, fmt.Errorf("%q: read config: %w", path, operationError)
	}
	var config Config
	if operationError = json.Unmarshal(data, &config); operationError != nil {
		return nil, fmt.Errorf("%q: decode config: %w", path, operationError)
	}
	if operationError = config.Validate(); operationError != nil {
		return nil, fmt.Errorf("%q: validate config: %w", path, operationError)
	}
	return &config, nil
}

func (config *Config) Validate() error {
	if err := validateOptions(config.Options, "options"); err != nil {
		return err
	}
	if operationError := validateEnv(config.Env, "env"); operationError != nil {
		return operationError
	}
	for name, script := range config.Scripts {
		fieldPath := MapPath("scripts", name)
		if !script.HasExecution() && len(script.Overrides) > 0 &&
			(script.Cwd != nil || script.Env != nil || script.Args != nil || script.IgnoreError != nil || script.Options.CheckEnv != nil) {
			return fmt.Errorf("%s: default execution fields have no default branch", fieldPath)
		}
		if operationError := validateBranch(script.Branch, fieldPath); operationError != nil {
			return operationError
		}
		seen := map[string]bool{}
		for index, override := range script.Overrides {
			overridePath := fmt.Sprintf("%s.overrides[%d]", fieldPath, index)
			if operationError := override.Validate(seen); operationError != nil {
				return fmt.Errorf("%s: %w", overridePath, operationError)
			}
			if operationError := validateBranch(override.Branch, overridePath); operationError != nil {
				return operationError
			}
		}
	}
	return nil
}

func (command *Command) UnmarshalJSON(data []byte) error {
	type plainCommand Command
	decoded := struct {
		*plainCommand

		Options Options `json:"options"`
	}{plainCommand: (*plainCommand)(command)}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if decoded.Options.CheckEnv != nil {
		return fmt.Errorf("commands entries do not support options.checkEnv")
	}
	return nil
}

func validateBranch(branch Branch, fieldPath string) error {
	if branch.Command != nil && branch.Commands != nil {
		return fmt.Errorf("%s: command and commands are mutually exclusive", fieldPath)
	}
	if branch.Command == nil && (branch.Args != nil || branch.IgnoreError != nil) {
		return fmt.Errorf("%s: args and ignore_error require a single command", fieldPath)
	}
	if err := validateOptions(branch.Options, fieldPath+".options"); err != nil {
		return err
	}
	return validateEnv(branch.Env, fieldPath+".env")
}

func validateEnv(environment map[string]EnvDefinition, path string) error {
	for name, definition := range environment {
		fieldPath := MapPath(path, name)
		if name == "" || strings.Contains(name, "=") {
			return fmt.Errorf("%s: environment name must be nonempty and must not contain '='", fieldPath)
		}
		if definition.Value == nil && len(definition.Overrides) == 0 {
			return fmt.Errorf("%s: environment definition has no value source", fieldPath)
		}
		seen := map[string]bool{}
		for index, override := range definition.Overrides {
			overridePath := fmt.Sprintf("%s.overrides[%d]", fieldPath, index)
			if operationError := override.Validate(seen); operationError != nil {
				return fmt.Errorf("%s: %w", overridePath, operationError)
			}
			if override.Value == nil {
				return fmt.Errorf("%s.value: value is required", overridePath)
			}
		}
	}
	return nil
}

func MapPath(base, entryName string) string { return base + "[" + strconv.Quote(entryName) + "]" }

func validateOptions(options Options, path string) error {
	for name, rule := range options.CheckEnv {
		rulePath := MapPath(path+".checkEnv", name)
		if rule.IsRequired && rule.Enum != nil && len(rule.Enum) == 0 {
			return fmt.Errorf("%s: required cannot be combined with an empty enum", rulePath)
		}
		rule.CompiledPattern = nil
		if rule.Pattern != nil {
			// Parse independently so grouping cannot repair invalid expressions,
			// and normalize quoted literals before adding full-value boundaries.
			expression, err := syntax.Parse(*rule.Pattern, syntax.Perl)
			if err != nil {
				return fmt.Errorf("%s.pattern: %w", rulePath, err)
			}
			compiled, err := regexp.Compile(`\A(?:` + expression.String() + `)\z`)
			if err != nil {
				return fmt.Errorf("%s.pattern: %w", rulePath, err)
			}
			rule.CompiledPattern = compiled
		}
		options.CheckEnv[name] = rule
	}
	return nil
}
