package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/gastownhall/gascity/internal/config"
)

// assigneeGateEscapeEnv lets a caller write an assignee the roster cannot
// confirm. It exists so the gate cannot wedge a seat that legitimately needs a
// target the static config does not describe; using it is a deliberate act that
// shows up in the command, not a silent default.
const assigneeGateEscapeEnv = "GC_ALLOW_UNRESOLVED_ASSIGNEE"

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
func extractAssigneeArgs(args []string) []string {
	var values []string
	var positional []string
	subcommand := ""
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
		case strings.HasPrefix(arg, "-"):
			// Unknown flag; its value, if separate, is not an assignee.
		default:
			if subcommand == "" {
				subcommand = arg
				continue
			}
			positional = append(positional, arg)
		}
	}
	// `bd assign <issue> <assignee>` writes the field positionally.
	if subcommand == "assign" && len(positional) >= 2 {
		values = append(values, positional[len(positional)-1])
	}
	return values
}

func quoteAll(values []string) []string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, fmt.Sprintf("%q", value))
	}
	return quoted
}
