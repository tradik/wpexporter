package version

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// repoFile reads a file relative to the repository root.
func repoFile(t *testing.T, rel string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "..", rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}

	return string(data)
}

// TestDefaultMatchesVersionFile keeps the hardcoded default in step with the
// VERSION file, as this package's doc comment promises.
//
// The default is what a `go install` build and any build whose linker flags fail
// to reach this package report. When 1.8.1 shipped, the snap's flags targeted a
// symbol that no longer existed, so the binary fell back to this default — and
// because the default still said 1.8.0, the released snap reported the wrong
// version. Drift here is therefore not cosmetic.
func TestDefaultMatchesVersionFile(t *testing.T) {
	want := strings.TrimSpace(repoFile(t, "VERSION"))

	if Version != want {
		t.Errorf("version.Version = %q, VERSION file = %q; bump both together", Version, want)
	}
}

// TestBuildFilesStampThisPackage guards the linker path itself.
//
// `go build -X <symbol>=<value>` is silently ignored when the symbol does not
// exist: no error, no warning, just a binary carrying the default. The commands
// became thin wrappers in 1.8.0 and stopped declaring `main.Version`, so any
// build file still stamping `main.*` produces a binary that lies about its
// version — which is exactly how the 1.8.1 snap shipped as 1.8.0.
func TestBuildFilesStampThisPackage(t *testing.T) {
	const (
		wantPkg  = "internal/version"
		deadPath = "main.Version"
	)

	for _, name := range []string{"Makefile", "snap/snapcraft.yaml", "Dockerfile"} {
		content := repoFile(t, name)

		if strings.Contains(content, deadPath) {
			t.Errorf("%s stamps %s, which no longer exists; stamp %s instead", name, deadPath, wantPkg)
		}
		// Both files build the flag from a variable holding the package path, so
		// assert on the path and the stamp rather than on one literal string.
		if !strings.Contains(content, wantPkg) || !strings.Contains(content, ".Version=") {
			t.Errorf("%s does not stamp %s.Version; its binaries will report the default", name, wantPkg)
		}
	}
}
