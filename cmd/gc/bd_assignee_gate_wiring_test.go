package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gastownhall/gascity/internal/config"
)

// gateWiringCity stands up the smallest city that reaches the assignee gate in
// doBd: a bd-backed rig, plus a stub bd on PATH that records whether it ran.
// The stub is the point of the fixture. A refused write must never reach bd,
// and a permitted one must, so "the gate fired" is observed from the outside
// rather than asserted about the helper in isolation.
func gateWiringCity(t *testing.T) (ranMarker string) {
	t.Helper()
	origCityFlag, origRigFlag, origProbe := cityFlag, rigFlag, bdBeadExists
	t.Cleanup(func() {
		cityFlag, rigFlag, bdBeadExists = origCityFlag, origRigFlag, origProbe
	})
	bdBeadExists = func(_ string, _ *config.City, _ execStoreTarget, _ string) bool { return true }
	cityFlag, rigFlag = "", ""

	cityDir := t.TempDir()
	port := strconv.Itoa(writeReachableManagedDoltState(t, cityDir))
	_ = port
	rigDir := filepath.Join(cityDir, "repo")
	if err := os.MkdirAll(filepath.Join(rigDir, ".beads"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cityDir, "city.toml"), []byte(`[workspace]
name = "demo"

[[rigs]]
name = "repo"
path = "repo"
prefix = "repo"

[[agent]]
name = "polecat"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rigDir, ".beads", "config.yaml"), []byte(`issue_prefix: repo
gc.endpoint_origin: inherited_city
gc.endpoint_status: verified
dolt.auto-start: false
`), 0o644); err != nil {
		t.Fatal(err)
	}

	marker := filepath.Join(t.TempDir(), "bd-ran.txt")
	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "bd"), []byte(`#!/bin/sh
printf '%s' "$*" > "${BD_RAN_MARKER}"
`), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("BD_RAN_MARKER", marker)
	t.Setenv("GC_CITY_PATH", cityDir)
	t.Setenv(assigneeGateEscapeEnv, "")
	return marker
}

func bdRan(t *testing.T, marker string) bool {
	t.Helper()
	_, err := os.Stat(marker)
	return err == nil
}

// The gate is wired into doBd, not merely available to it. Without this the
// call site could be dropped or reordered in a refactor and every existing
// test would stay green.
func TestDoBdRefusesUnroutableAssigneeBeforeInvokingBd(t *testing.T) {
	for _, args := range [][]string{
		{"update", "repo-1", "--assignee", "goal-5-temporal"},
		{"update", "repo-1", "--assignee=goal-5-temporal"},
		{"update", "repo-1", "-a", "goal-5-temporal"},
		{"assign", "repo-1", "goal-5-temporal"},
		// A global flag carrying its own value pushes a non-flag token ahead
		// of the subcommand. The gate must still find the positional assignee.
		{"--db", "/tmp/x.db", "assign", "repo-1", "goal-5-temporal"},
	} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			marker := gateWiringCity(t)
			var stdout, stderr bytes.Buffer
			if got := doBd(args, &stdout, &stderr); got == 0 {
				t.Fatalf("doBd(%v) = 0, want refusal; stderr=%q", args, stderr.String())
			}
			if !strings.Contains(stderr.String(), "matches no configured agent") {
				t.Errorf("refusal did not explain itself: %q", stderr.String())
			}
			if bdRan(t, marker) {
				t.Errorf("bd was invoked despite the refusal")
			}
		})
	}
}

// The negative rail. A routable assignee must pass straight through, or the
// gate is just an outage.
func TestDoBdPermitsRoutableAssignee(t *testing.T) {
	marker := gateWiringCity(t)
	var stdout, stderr bytes.Buffer
	if got := doBd([]string{"update", "repo-1", "--assignee", "polecat"}, &stdout, &stderr); got != 0 {
		t.Fatalf("doBd() = %d for a routable assignee; stderr=%q", got, stderr.String())
	}
	if !bdRan(t, marker) {
		t.Errorf("bd was not invoked for a routable assignee")
	}
}

// The escape hatch is reachable from the real call site, and a false spelling
// does not silently disable the gate.
func TestDoBdEscapeHatchAppliesAtTheCallSite(t *testing.T) {
	t.Run("truthy bypasses", func(t *testing.T) {
		marker := gateWiringCity(t)
		t.Setenv(assigneeGateEscapeEnv, "1")
		var stdout, stderr bytes.Buffer
		if got := doBd([]string{"assign", "repo-1", "goal-5-temporal"}, &stdout, &stderr); got != 0 {
			t.Fatalf("doBd() = %d with the escape hatch set; stderr=%q", got, stderr.String())
		}
		if !bdRan(t, marker) {
			t.Errorf("bd was not invoked with the escape hatch set")
		}
	})
	t.Run("false spelling does not bypass", func(t *testing.T) {
		marker := gateWiringCity(t)
		t.Setenv(assigneeGateEscapeEnv, "0")
		var stdout, stderr bytes.Buffer
		if got := doBd([]string{"assign", "repo-1", "goal-5-temporal"}, &stdout, &stderr); got == 0 {
			t.Fatalf("GC_ALLOW_UNRESOLVED_ASSIGNEE=0 disabled the gate; stderr=%q", stderr.String())
		}
		if bdRan(t, marker) {
			t.Errorf("bd was invoked although the bypass was set to a false value")
		}
	})
}
