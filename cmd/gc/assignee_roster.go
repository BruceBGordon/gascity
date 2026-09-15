package main

import (
	"regexp"
	"strings"

	"github.com/gastownhall/gascity/internal/config"
)

// assigneeRoster answers one question: can this assignee name ever be routed to?
//
// The city has always had this information — build_desired_state.go consults it
// on every reconcile — but it only ever asked the question in the direction
// "which beads does this spec own?". A bead whose assignee matches no spec is
// never visited by that loop, so an unroutable owner has never been
// representable as a finding. Everything below exists to ask it the other way.
type assigneeRoster struct {
	// exact holds every name that is directly slingable: named-session
	// identities (short and qualified) and configured agent names.
	exact map[string]struct{}
	// poolStems holds agent names that materialize numbered pool instances,
	// so "polecat-4" resolves via the stem "polecat".
	poolStems map[string]struct{}
}

// poolInstanceSuffix matches the numbered (and optionally lettered) suffix the
// reconciler appends to a pool agent name when it materializes an instance.
var poolInstanceSuffix = regexp.MustCompile(`-\d+[a-z]?$`)

// sessionIdentityShape matches the opaque identifiers the runtime assigns:
// session bead IDs (dr-…, gc-…) and generated adhoc/auto session names. These
// are legitimate assignees that exist only at runtime, so a static roster
// cannot confirm them and must not reject them.
var sessionIdentityShape = regexp.MustCompile(`^(?:[a-z]{2,4}-[0-9a-z]{4,}|.*-adhoc-[0-9a-f]{6,}|.*-auto-\d+)$`)

// reservedAssignees are meaningful owners that are not city configuration:
// a person, and the mailbox the mayor seat reads.
var reservedAssignees = map[string]struct{}{
	"human": {},
	"mayor": {},
}

func newAssigneeRoster(cfg *config.City) *assigneeRoster {
	r := &assigneeRoster{
		exact:     map[string]struct{}{},
		poolStems: map[string]struct{}{},
	}
	if cfg == nil {
		return r
	}
	for i := range cfg.NamedSessions {
		r.addExact(cfg.NamedSessions[i].IdentityName())
		r.addExact(cfg.NamedSessions[i].QualifiedName())
	}
	for i := range cfg.Agents {
		name := strings.TrimSpace(cfg.Agents[i].Name)
		r.addExact(name)
		r.addExact(cfg.Agents[i].QualifiedName())
		if name != "" {
			r.poolStems[name] = struct{}{}
		}
	}
	return r
}

// Empty reports whether the roster knows of no routable target at all. A roster
// built from a config that failed to expand its packs looks exactly like a city
// with no agents, and would then declare every assignee in the fleet
// unroutable. Callers must treat an empty roster as "cannot answer" rather than
// "answer is no" — the same failure this whole change exists to prevent, one
// layer up.
func (r *assigneeRoster) Empty() bool {
	return len(r.exact) == 0 && len(r.poolStems) == 0
}

func (r *assigneeRoster) addExact(name string) {
	name = strings.TrimSpace(name)
	if name != "" {
		r.exact[name] = struct{}{}
	}
}

// Resolves reports whether assignee names something the city could route work
// to. An empty assignee resolves: unowned is a valid state, distinct from
// owned-by-nobody, and is reported separately by the reconciler.
func (r *assigneeRoster) Resolves(assignee string) bool {
	assignee = strings.TrimSpace(assignee)
	if assignee == "" {
		return true
	}
	if _, ok := reservedAssignees[assignee]; ok {
		return true
	}
	// A runtime identity cannot be confirmed against static config. Treating
	// an unconfirmable name as a finding would bury the real ones.
	if sessionIdentityShape.MatchString(assignee) {
		return true
	}
	for _, candidate := range assigneeCandidates(assignee) {
		if _, ok := r.exact[candidate]; ok {
			return true
		}
		if _, ok := r.poolStems[poolInstanceSuffix.ReplaceAllString(candidate, "")]; ok {
			return true
		}
	}
	return false
}

// assigneeCandidates expands the spellings the same owner is written in across
// the fleet: bare, qualified with a rig or directory prefix, and absolute paths
// (the file store records `/home/ds/gas-city/goal-5-temporal` for what the bd
// store records as `goal-5-temporal`).
func assigneeCandidates(assignee string) []string {
	candidates := []string{assignee}
	if idx := strings.LastIndex(assignee, "/"); idx >= 0 && idx+1 < len(assignee) {
		candidates = append(candidates, assignee[idx+1:])
	}
	if idx := strings.LastIndex(assignee, "."); idx >= 0 && idx+1 < len(assignee) {
		candidates = append(candidates, assignee[idx+1:])
	}
	return candidates
}
