package main

import (
	"io"
	"strings"
	"testing"

	"github.com/gastownhall/gascity/internal/config"
)

func testRosterCity() *config.City {
	return &config.City{
		Agents: []config.Agent{
			{Name: "polecat"},
			{Name: "city-infra-worker"},
		},
		NamedSessions: []config.NamedSession{
			{Name: "goal-4-context"},
			{Name: "mayor"},
		},
	}
}

func TestAssigneeRosterRejectsUnconfiguredTargets(t *testing.T) {
	roster := newAssigneeRoster(testRosterCity())

	// The names the 2026-09-15 audit found holding open work hostage.
	for _, assignee := range []string{
		"goal-5-temporal",
		"/home/ds/gas-city/goal-5-temporal",
		"controller",
		"pr-pipeline-review-17",
	} {
		if roster.Resolves(assignee) {
			t.Errorf("assignee %q resolved, want unresolvable", assignee)
		}
	}
}

func TestAssigneeRosterAcceptsRoutableTargets(t *testing.T) {
	roster := newAssigneeRoster(testRosterCity())

	for _, assignee := range []string{
		"",                          // unowned is a valid state
		"goal-4-context",            // named session
		"polecat",                   // pool agent
		"polecat-4",                 // materialized pool instance
		"city-infra-worker",         // agent
		"human",                     // a person
		"mayor",                     // reserved mailbox
		"dr-toegp",                  // session bead id
		"gc-4cyi0a",                 // session bead id
		"claude-1-adhoc-6f649101fd", // runtime adhoc session
		"claude-auto-2",             // runtime auto session
	} {
		if !roster.Resolves(assignee) {
			t.Errorf("assignee %q did not resolve, want routable", assignee)
		}
	}
}

func TestAssigneeRosterEmptyIsReportedNotEnforced(t *testing.T) {
	// A config that fails to expand its packs loads with zero agents, which is
	// indistinguishable from a city with none. Measured on the real city
	// 2026-09-15: `gc doctor` reported `city.toml loaded (0 agents, 11 rigs)`
	// while packv2-import-state was failing. Enforcing against that roster
	// would have declared every assignee in the fleet unroutable.
	roster := newAssigneeRoster(nil)
	if !roster.Empty() {
		t.Fatal("roster built from nil config does not report Empty")
	}
	if newAssigneeRoster(testRosterCity()).Empty() {
		t.Fatal("a populated roster must not report Empty")
	}

	// The write gate must pass the write through, not block it.
	if err := checkBdAssigneeArgs(nil, []string{"update", "dr-1", "--assignee", "anything"}, io.Discard); err != nil {
		t.Fatalf("gate enforced against an empty roster: %v", err)
	}
}

func TestExtractAssigneeArgsCoversEverySpelling(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want []string
	}{
		{"separate flag", []string{"update", "dr-1", "--assignee", "ghost"}, []string{"ghost"}},
		{"joined flag", []string{"update", "dr-1", "--assignee=ghost"}, []string{"ghost"}},
		{"short flag", []string{"create", "t", "-a", "ghost"}, []string{"ghost"}},
		{"positional assign", []string{"assign", "dr-1", "ghost"}, []string{"ghost"}},
		{"no assignee", []string{"list", "--status", "open"}, nil},
		{"status value is not an assignee", []string{"list", "--status", "open", "--json"}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractAssigneeArgs(tc.args)
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("extractAssigneeArgs(%v) = %v, want %v", tc.args, got, tc.want)
			}
		})
	}
}

func TestCheckBdAssigneeArgsGate(t *testing.T) {
	cfg := testRosterCity()

	err := checkBdAssigneeArgs(cfg, []string{"update", "dr-1", "--assignee", "goal-5-temporal"}, nil)
	if err == nil {
		t.Fatal("gate allowed an unresolvable assignee")
	}
	if !strings.Contains(err.Error(), assigneeGateEscapeEnv) {
		t.Errorf("rejection does not name the escape hatch: %v", err)
	}

	if err := checkBdAssigneeArgs(cfg, []string{"update", "dr-1", "--assignee", "polecat-2"}, nil); err != nil {
		t.Fatalf("gate rejected a routable assignee: %v", err)
	}
	if err := checkBdAssigneeArgs(cfg, []string{"list", "--status", "open"}, nil); err != nil {
		t.Fatalf("gate rejected a command that writes no assignee: %v", err)
	}
}

// rosterCityWithPrefixes declares the bead ID prefixes the city issues, so the
// roster can tell a session identity from a misspelled agent name.
func rosterCityWithPrefixes() *config.City {
	c := testRosterCity()
	c.Workspace.Prefix = "gc"
	c.Rigs = []config.Rig{{Name: "research", Path: "research", Prefix: "dr"}}
	return c
}

// A session identity is named after a bead the city issued. Checking the shape
// alone (2-4 letters, hyphen, alphanumerics) also matches an ordinary typo of
// an agent name, which would wave through the exact error class this exists to
// catch.
func TestAssigneeRosterDistinguishesSessionIDsFromTypos(t *testing.T) {
	roster := newAssigneeRoster(rosterCityWithPrefixes())

	for _, assignee := range []string{"dr-huhn", "gc-818bx", "dr-a95w9", "repo-adhoc-1a2b3c", "worker-auto-7"} {
		if !roster.Resolves(assignee) {
			t.Errorf("runtime identity %q was rejected; a static roster cannot confirm these", assignee)
		}
	}
	// Same shape, prefix the city never issues.
	for _, assignee := range []string{"poly-cat1", "xy-1234", "abc-9999"} {
		if roster.Resolves(assignee) {
			t.Errorf("assignee %q resolved as a session identity, but no city prefix issues it", assignee)
		}
	}
}

// With no prefixes to check against there is nothing to be right about, so any
// bead-shaped name is accepted. Same cannot-answer posture as Empty().
func TestAssigneeRosterAcceptsAnyIDShapeWhenNoPrefixesDeclared(t *testing.T) {
	c := &config.City{Agents: []config.Agent{{Name: "polecat"}}}
	if !newAssigneeRoster(c).Resolves("poly-cat1") {
		t.Error("bead-shaped name rejected although the city declares no prefixes")
	}
}

// A pool instance is not always <agent>-<slot>: an agent declaring a namepool
// materializes instances under the declared names, which share nothing with
// the stem.
func TestAssigneeRosterResolvesNamepoolInstances(t *testing.T) {
	c := testRosterCity()
	c.Agents = append(c.Agents, config.Agent{Name: "herder", NamepoolNames: []string{"rivet", "gasket"}})
	roster := newAssigneeRoster(c)

	for _, assignee := range []string{"rivet", "gasket", "herder-2"} {
		if !roster.Resolves(assignee) {
			t.Errorf("pool instance %q was rejected; it is routable", assignee)
		}
	}
}

func TestPositionalAssignArgSurvivesLeadingGlobalFlagValues(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"assign", "gc-1", "polecat"}, "polecat"},
		{[]string{"--db", "/tmp/x.db", "assign", "gc-1", "polecat"}, "polecat"},
		{[]string{"--db", "/tmp/x.db", "assign", "--force", "gc-1", "polecat"}, "polecat"},
	} {
		got, ok := positionalAssignArg(tc.args)
		if !ok || got != tc.want {
			t.Errorf("positionalAssignArg(%v) = (%q, %v), want (%q, true)", tc.args, got, ok, tc.want)
		}
	}
	// Not the assign subcommand, so there is no positional assignee to take.
	for _, args := range [][]string{
		{"list", "-s", "open"},
		{"update", "gc-1", "--assignee", "polecat"},
		{"assign", "gc-1"},
	} {
		if got, ok := positionalAssignArg(args); ok {
			t.Errorf("positionalAssignArg(%v) = (%q, true), want no positional assignee", args, got)
		}
	}
}
