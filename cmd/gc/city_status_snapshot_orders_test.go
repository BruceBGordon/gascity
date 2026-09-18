package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gastownhall/gascity/internal/beads"
	"github.com/gastownhall/gascity/internal/config"
	"github.com/gastownhall/gascity/internal/doctor"
	"github.com/gastownhall/gascity/internal/events"
	"github.com/gastownhall/gascity/internal/runtime"
)

// cityStatusOrderTestCity creates a minimal on-disk city (orders/ and
// formulas/ dirs) suitable for doctor.NewOrderFiringCurrentCheck and
// doctor.NewOrderOutcomeHealthyCheck. internal/doctor's own
// orderFiringTestCity fixture is unexported and lives in a different
// package, so its shape is reproduced here rather than imported.
func cityStatusOrderTestCity(t *testing.T) (string, *config.City) {
	t.Helper()
	cityPath := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cityPath, "orders"), 0o755); err != nil {
		t.Fatalf("mkdir orders: %v", err)
	}
	formulasDir := filepath.Join(cityPath, "formulas")
	if err := os.MkdirAll(formulasDir, 0o755); err != nil {
		t.Fatalf("mkdir formulas: %v", err)
	}
	return cityPath, &config.City{
		Workspace:     config.Workspace{Name: "city"},
		FormulaLayers: config.FormulaLayers{City: []string{formulasDir}},
	}
}

// writeCityStatusOrderTOML writes a minimal order definition. name becomes
// the order's bare Name (orders.ScanAll derives it from the filename stem),
// matching the convention cmd_doctor_order_firing_test.go and
// internal/doctor's writeOrderFiringTestOrder already use.
func writeCityStatusOrderTOML(t *testing.T, cityPath, name, trigger, interval string) {
	t.Helper()
	body := fmt.Sprintf("[order]\nexec = \"true\"\ntrigger = %q\n", trigger)
	if interval != "" {
		body += fmt.Sprintf("interval = %q\n", interval)
	}
	mustWriteDoctorOrderFiringTestFile(t, filepath.Join(cityPath, "orders", name+".toml"), body)
}

// writeCityStatusOrderEvents appends events to <cityPath>/.gc/events.jsonl,
// mirroring internal/doctor's writeOrderFiringTestEvents helper (unexported,
// different package, so reproduced here).
func writeCityStatusOrderEvents(t *testing.T, cityPath string, evts ...events.Event) {
	t.Helper()
	rec, err := events.NewFileRecorder(filepath.Join(cityPath, ".gc", "events.jsonl"), io.Discard)
	if err != nil {
		t.Fatalf("NewFileRecorder: %v", err)
	}
	for _, e := range evts {
		rec.Record(e)
	}
	if err := rec.Close(); err != nil {
		t.Fatalf("Close recorder: %v", err)
	}
}

func findCityStatusOrder(t *testing.T, orders []cityStatusOrder, name string) cityStatusOrder {
	t.Helper()
	for _, o := range orders {
		if o.Name == name {
			return o
		}
	}
	t.Fatalf("no Orders entry named %q in %+v", name, orders)
	return cityStatusOrder{}
}

// TestCityStatusOrdersEmptyOnHealthyCity pins the zero-config happy path:
// gc status must stay byte-for-byte unchanged (no "Orders:" section, empty
// Orders slice) when nothing is overdue, failing, or gate-suppressed. Every
// other test in this file adds a positive signal; this one proves silence
// when there is nothing to report, per this bead's exit contract ("healthy
// city -> gc status output unchanged").
func TestCityStatusOrdersEmptyOnHealthyCity(t *testing.T) {
	cityPath, cfg := cityStatusOrderTestCity(t)
	// No order files and no events.jsonl at all: scanOrderFiringCurrentOrders
	// and consecutiveOrderFailures both treat "nothing monitored" as
	// StatusOK, mirroring internal/doctor's own
	// TestOrderOutcomeHealthyReportsCleanCity.

	sp := runtime.NewFake()
	store := beads.NewMemStore()
	var stderr bytes.Buffer
	snapshot := collectCityStatusSnapshot(sp, cfg, cityPath, store, &stderr)

	if len(snapshot.Orders) != 0 {
		t.Fatalf("Orders = %+v, want empty on a healthy city", snapshot.Orders)
	}

	var stdout bytes.Buffer
	renderCityStatusText(snapshot, newDrainOps(sp), &stdout)
	if strings.Contains(stdout.String(), "Orders:") {
		t.Fatalf("stdout = %q, want no Orders section on a healthy city", stdout.String())
	}
}

// TestCityStatusOrdersSurfacesStaleFiring cross-checks gc status against
// gc doctor's own order-firing-current check on the same fixture: an order
// whose last fire crosses classifyOrderFiring's 3x-interval "CRITICAL:
// stale" threshold. gc status must carry the exact Status/Severity/Message
// doctor.NewOrderFiringCurrentCheck.Run produces -- not a re-derived
// approximation -- so the two commands can never disagree (this bead's
// core zero-config requirement).
func TestCityStatusOrdersSurfacesStaleFiring(t *testing.T) {
	cityPath, cfg := cityStatusOrderTestCity(t)
	writeCityStatusOrderTOML(t, cityPath, "stale-order", "cooldown", "5m")

	now := time.Now().UTC()
	writeCityStatusOrderEvents(t, cityPath,
		events.Event{Type: events.OrderFired, Subject: "stale-order", Ts: now.Add(-20 * time.Minute)},
	)

	wantResult := doctor.NewOrderFiringCurrentCheck(cfg, cityPath).Run(&doctor.CheckContext{CityPath: cityPath})
	if wantResult.Status != doctor.StatusError {
		t.Fatalf("fixture sanity: doctor order-firing-current status = %v, want StatusError (fixture drifted off the 3x-interval threshold)", wantResult.Status)
	}

	sp := runtime.NewFake()
	store := beads.NewMemStore()
	var stderr bytes.Buffer
	snapshot := collectCityStatusSnapshot(sp, cfg, cityPath, store, &stderr)

	order := findCityStatusOrder(t, snapshot.Orders, wantResult.Name)
	if order.Status != wantResult.Status {
		t.Fatalf("Orders[%s].Status = %v, want %v (from gc doctor's own Run)", wantResult.Name, order.Status, wantResult.Status)
	}
	if order.Severity != wantResult.Severity {
		t.Fatalf("Orders[%s].Severity = %v, want %v", wantResult.Name, order.Severity, wantResult.Severity)
	}
	if order.Message != wantResult.Message {
		t.Fatalf("Orders[%s].Message = %q, want %q (gc status and gc doctor must never disagree)", wantResult.Name, order.Message, wantResult.Message)
	}

	var stdout bytes.Buffer
	renderCityStatusText(snapshot, newDrainOps(sp), &stdout)
	if !strings.Contains(stdout.String(), "Orders:") {
		t.Fatalf("stdout missing Orders section:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), wantResult.Message) {
		t.Fatalf("stdout = %q, want it to contain doctor's message %q", stdout.String(), wantResult.Message)
	}
}

// TestCityStatusOrdersSurfacesRepeatedFailures cross-checks gc status
// against gc doctor's order-outcome-healthy check: an order with three
// consecutive order.failed events crosses classifyOrderOutcome's
// failure-streak threshold. Mirrors internal/doctor's
// TestOrderOutcomeHealthy_FlagsRigScopedOrderFailureStreak fixture shape
// (three failures at -18h/-12h/-6h, well clear of any controller-start
// grace window) without rig scoping, since gc status's Orders surfacing
// does not depend on rig scope.
func TestCityStatusOrdersSurfacesRepeatedFailures(t *testing.T) {
	cityPath, cfg := cityStatusOrderTestCity(t)
	writeCityStatusOrderTOML(t, cityPath, "flaky-order", "cooldown", "5m")

	now := time.Now().UTC()
	writeCityStatusOrderEvents(t, cityPath,
		events.Event{Type: events.OrderFailed, Subject: "flaky-order", Ts: now.Add(-18 * time.Hour), Message: "exit status 1"},
		events.Event{Type: events.OrderFailed, Subject: "flaky-order", Ts: now.Add(-12 * time.Hour), Message: "exit status 1"},
		events.Event{Type: events.OrderFailed, Subject: "flaky-order", Ts: now.Add(-6 * time.Hour), Message: "exit status 1"},
	)

	wantResult := doctor.NewOrderOutcomeHealthyCheck(cfg, cityPath).Run(&doctor.CheckContext{CityPath: cityPath})
	if wantResult.Status != doctor.StatusWarning {
		t.Fatalf("fixture sanity: doctor order-outcome-healthy status = %v, want StatusWarning (fixture drifted off the 3-consecutive-failure threshold)", wantResult.Status)
	}

	sp := runtime.NewFake()
	store := beads.NewMemStore()
	var stderr bytes.Buffer
	snapshot := collectCityStatusSnapshot(sp, cfg, cityPath, store, &stderr)

	order := findCityStatusOrder(t, snapshot.Orders, wantResult.Name)
	if order.Message != wantResult.Message {
		t.Fatalf("Orders[%s].Message = %q, want %q (gc status and gc doctor must never disagree)", wantResult.Name, order.Message, wantResult.Message)
	}
	if order.Severity != doctor.SeverityAdvisory {
		t.Fatalf("Orders[%s].Severity = %v, want SeverityAdvisory (a failing order must not gate gc status)", wantResult.Name, order.Severity)
	}

	var stdout bytes.Buffer
	renderCityStatusText(snapshot, newDrainOps(sp), &stdout)
	if !strings.Contains(stdout.String(), wantResult.Message) {
		t.Fatalf("stdout = %q, want it to contain doctor's message %q", stdout.String(), wantResult.Message)
	}
}

// TestCityStatusOrdersSurfacesGateSuppression is the first consumer of
// events.OrderSuppressed (emitted by order_dispatch.go's
// noteOpenWorkSuppressed) anywhere in the repo. Neither doctor check reads
// this event type, so this signal only reaches gc status through the new
// bounded tail-read of events.jsonl this bead adds.
func TestCityStatusOrdersSurfacesGateSuppression(t *testing.T) {
	cityPath, cfg := cityStatusOrderTestCity(t)

	now := time.Now().UTC()
	firstSuppressed := now.Add(-90 * time.Minute).Format(time.RFC3339)
	payload := events.OrderSuppressedPayload{
		OrderName:       "watched-order",
		Consecutive:     12,
		FirstSuppressed: firstSuppressed,
		SuppressedForMS: 90 * 60 * 1000,
	}
	writeCityStatusOrderEvents(t, cityPath,
		events.Event{
			Type:    events.OrderSuppressed,
			Actor:   "controller",
			Subject: "watched-order",
			Ts:      now,
			Message: "open-work gate has suppressed this order for 12 consecutive dispatch checks",
			Payload: events.OrderSuppressedPayloadJSON(payload),
		},
	)

	sp := runtime.NewFake()
	store := beads.NewMemStore()
	var stderr bytes.Buffer
	snapshot := collectCityStatusSnapshot(sp, cfg, cityPath, store, &stderr)

	order := findCityStatusOrder(t, snapshot.Orders, "watched-order")
	if order.Consecutive != 12 {
		t.Fatalf("Orders[watched-order].Consecutive = %d, want 12", order.Consecutive)
	}
	if order.FirstSuppressed != firstSuppressed {
		t.Fatalf("Orders[watched-order].FirstSuppressed = %q, want %q", order.FirstSuppressed, firstSuppressed)
	}

	var stdout bytes.Buffer
	renderCityStatusText(snapshot, newDrainOps(sp), &stdout)
	if !strings.Contains(stdout.String(), "watched-order") {
		t.Fatalf("stdout missing suppressed order name:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "12") {
		t.Fatalf("stdout missing consecutive-suppression count:\n%s", stdout.String())
	}
}
