// Copyright 2026 Red Hat, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package testutil

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// ExampleCommand represents a parsed command from an Example string
type ExampleCommand struct {
	Raw         string            // Original command line
	Binary      string            // Binary name (should be "kdn")
	Args        []string          // Subcommands and positional arguments
	FlagPresent map[string]bool   // Flags that were present in the command
	FlagValues  map[string]string // Values for flags (empty string if no value provided)
	Flags       map[string]string // Deprecated: use FlagPresent and FlagValues instead
	EnvVars     map[string]string // Environment variable assignments that preceded the command
}

// ParseExampleCommands extracts kdn commands from Example string
// - Ignores empty lines and comment lines (starting with #)
// - Lines may be prefixed with VAR=VALUE environment variable assignments
// - Returns error if non-comment lines do not contain a kdn command
func ParseExampleCommands(example string) ([]ExampleCommand, error) {
	var commands []ExampleCommand
	lines := strings.Split(example, "\n")

	for i, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines
		if line == "" {
			continue
		}

		// Skip comment lines
		if strings.HasPrefix(line, "#") {
			continue
		}

		// Parse command line
		cmd, err := parseCommandLine(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, err)
		}

		commands = append(commands, cmd)
	}

	return commands, nil
}

// isEnvVarAssignment returns true if token looks like KEY=VALUE where KEY is a
// valid shell identifier (letters, digits, underscores; not starting with a digit).
func isEnvVarAssignment(token string) bool {
	idx := strings.Index(token, "=")
	if idx <= 0 {
		return false
	}
	key := token[:idx]
	if key[0] >= '0' && key[0] <= '9' {
		return false
	}
	for _, ch := range key {
		if !((ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_') {
			return false
		}
	}
	return true
}

// parseCommandLine parses a single command line into ExampleCommand.
// Leading VAR=VALUE tokens are collected into EnvVars; the next token must be "kdn".
func parseCommandLine(line string) (ExampleCommand, error) {
	// Split by whitespace, respecting quotes (simple parsing)
	parts := splitCommandLine(line)
	if len(parts) == 0 {
		return ExampleCommand{}, fmt.Errorf("empty command line")
	}

	// Collect leading VAR=VALUE environment variable assignments.
	envVars := make(map[string]string)
	start := 0
	for start < len(parts) && isEnvVarAssignment(parts[start]) {
		idx := strings.Index(parts[start], "=")
		envVars[parts[start][:idx]] = parts[start][idx+1:]
		start++
	}

	if start >= len(parts) {
		return ExampleCommand{}, fmt.Errorf("no command found after environment variable assignments")
	}

	if parts[start] != "kdn" {
		return ExampleCommand{}, fmt.Errorf("command must start with 'kdn', got '%s'", parts[start])
	}

	cmd := ExampleCommand{
		Raw:         line,
		Binary:      parts[start],
		Args:        []string{},
		FlagPresent: make(map[string]bool),
		FlagValues:  make(map[string]string),
		Flags:       make(map[string]string),
		EnvVars:     envVars,
	}

	// Parse remaining parts as args and flags
	for i := start + 1; i < len(parts); i++ {
		part := parts[i]

		// Check for -- separator (stop flag parsing)
		if part == "--" {
			// Everything after -- is treated as positional arguments
			cmd.Args = append(cmd.Args, parts[i+1:]...)
			break
		}

		if strings.HasPrefix(part, "--") {
			// Long flag
			flagName, flagValue, hasValue := parseLongFlag(part, parts, &i)
			cmd.FlagPresent[flagName] = true
			cmd.FlagValues[flagName] = flagValue
			// Maintain backward compatibility with deprecated Flags field
			cmd.Flags[flagName] = flagValue
			_ = hasValue // Will be used in validation
		} else if strings.HasPrefix(part, "-") && len(part) > 1 {
			// Short flag
			flagName, flagValue, hasValue := parseShortFlag(part, parts, &i)
			cmd.FlagPresent[flagName] = true
			cmd.FlagValues[flagName] = flagValue
			// Maintain backward compatibility with deprecated Flags field
			cmd.Flags[flagName] = flagValue
			_ = hasValue // Will be used in validation
		} else {
			// Positional argument
			cmd.Args = append(cmd.Args, part)
		}
	}

	return cmd, nil
}

// parseLongFlag parses a long flag (--flag or --flag=value or --flag value)
// Returns: flagName, flagValue, hasValue
func parseLongFlag(part string, parts []string, i *int) (string, string, bool) {
	// Remove -- prefix
	flagPart := strings.TrimPrefix(part, "--")

	// Check for --flag=value format
	if idx := strings.Index(flagPart, "="); idx != -1 {
		return flagPart[:idx], flagPart[idx+1:], true
	}

	// Check for --flag value format
	if *i+1 < len(parts) && !strings.HasPrefix(parts[*i+1], "-") {
		*i++
		return flagPart, parts[*i], true
	}

	// Flag with no value (--flag)
	return flagPart, "", false
}

// parseShortFlag parses a short flag (-f or -f value)
// Returns: flagName, flagValue, hasValue
func parseShortFlag(part string, parts []string, i *int) (string, string, bool) {
	// Remove - prefix
	flagPart := strings.TrimPrefix(part, "-")

	// Check for -f value format
	if *i+1 < len(parts) && !strings.HasPrefix(parts[*i+1], "-") {
		*i++
		return flagPart, parts[*i], true
	}

	// Flag with no value (-f)
	return flagPart, "", false
}

// splitCommandLine splits a command line by whitespace
func splitCommandLine(line string) []string {
	var parts []string
	var current strings.Builder
	inQuote := false
	quoteChar := rune(0)

	for _, char := range line {
		switch {
		case (char == '"' || char == '\'') && !inQuote:
			inQuote = true
			quoteChar = char
		case char == quoteChar && inQuote:
			inQuote = false
			quoteChar = 0
		case char == ' ' && !inQuote:
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(char)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// ValidateExampleCommand verifies command exists and flags are valid
// - Uses Cobra's Find() to locate command in tree
// - Checks both command-specific and persistent flags
func ValidateExampleCommand(rootCmd *cobra.Command, exampleCmd ExampleCommand) error {
	// Find the command in the tree
	cmd, remainingArgs, err := rootCmd.Find(exampleCmd.Args)
	if err != nil {
		return fmt.Errorf("command not found: %s: %w", strings.Join(exampleCmd.Args, " "), err)
	}

	// Validate remaining arguments using the command's Args validator
	// This respects cobra.NoArgs, cobra.ExactArgs, etc.
	if cmd.Args != nil {
		if err := cmd.Args(cmd, remainingArgs); err != nil {
			// Provide a clearer error message for unknown subcommands
			if len(remainingArgs) > 0 && cmd.HasSubCommands() {
				return fmt.Errorf("unknown command %q for %q", strings.Join(remainingArgs, " "), cmd.CommandPath())
			}
			return fmt.Errorf("invalid arguments for %q: %w", cmd.CommandPath(), err)
		}
	}

	// Validate each flag
	for flagName := range exampleCmd.FlagPresent {
		var flag *pflag.Flag

		// Determine if this is a short flag (single character)
		isShortFlag := len(flagName) == 1

		if isShortFlag {
			// For short flags, use ShorthandLookup
			// Check local flags
			flag = cmd.Flags().ShorthandLookup(flagName)
			if flag == nil {
				// Check persistent flags
				flag = cmd.PersistentFlags().ShorthandLookup(flagName)
			}
			if flag == nil {
				// Check inherited flags from parent commands
				flag = cmd.InheritedFlags().ShorthandLookup(flagName)
			}
		} else {
			// For long flags, use regular Lookup
			// Check local flags
			flag = cmd.Flags().Lookup(flagName)
			if flag == nil {
				// Check persistent flags
				flag = cmd.PersistentFlags().Lookup(flagName)
			}
			if flag == nil {
				// Check inherited flags from parent commands
				flag = cmd.InheritedFlags().Lookup(flagName)
			}
		}

		if flag == nil {
			cmdPath := cmd.CommandPath()
			prefix := "--"
			if isShortFlag {
				prefix = "-"
			}
			return fmt.Errorf("flag %s%s not found in command: %s", prefix, flagName, cmdPath)
		}

		// Validate that flags requiring values have values
		// A flag requires a value if:
		// - It's present in the command line
		// - It has no value (empty string)
		// - NoOptDefVal is empty (meaning the flag doesn't accept being used without a value)
		// - It's not a boolean flag
		flagValue := exampleCmd.FlagValues[flagName]
		if flagValue == "" && flag.NoOptDefVal == "" && flag.Value.Type() != "bool" {
			cmdPath := cmd.CommandPath()
			prefix := "--"
			if isShortFlag {
				prefix = "-"
			}
			return fmt.Errorf("flag %s%s requires a value in command: %s", prefix, flagName, cmdPath)
		}
	}

	return nil
}

// ValidateCommandExamples combines parsing and validation
func ValidateCommandExamples(rootCmd *cobra.Command, example string) error {
	commands, err := ParseExampleCommands(example)
	if err != nil {
		return fmt.Errorf("failed to parse examples: %w", err)
	}

	for _, cmd := range commands {
		if err := ValidateExampleCommand(rootCmd, cmd); err != nil {
			return fmt.Errorf("invalid example '%s': %w", cmd.Raw, err)
		}
	}

	return nil
}
