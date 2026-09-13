package main

import (
	"io"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gastownhall/gascity/internal/config"
	"github.com/gastownhall/gascity/internal/convergence"
	"github.com/gastownhall/gascity/internal/fsys"
)

func TestResolveTemplatePrependsGCBinDirToPATH(t *testing.T) {
	cityPath := t.TempDir()
	writeTemplateResolveCityConfig(t, cityPath, "file")
	sep := string(os.PathListSeparator)
	t.Setenv("PATH", "/opt/homebrew/bin"+sep+"/usr/bin")

	params := &agentBuildParams{
		cityName:   "city",
		cityPath:   cityPath,
		workspace:  &config.Workspace{Provider: "test"},
		providers:  map[string]config.ProviderSpec{"test": {Command: "echo", PromptMode: "none"}},
		lookPath:   func(string) (string, error) { return "/bin/echo", nil },
		fs:         fsys.OSFS{},
		beaconTime: time.Unix(0, 0),
		beadNames:  make(map[string]string),
		stderr:     io.Discard,
	}

	agent := &config.Agent{Name: "runner"}
	tp, err := resolveTemplate(params, agent, agent.QualifiedName(), nil)
	if err != nil {
		t.Fatalf("resolveTemplate: %v", err)
	}

	gcBin := tp.Env["GC_BIN"]
	if gcBin == "" {
		t.Fatal("GC_BIN is empty")
	}
	wantDir := filepath.Dir(gcBin)
	parts := strings.Split(tp.Env["PATH"], sep)
	if len(parts) == 0 || parts[0] != wantDir {
		t.Fatalf("PATH first entry = %q, want gc bin dir %q (PATH=%q)", parts[0], wantDir, tp.Env["PATH"])
	}
	count := 0
	for _, part := range parts {
		if part == wantDir {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("gc bin dir %q should appear exactly once, found %d in PATH=%q", wantDir, count, tp.Env["PATH"])
	}
}

func TestResolveTemplatePrependsGCBinDirToConfiguredAgentPATH(t *testing.T) {
	cityPath := t.TempDir()
	writeTemplateResolveCityConfig(t, cityPath, "file")
	sep := string(os.PathListSeparator)
	t.Setenv("PATH", "/opt/homebrew/bin"+sep+"/usr/bin")

	params := &agentBuildParams{
		cityName:   "city",
		cityPath:   cityPath,
		workspace:  &config.Workspace{Provider: "test"},
		providers:  map[string]config.ProviderSpec{"test": {Command: "echo", PromptMode: "none"}},
		lookPath:   func(string) (string, error) { return "/bin/echo", nil },
		fs:         fsys.OSFS{},
		beaconTime: time.Unix(0, 0),
		beadNames:  make(map[string]string),
		stderr:     io.Discard,
	}

	configuredPATH := "/custom/tools" + sep + "/usr/local/bin"
	agent := &config.Agent{
		Name: "runner",
		Env:  map[string]string{"PATH": configuredPATH},
	}
	tp, err := resolveTemplate(params, agent, agent.QualifiedName(), nil)
	if err != nil {
		t.Fatalf("resolveTemplate: %v", err)
	}

	gcBin := tp.Env["GC_BIN"]
	if gcBin == "" {
		t.Fatal("GC_BIN is empty")
	}
	wantDir := filepath.Dir(gcBin)
	parts := strings.Split(tp.Env["PATH"], sep)
	wantPrefix := []string{wantDir, "/custom/tools", "/usr/local/bin"}
	if len(parts) < len(wantPrefix) {
		t.Fatalf("PATH=%q has fewer entries than expected prefix %v", tp.Env["PATH"], wantPrefix)
	}
	for i, want := range wantPrefix {
		if parts[i] != want {
			t.Fatalf("PATH entry %d = %q, want %q (PATH=%q)", i, parts[i], want, tp.Env["PATH"])
		}
	}
}

func TestResolveTemplateUsesTrustedRuntimeRootForControlTraceDefault(t *testing.T) {
	cityPath := t.TempDir()
	writeTemplateResolveCityConfig(t, cityPath, "file")
	customRuntimeDir := filepath.Join(t.TempDir(), "runtime-root")
	t.Setenv("GC_CITY_PATH", cityPath)
	t.Setenv("GC_CITY_RUNTIME_DIR", customRuntimeDir)

	params := &agentBuildParams{
		cityName:   "city",
		cityPath:   cityPath,
		workspace:  &config.Workspace{Provider: "test"},
		providers:  map[string]config.ProviderSpec{"test": {Command: "echo", PromptMode: "none"}},
		lookPath:   func(string) (string, error) { return "/bin/echo", nil },
		fs:         fsys.OSFS{},
		beaconTime: time.Unix(0, 0),
		beadNames:  make(map[string]string),
		stderr:     io.Discard,
	}

	agent := &config.Agent{Name: "runner"}
	tp, err := resolveTemplate(params, agent, agent.QualifiedName(), nil)
	if err != nil {
		t.Fatalf("resolveTemplate: %v", err)
	}

	if got := tp.Env["GC_CITY_RUNTIME_DIR"]; got != customRuntimeDir {
		t.Fatalf("GC_CITY_RUNTIME_DIR = %q, want %q", got, customRuntimeDir)
	}
	wantTraceDefault := filepath.Join(customRuntimeDir, "control-dispatcher-trace.log")
	if got := tp.Env["GC_CONTROL_DISPATCHER_TRACE_DEFAULT"]; got != wantTraceDefault {
		t.Fatalf("GC_CONTROL_DISPATCHER_TRACE_DEFAULT = %q, want %q", got, wantTraceDefault)
	}
}

func TestResolveTemplateUsesTrustedRuntimeRootForControlDispatcherTraceDefault(t *testing.T) {
	cityPath := t.TempDir()
	writeTemplateResolveCityConfig(t, cityPath, "file")
	customRuntimeDir := filepath.Join(t.TempDir(), "runtime-root")
	t.Setenv("GC_CITY_PATH", cityPath)
	t.Setenv("GC_CITY_RUNTIME_DIR", customRuntimeDir)

	params := &agentBuildParams{
		cityName:   "city",
		cityPath:   cityPath,
		workspace:  &config.Workspace{Provider: "test"},
		providers:  map[string]config.ProviderSpec{"test": {Command: "echo", PromptMode: "none"}},
		lookPath:   func(string) (string, error) { return "/bin/echo", nil },
		fs:         fsys.OSFS{},
		beaconTime: time.Unix(0, 0),
		beadNames:  make(map[string]string),
		stderr:     io.Discard,
	}

	qualifiedName := "app/" + config.ControlDispatcherAgentName
	agent := &config.Agent{Name: config.ControlDispatcherAgentName, Dir: "app"}
	tp, err := resolveTemplate(params, agent, qualifiedName, nil)
	if err != nil {
		t.Fatalf("resolveTemplate: %v", err)
	}

	if got := tp.Env["GC_CITY_RUNTIME_DIR"]; got != customRuntimeDir {
		t.Fatalf("GC_CITY_RUNTIME_DIR = %q, want %q", got, customRuntimeDir)
	}
	wantTraceDefault := filepath.Join(customRuntimeDir, "app--control-dispatcher-trace.log")
	if got := tp.Env["GC_CONTROL_DISPATCHER_TRACE_DEFAULT"]; got != wantTraceDefault {
		t.Fatalf("GC_CONTROL_DISPATCHER_TRACE_DEFAULT = %q, want %q", got, wantTraceDefault)
	}
}

// TestResolveTemplateInjectsPerDispatcherTraceDefault asserts that
// resolveTemplate produces a per-dispatcher GC_CONTROL_DISPATCHER_TRACE_DEFAULT
// in agentEnv for control-dispatcher agents (closes #1650). The override
// goes in agentEnv (last in mergeEnv) so it deterministically wins over
// the uniform city-level default seeded by cityRuntimeEnvMapForCity.
func TestResolveTemplateInjectsPerDispatcherTraceDefault(t *testing.T) {
	cases := []struct {
		name          string
		dir           string
		qualifiedName string
		wantFilename  string
	}{
		{
			name:          "city dispatcher",
			dir:           "",
			qualifiedName: config.ControlDispatcherAgentName,
			wantFilename:  "control-dispatcher-trace.log",
		},
		{
			name:          "rig dispatcher uses double-dash filename",
			dir:           "app",
			qualifiedName: "app/control-dispatcher",
			wantFilename:  "app--control-dispatcher-trace.log",
		},
		{
			name:          "non-dispatcher agent untouched",
			dir:           "",
			qualifiedName: "polecat",
			wantFilename:  "control-dispatcher-trace.log", // city-uniform default preserved
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cityPath := t.TempDir()
			writeTemplateResolveCityConfig(t, cityPath, "file")
			t.Setenv("GC_CITY_PATH", cityPath)
			t.Setenv("GC_CITY_RUNTIME_DIR", "")

			params := &agentBuildParams{
				cityName:   "city",
				cityPath:   cityPath,
				workspace:  &config.Workspace{Provider: "test"},
				providers:  map[string]config.ProviderSpec{"test": {Command: "echo", PromptMode: "none"}},
				lookPath:   func(string) (string, error) { return "/bin/echo", nil },
				fs:         fsys.OSFS{},
				beaconTime: time.Unix(0, 0),
				beadNames:  make(map[string]string),
				stderr:     io.Discard,
			}

			agentName := config.ControlDispatcherAgentName
			if tc.qualifiedName == "polecat" {
				agentName = "polecat"
			}
			agent := &config.Agent{Name: agentName, Dir: tc.dir}
			tp, err := resolveTemplate(params, agent, tc.qualifiedName, nil)
			if err != nil {
				t.Fatalf("resolveTemplate: %v", err)
			}

			wantPath := filepath.Join(cityPath, ".gc", "runtime", tc.wantFilename)
			if got := tp.Env["GC_CONTROL_DISPATCHER_TRACE_DEFAULT"]; got != wantPath {
				t.Fatalf("GC_CONTROL_DISPATCHER_TRACE_DEFAULT = %q, want %q", got, wantPath)
			}
		})
	}
}

// The controller token is controller scope, and every layer resolveTemplate
// merges after the passthrough is config-authored. Two exfiltration shapes
// exist, so both are driven here: a literal entry that overwrites the empty pin,
// and a "$GC_CONTROLLER_TOKEN" reference, which expandEnvMap resolves against
// the controller process and would land under a name no key-level guard
// watches. The upstream block is the same merge one layer later and writes
// AFTER the ScrubTokenEnv call, so it gets both shapes too.
func TestResolveTemplateWithholdsControllerTokenFromConfigAuthoredEnv(t *testing.T) {
	const token = "super-secret-controller-token"
	cityPath := t.TempDir()
	writeTemplateResolveCityConfig(t, cityPath, "file")
	t.Setenv(convergence.TokenEnvVar, token)

	params := &agentBuildParams{
		cityName: "city",
		cityPath: cityPath,
		city: &config.City{Upstreams: map[string]config.UpstreamSpec{
			"gateway": {
				// Abstract serving field, rendered onto the name the upstream
				// picks — a second expansion site, and one that writes after
				// the ScrubTokenEnv call.
				APIKey:    "$" + convergence.TokenEnvVar,
				APIKeyEnv: "UPSTREAM_SERVING_COPY",
				Env: map[string]string{
					convergence.TokenEnvVar: "upstream-literal",
					"UPSTREAM_COPY":         "$" + convergence.TokenEnvVar,
				},
			},
		}},
		workspace: &config.Workspace{
			Provider: "test",
			Env: map[string]string{
				convergence.TokenEnvVar: "workspace-literal",
				"WORKSPACE_COPY":        "$" + convergence.TokenEnvVar,
			},
		},
		providers:  map[string]config.ProviderSpec{"test": {Command: "echo", PromptMode: "none"}},
		lookPath:   func(string) (string, error) { return "/bin/echo", nil },
		fs:         fsys.OSFS{},
		beaconTime: time.Unix(0, 0),
		beadNames:  make(map[string]string),
		stderr:     io.Discard,
	}
	agent := &config.Agent{
		Name:     "mayor",
		Upstream: "gateway",
		Env: map[string]string{
			convergence.TokenEnvVar: "agent-literal",
			"AGENT_COPY":            "${" + convergence.TokenEnvVar + "}",
		},
	}

	tp, err := resolveTemplate(params, agent, agent.QualifiedName(), nil)
	if err != nil {
		t.Fatalf("resolveTemplate: %v", err)
	}

	val, ok := tp.Env[convergence.TokenEnvVar]
	if !ok {
		t.Errorf("Env omits %s; want present and empty so the session cannot inherit the controller's value", convergence.TokenEnvVar)
	} else if val != "" {
		t.Errorf("Env[%s] = %q, want empty (a config-authored literal must not overwrite the pin)", convergence.TokenEnvVar, val)
	}
	for _, key := range []string{"WORKSPACE_COPY", "AGENT_COPY", "UPSTREAM_COPY", "UPSTREAM_SERVING_COPY"} {
		if got := tp.Env[key]; got != "" {
			t.Errorf("Env[%s] = %q, want empty ($VAR expansion must not copy the controller token into another name)", key, got)
		}
	}
	for key, val := range tp.Env {
		if strings.Contains(val, token) {
			t.Errorf("Env[%s] = %q carries the controller token", key, val)
		}
	}
}

// OperatorEnv (ga-evj082) must carry exactly the operator-authored config
// layers — workspace.Env, the resolved provider's Env, and agent.Env — with
// the same last-wins override order as the main Env merge, so a resolved
// config env change drives runtime.Config.OperatorEnv and lands on the
// Launch-tier fingerprint instead of being a silent no-op. It must exclude
// everything else that also flows into tp.Env: passthrough (host/controller
// process env) and agentEnv (generated GC_* plumbing).
func TestResolveTemplatePopulatesOperatorEnvFromOperatorAuthoredLayers(t *testing.T) {
	cityPath := t.TempDir()
	writeTemplateResolveCityConfig(t, cityPath, "file")

	params := &agentBuildParams{
		cityName: "city",
		cityPath: cityPath,
		workspace: &config.Workspace{
			Provider: "test",
			Env: map[string]string{
				"WORKSPACE_VAR": "from-workspace",
				"OVERRIDE_VAR":  "workspace-value",
			},
		},
		providers: map[string]config.ProviderSpec{"test": {
			Command:    "echo",
			PromptMode: "none",
			Env: map[string]string{
				"PROVIDER_VAR": "from-provider",
				"OVERRIDE_VAR": "provider-value",
			},
		}},
		lookPath:   func(string) (string, error) { return "/bin/echo", nil },
		fs:         fsys.OSFS{},
		beaconTime: time.Unix(0, 0),
		beadNames:  make(map[string]string),
		stderr:     io.Discard,
	}
	agent := &config.Agent{
		Name: "runner",
		Env: map[string]string{
			"AGENT_VAR":    "from-agent",
			"OVERRIDE_VAR": "agent-value",
		},
	}

	tp, err := resolveTemplate(params, agent, agent.QualifiedName(), nil)
	if err != nil {
		t.Fatalf("resolveTemplate: %v", err)
	}

	want := map[string]string{
		"WORKSPACE_VAR": "from-workspace",
		"PROVIDER_VAR":  "from-provider",
		"AGENT_VAR":     "from-agent",
		"OVERRIDE_VAR":  "agent-value", // agent layer wins, same order as the Env merge.
	}
	if !maps.Equal(tp.OperatorEnv, want) {
		t.Errorf("OperatorEnv = %#v, want exactly %#v", tp.OperatorEnv, want)
	}

	// PATH is unconditionally passthrough-forwarded from the real process env
	// (processenv.ProviderProcessPassthroughEnv) into tp.Env, but it is not
	// operator-authored config and must not leak into OperatorEnv.
	if _, ok := tp.Env["PATH"]; !ok {
		t.Fatal("test invariant broken: expected passthrough PATH in tp.Env")
	}
	if _, ok := tp.OperatorEnv["PATH"]; ok {
		t.Errorf("OperatorEnv[PATH] present, want excluded (passthrough, not operator-authored)")
	}

	// GC_BIN is agentEnv-generated plumbing, present in tp.Env but never
	// operator-authored, and must not leak into OperatorEnv.
	if _, ok := tp.Env["GC_BIN"]; !ok {
		t.Fatal("test invariant broken: expected agentEnv-generated GC_BIN in tp.Env")
	}
	if _, ok := tp.OperatorEnv["GC_BIN"]; ok {
		t.Errorf("OperatorEnv[GC_BIN] present, want excluded (agentEnv-generated, not operator-authored)")
	}
}

// TestResolveTemplatePopulatesOperatorEnvWithExplicitProviderSelection covers
// ga-evj082 acceptance criterion 3's second provider-selection path: an agent
// that names its own provider (config.Agent.Provider) rather than falling
// back to workspace.Provider. resolveTemplate only ever consumes the
// already-resolved cfgAgent.Provider string (rig-patch composition happens
// upstream in internal/config/compose.go before cfgAgent reaches here), so
// this also stands in for the [[rigs.patches]].provider path.
func TestResolveTemplatePopulatesOperatorEnvWithExplicitProviderSelection(t *testing.T) {
	cityPath := t.TempDir()
	writeTemplateResolveCityConfig(t, cityPath, "file")

	params := &agentBuildParams{
		cityName: "city",
		cityPath: cityPath,
		workspace: &config.Workspace{
			Provider: "default-provider",
			Env:      map[string]string{"WORKSPACE_VAR": "from-workspace"},
		},
		providers: map[string]config.ProviderSpec{
			"default-provider": {
				Command:    "echo",
				PromptMode: "none",
				Env:        map[string]string{"DEFAULT_PROVIDER_VAR": "should-not-appear"},
			},
			"explicit-provider": {
				Command:    "echo",
				PromptMode: "none",
				Env:        map[string]string{"EXPLICIT_PROVIDER_VAR": "from-explicit-provider"},
			},
		},
		lookPath:   func(string) (string, error) { return "/bin/echo", nil },
		fs:         fsys.OSFS{},
		beaconTime: time.Unix(0, 0),
		beadNames:  make(map[string]string),
		stderr:     io.Discard,
	}
	agent := &config.Agent{Name: "runner", Provider: "explicit-provider"}

	tp, err := resolveTemplate(params, agent, agent.QualifiedName(), nil)
	if err != nil {
		t.Fatalf("resolveTemplate: %v", err)
	}

	want := map[string]string{
		"WORKSPACE_VAR":         "from-workspace",
		"EXPLICIT_PROVIDER_VAR": "from-explicit-provider",
	}
	if !maps.Equal(tp.OperatorEnv, want) {
		t.Errorf("OperatorEnv = %#v, want exactly %#v", tp.OperatorEnv, want)
	}
}
