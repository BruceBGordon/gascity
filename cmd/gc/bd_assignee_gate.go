package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gastownhall/gascity/internal/config"
)

// assigneeGateEscapeEnv lets a caller write an assignee the roster cannot
// confirm. It exists so the gate cannot wedge a seat that legitimately needs a
// target the static config does not describe; using it is a deliberate act that
// shows up in the command, not a silent default.
const assigneeGateEscapeEnv = "GC_ALLOW_UNRESOLVED_ASSIGNEE"

// assigneeGateBypassed reports whether the caller asked to skip the check.
// Recognized false spellings turn the bypass OFF rather than on: the error
// message tells operators to "set GC_ALLOW_UNRESOLVED_ASSIGNEE=1", and someone
// who writes 0 in an env file to turn it back off must not silently disable
// the gate instead.
func assigneeGateBypassed() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(assigneeGateEscapeEnv))) {
	case "", "0", "false", "no", "off":
		return false
	default:
		return true
	}
}

// checkBdAssigneeArgs rejects a bd invocation that would write an assignee
// naming nothing the city can route to.
//
// This covers writes that go through `gc bd`. It is not a complete boundary:
// anything invoking the `bd` binary directly bypasses it, which is why the
// reconciler report and the `assignee-resolves` doctor check exist alongside
// it. Those catch the value wherever it came from; this one stops the common
// case at the moment the mistake is made, when the caller still knows what they
// meant.
func checkBdAssigneeArgs(cfg *config.City, args []string, stderr io.Writer) error {
	values := extractAssigneeArgs(args)
	if len(values) == 0 {
		return nil
	}
	roster := newAssigneeRoster(cfg)
	if roster.Empty() {
		// Refusing every write because the config did not expand would be a
		// worse failure than the one being prevented. Say so and allow it.
		fmt.Fprintf(stderr, "gc bd: warning: no agents or named sessions resolved from config; assignee not checked\n") //nolint:errcheck
		return nil
	}
	var bad []string
	for _, value := range values {
		if !roster.Resolves(value) {
			bad = append(bad, value)
		}
	}
	if len(bad) == 0 {
		return nil
	}
	return fmt.Errorf(
		"assignee %s matches no configured agent or named session, so nothing would ever pick this work up.\n"+
			"  Assign to a configured target, add the missing one to city.toml, or set %s=1 to write it anyway",
		strings.Join(quoteAll(bad), ", "), assigneeGateEscapeEnv)
}

// extractAssigneeArgs pulls every value a bd invocation would write to the
// assignee field, in both `--assignee=x` and `--assignee x` spellings, plus the
// positional form of `bd assign <id> <assignee>`.
//
// The positional form is found by locating the literal `assign` token rather
// than by taking the first non-flag argument as the subcommand. A global flag
// that carries a separate value (`bd --db /path assign gc-1 polecat-4`) puts a
// non-flag token ahead of the subcommand, and the positional reading used to
// land on that value instead, silently skipping the check for exactly the
// invocation shape it exists to catch.
func extractAssigneeArgs(args []string) []string {
	var values []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--assignee" || arg == "-a":
			if i+1 < len(args) {
				values = append(values, args[i+1])
				i++
			}
		case strings.HasPrefix(arg, "--assignee="):
			values = append(values, strings.TrimPrefix(arg, "--assignee="))
		}
	}
	if positional, ok := positionalAssignArg(args); ok {
		values = append(values, positional)
	}
	return values
}

// positionalAssignArg returns the assignee written by `bd assign <id> <name>`.
// bd documents exactly two positional operands for that subcommand, so the
// assignee is the second one after the token, not the last argument on the
// line.
func positionalAssignArg(args []string) (string, bool) {
	for i, arg := range args {
		if arg != "assign" {
			continue
		}
		var operands []string
		for _, rest := range args[i+1:] {
			if strings.HasPrefix(rest, "-") {
				continue
			}
			operands = append(operands, rest)
			if len(operands) == 2 {
				return operands[1], true
			}
		}
		return "", false
	}
	return "", false
}

func quoteAll(values []string) []string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, fmt.Sprintf("%q", value))
	}
	return quoted
}
