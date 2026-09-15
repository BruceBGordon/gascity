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
		"",                   // unowned is a valid state
		"goal-4-context",     // named session
		"polecat",            // pool agent
		"polecat-4",          // materialized pool instance
		"city-infra-worker",  // agent
		"human",              // a person
		"mayor",              // reserved mailbox
		"dr-toegp",           // session bead id
		"gc-4cyi0a",          // session bead id
		"claude-1-adhoc-6f649101fd", // runtime adhoc session
		"claude-auto-2",      // runtime auto session
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
