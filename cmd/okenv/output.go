package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"text/tabwriter"

	"github.com/okutils/okenv/internal/parser"
	"github.com/okutils/okenv/internal/runner"
)

const Warning = "Warning: Output may contain sensitive values. Review before sharing or logging.\n"

type listItem struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Available   bool   `json:"available"`
}

func listBody(config *parser.Config, operatingSystem, architecture, format string) ([]byte, error) {
	names := make([]string, 0, len(config.Scripts))
	for name := range config.Scripts {
		names = append(names, name)
	}
	sort.Strings(names)
	items := make([]listItem, 0, len(names))

	for _, name := range names {
		script := config.Scripts[name]
		_, _, isAvailable := runner.Select(script, operatingSystem, architecture)
		items = append(items, listItem{Name: name, Description: script.Description, Available: isAvailable})
	}

	if format == "json" {
		return jsonBody(struct {
			Scripts []listItem `json:"scripts"`
		}{Scripts: items})
	}

	var buffer bytes.Buffer
	tableWriter := tabwriter.NewWriter(
		&buffer, /* minWidth */
		0,       /* tabWidth */
		0,       /* padding */
		2,       /* padCharacter */
		' ',     /* flags */
		0,
	)
	if _, operationError := fmt.Fprintln(tableWriter, "NAME\tAVAILABLE\tDESCRIPTION"); operationError != nil {
		return nil, fmt.Errorf("write script list header: %w", operationError)
	}

	for _, item := range items {
		available := "no"
		if item.Available {
			available = "yes"
		}
		if _, operationError := fmt.Fprintf(
			tableWriter,
			"%s\t%s\t%s\n",
			strconv.Quote(item.Name),
			available,
			strconv.Quote(item.Description),
		); operationError != nil {
			return nil, fmt.Errorf("write script list row: %w", operationError)
		}
	}

	if operationError := tableWriter.Flush(); operationError != nil {
		return nil, fmt.Errorf("format script list: %w", operationError)
	}
	return buffer.Bytes(), nil
}

func planBody(executionPlan *runner.Plan, format string) ([]byte, error) {
	if format == "json" {
		return jsonBody(executionPlan)
	}
	buffer := commonBody(executionPlan)
	for index := range executionPlan.Commands {
		buffer = append(buffer, commandBody(executionPlan, index)...)
	}
	return buffer, nil
}

func commonBody(executionPlan *runner.Plan) []byte {
	var buffer bytes.Buffer
	fmt.Fprintf(
		&buffer,
		"context:\n  config: %s\n  os: %s\n  arch: %s\n  branch: %s\nscript: %s\ncwd: %s\n",
		strconv.Quote(executionPlan.Context.Config),
		strconv.Quote(executionPlan.Context.OS),
		strconv.Quote(executionPlan.Context.Arch),
		strconv.Quote(executionPlan.Context.Branch),
		strconv.Quote(executionPlan.Script),
		strconv.Quote(executionPlan.Cwd),
	)
	if len(executionPlan.Env) == 0 {
		fmt.Fprintln(&buffer, "env: []")
	}
	for index, environmentEntry := range executionPlan.Env {
		fmt.Fprintf(&buffer, "env[%d]: %s\n", index, strconv.Quote(environmentEntry))
	}
	if len(executionPlan.Commands) == 0 {
		fmt.Fprintln(&buffer, "commands: []")
	}
	return buffer.Bytes()
}

func commandBody(executionPlan *runner.Plan, index int) []byte {
	var buffer bytes.Buffer
	command := executionPlan.Commands[index]
	fmt.Fprintf(&buffer, "\ncommand %d:\n  command: %s\n", index+1, strconv.Quote(command.Command))
	if len(command.Args) == 0 {
		fmt.Fprintln(&buffer, "  args: []")
	}
	if len(command.Args) > 0 {
		fmt.Fprintln(&buffer, "  args:")
		for argumentIndex, argument := range command.Args {
			fmt.Fprintf(&buffer, "    [%d]: %s\n", argumentIndex, strconv.Quote(argument))
		}
	}
	fmt.Fprintf(&buffer, "  ignore_error: %t\n", command.IgnoreError)
	return buffer.Bytes()
}

func jsonBody(value any) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetIndent("", "  ")
	if operationError := encoder.Encode(value); operationError != nil {
		return nil, fmt.Errorf("encode JSON output: %w", operationError)
	}
	return buffer.Bytes(), nil
}

func write(writer io.Writer, data []byte) error {
	bytesWritten, operationError := writer.Write(data)
	if operationError == nil && bytesWritten != len(data) {
		operationError = io.ErrShortWrite
	}
	if operationError != nil {
		return fmt.Errorf("write output: %w", operationError)
	}
	return nil
}
