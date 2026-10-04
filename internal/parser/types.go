package parser

import (
	"regexp"

	"github.com/okutils/okenv/internal/platform"
)

type Config struct {
	Options Options                  `json:"options"`
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
	IgnoreError bool     `json:"ignoreError"`
}

type Branch struct {
	Options     Options                  `json:"options"`
	Command     *string                  `json:"command"`
	Args        []string                 `json:"args"`
	Commands    []Command                `json:"commands"`
	Cwd         *string                  `json:"cwd"`
	Env         map[string]EnvDefinition `json:"env"`
	IgnoreError *bool                    `json:"ignoreError"`
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

type Options struct {
	CheckEnv map[string]EnvCheck `json:"checkEnv"`
}

type EnvCheck struct {
	IsRequired      bool           `json:"required"`
	Enum            []string       `json:"enum"`
	Pattern         *string        `json:"pattern"`
	CompiledPattern *regexp.Regexp `json:"-"`
}
