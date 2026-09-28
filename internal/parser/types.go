package parser

import "github.com/okutils/okenv/internal/platform"

type Config struct {
	Env     map[string]EnvDefinition `json:"env"`
	Scripts map[string]Script        `json:"scripts"`
}

type EnvDefinition struct {
	Value     *string       `json:"value"`
	Overrides []EnvOverride `json:"overrides"`
}

type EnvOverride struct {
	platform.Condition

	Value *string `json:"value"`
}

type Command struct {
	Command     string   `json:"command"`
	Args        []string `json:"args"`
	IgnoreError bool     `json:"ignore_error"`
}

type Branch struct {
	Command     *string                  `json:"command"`
	Args        []string                 `json:"args"`
	Commands    []Command                `json:"commands"`
	Cwd         *string                  `json:"cwd"`
	Env         map[string]EnvDefinition `json:"env"`
	IgnoreError *bool                    `json:"ignore_error"`
}

type Script struct {
	Branch
	Description string     `json:"description"`
	Overrides   []Override `json:"overrides"`
}

type Override struct {
	platform.Condition
	Branch
}
