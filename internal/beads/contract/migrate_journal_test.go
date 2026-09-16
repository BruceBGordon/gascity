package contract

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestMigrateJournalFileMatchesPinnedBeads reads the name out of the pinned
// beads module rather than restating it.
//
// The previous spelling of this constant was a plausible-looking guess
// ("migrate-dolt-mode.json"). Nothing failed: the product's post-migration
// verification stat'd a file that could never exist, so it always passed, and so
// did the tests asserting the residue was gone. A constant that is only ever
// compared against itself proves nothing, so this one is compared against bd.
func TestMigrateJournalFileMatchesPinnedBeads(t *testing.T) {
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/steveyegge/beads").Output()
	if err != nil {
		t.Skipf("pinned beads module is not in the local module cache: %v", err)
	}
	dir := strings.TrimSpace(string(out))
	if dir == "" {
		t.Skip("pinned beads module has no local directory")
	}
	source, err := os.ReadFile(filepath.Join(dir, "cmd", "bd", "migrate_dolt_mode.go"))
	if err != nil {
		t.Skipf("pinned beads module does not carry the migrate source: %v", err)
	}
	const decl = `migrateJournalFileName = "`
	idx := strings.Index(string(source), decl)
	if idx < 0 {
		t.Fatalf("pinned beads no longer declares migrateJournalFileName; re-derive %s by hand", MigrateDoltModeJournalFile)
	}
	rest := string(source)[idx+len(decl):]
	end := strings.Index(rest, `"`)
	if end < 0 {
		t.Fatalf("could not parse migrateJournalFileName out of the pinned beads source")
	}
	if got := rest[:end]; got != MigrateDoltModeJournalFile {
		t.Fatalf("MigrateDoltModeJournalFile = %q, pinned bd writes %q", MigrateDoltModeJournalFile, got)
	}
}
