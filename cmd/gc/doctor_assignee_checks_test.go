package main

import (
	"strings"
	"testing"

	"github.com/gastownhall/gascity/internal/doctor"
)

func TestAssigneeResolvesCheckWarnsWhenRosterIsEmpty(t *testing.T) {
	// A config that failed to expand loads with zero agents. Reporting every
	// assignee as unroutable off that roster would be worse than the bug.
	result := newAssigneeResolvesCheck(nil, "/nonexistent", nil).Run(nil)
	if result.Status != doctor.StatusWarning {
		t.Fatalf("status = %v, want warning", result.Status)
	}
	if !strings.Contains(result.Message, "cannot be checked") {
		t.Errorf("message does not say the check could not answer: %q", result.Message)
	}
}

func TestIsOpenWorkStatus(t *testing.T) {
	for _, status := range []string{"open", "in_progress", "blocked", " open "} {
		if !isOpenWorkStatus(status) {
			t.Errorf("isOpenWorkStatus(%q) = false, want true", status)
		}
	}
	for _, status := range []string{"closed", "", "done"} {
		if isOpenWorkStatus(status) {
			t.Errorf("isOpenWorkStatus(%q) = true, want false", status)
		}
	}
}
