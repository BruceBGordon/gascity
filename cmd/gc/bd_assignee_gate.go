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

// assigneeWriteSubcommands are the bd subcommands where --assignee/-a WRITES
// the assignee field. On "list" and "ready" the same flag is a read-only
// filter (bd list --help: "-a, --assignee string  Filter by assignee"; see
// internal/bdflags valueFlagsBySub["list"/"ready"]), and refusing those
// refuses the exact investigation this gate exists to enable. Sourced from
// bdflags' own per-subcommand manifest rather than hand-picked, so this set
// cannot drift from what bd actually accepts as a write.
var assigneeWriteSubcommands = map[string]bool{
	"create":   true,
	"update":   true,
	"mol pour": true,
}

// extractAssigneeArgs pulls every value a bd invocation would write to the
// assignee field, in both `--assignee=x` and `--assignee x` spellings, plus the
// positional form of `bd assign <id> <assignee>`.
//
// The flag form is scoped to assigneeWriteSubcommands via bdByIDSubcommand,
// the same subcommand locator the by-ID door uses, so `bd list --assignee X`
// and `bd ready --assignee X` — read-only filters — are never treated as a
// write.
//
// The positional form is found separately by locating the literal `assign`
// token rather than by taking the first non-flag argument as the subcommand.
// A global flag that carries a separate value (`bd --db /path assign gc-1
// polecat-4`) puts a non-flag token ahead of the subcommand, and the
// positional reading used to land on that value instead, silently skipping
// the check for exactly the invocation shape it exists to catch.
func extractAssigneeArgs(args []string) []string {
	var values []string
	if sub, subArgs, resolved := bdByIDSubcommand(args); resolved && assigneeWriteSubcommands[sub] {
		for i := 0; i < len(subArgs); i++ {
			arg := subArgs[i]
			switch {
			case arg == "--assignee" || arg == "-a":
				if i+1 < len(subArgs) {
					values = append(values, subArgs[i+1])
					i++
				}
			case strings.HasPrefix(arg, "--assignee="):
				values = append(values, strings.TrimPrefix(arg, "--assignee="))
			}
		}
	}
	if positional, ok := positionalAssignArg(args); ok {
		values = append(values, positional)
	}
	return values
}

// bdValuedGlobalFlags are the bd flags that consume the following argument,
// taken from `bd --help`. A flag outside this set is treated as a boolean; if
// bd gains a valued flag that is not listed here the effect is a missed check,
// never a refused write.
var bdValuedGlobalFlags = map[string]struct{}{
	"--actor": {}, "--database": {}, "--db": {},
	"-C": {}, "--directory": {},
	"--dolt-auto-commit": {}, "--mem-profile": {},
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
		// Distinguish the subcommand from a flag value that happens to be the
		// word "assign" (`bd list --label assign foo bar`). A subcommand is
		// never preceded directly by a flag; a flag value always is. Where
		// that reading is wrong the result is a missed check rather than a
		// refused write, which is the right way round: a false refusal on an
		// unrelated command teaches operators to bypass the gate.
		if i > 0 && strings.HasPrefix(args[i-1], "-") {
			continue
		}
		var operands []string
		rest := args[i+1:]
		for j := 0; j < len(rest); j++ {
			tok := rest[j]
			if strings.HasPrefix(tok, "-") {
				// A valued flag placed after the subcommand would otherwise
				// donate its value to the operand list, and the gate would
				// check the actor name instead of the assignee.
				if _, valued := bdValuedGlobalFlags[tok]; valued && !strings.Contains(tok, "=") {
					j++
				}
				continue
			}
			operands = append(operands, tok)
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
