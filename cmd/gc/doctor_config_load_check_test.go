package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/gastownhall/gascity/internal/doctor"
)

func TestConfigLoadCheckReportsFailureInsteadOfDisappearing(t *testing.T) {
	result := newConfigLoadCheck(errors.New(`city import "core": locked but not cached`)).Run(nil)
	if result.Status != doctor.StatusError {
		t.Fatalf("status = %v, want error", result.Status)
	}
	if !strings.Contains(result.Message, "config-dependent checks did not run") {
		t.Errorf("message does not say which checks were lost: %q", result.Message)
	}
	if !strings.Contains(result.Message, "locked but not cached") {
		t.Errorf("message drops the underlying cause: %q", result.Message)
	}
}

func TestConfigLoadCheckPassesOnSuccess(t *testing.T) {
	result := newConfigLoadCheck(nil).Run(nil)
	if result.Status != doctor.StatusOK {
		t.Fatalf("status = %v, want ok", result.Status)
	}
}
