package main

import (
	"fmt"

	"github.com/gastownhall/gascity/internal/doctor"
)

// configLoadCheck reports the deep config load that gates most of gc doctor.
//
// Roughly a dozen checks - config-valid, config-refs, pre-start-scripts, the
// MCP checks, beads-store, v2-routed-to-namespace, session-model and
// assignee-resolves - are registered only when this load succeeds. When it
// fails they are not skipped with a message; they are never registered, so
// they do not appear in the output at all and the summary line counts only
// what did run. Measured on the ds-research city 2026-09-15: `gc doctor`
// printed "26 passed, 2 warnings, 1 failed" with fourteen checks silently
// absent, while the separate city-config check reported the file as loaded.
//
// A health report that omits its own omissions is the failure it exists to
// catch, so the load result is now a check of its own.
type configLoadCheck struct {
	err error
}

func newConfigLoadCheck(err error) *configLoadCheck { return &configLoadCheck{err: err} }

func (c *configLoadCheck) Name() string { return "config-load" }

func (c *configLoadCheck) CanFix() bool { return false }

func (c *configLoadCheck) WarmupEligible() bool { return false }

func (c *configLoadCheck) Fix(_ *doctor.CheckContext) error { return nil }

func (c *configLoadCheck) Run(_ *doctor.CheckContext) *doctor.CheckResult {
	if c.err == nil {
		return okCheck(c.Name(), "full config load succeeded; config-dependent checks are registered")
	}
	return &doctor.CheckResult{
		Name:    c.Name(),
		Status:  doctor.StatusError,
		Message: fmt.Sprintf("full config load failed, so config-dependent checks did not run: %v", c.err),
		FixHint: "fix the config load (start with packv2-import-state and city-config), then rerun gc doctor; until then this report is incomplete",
	}
}
