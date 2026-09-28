package parser

import "slices"

func (branch Branch) HasExecution() bool { return branch.Command != nil || branch.Commands != nil }

func (branch Branch) Clone() Branch {
	branch.Command = clonePointer(branch.Command)
	branch.Cwd = clonePointer(branch.Cwd)
	branch.IgnoreError = clonePointer(branch.IgnoreError)
	branch.Args = slices.Clone(branch.Args)
	branch.Commands = slices.Clone(branch.Commands)
	for index := range branch.Commands {
		branch.Commands[index].Args = slices.Clone(branch.Commands[index].Args)
	}
	if branch.Env == nil {
		return branch
	}
	environment := make(map[string]EnvDefinition, len(branch.Env))
	for name, definition := range branch.Env {
		definition.Value = clonePointer(definition.Value)
		definition.Overrides = slices.Clone(definition.Overrides)
		for index := range definition.Overrides {
			definition.Overrides[index].Arch = slices.Clone(definition.Overrides[index].Arch)
			definition.Overrides[index].Value = clonePointer(definition.Overrides[index].Value)
		}
		environment[name] = definition
	}
	branch.Env = environment
	return branch
}

func clonePointer[Value any](value *Value) *Value {
	if value == nil {
		return nil
	}
	copiedValue := *value
	return &copiedValue
}
