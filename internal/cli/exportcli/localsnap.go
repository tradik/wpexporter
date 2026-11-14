package exportcli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// executable is os.Executable; swapped in tests.
var executable = os.Executable

// localSnapError reports that a snap-confined run cannot use local mode, or
// nil. Strict confinement denies a snap both /var/www and the MySQL socket, so
// the run would fail later with a permission error that does not say why —
// the same trap #19 was for /tmp.
//
// The test is that this binary lives under $SNAP. The variables alone are not
// enough: a program started from inside any snap (an IDE, a terminal, a
// snap-packaged Go) inherits SNAP and SNAP_NAME without being confined itself.
func localSnapError() error {
	snap := os.Getenv("SNAP")
	if snap == "" {
		return nil
	}
	exe, err := executable()
	if err != nil || !strings.HasPrefix(filepath.Clean(exe), filepath.Clean(snap)+string(filepath.Separator)) {
		return nil
	}
	return fmt.Errorf("--local reads the WordPress directory and its database, which a strictly confined snap " +
		"cannot reach (/var/www, the MySQL socket). Use the release binary, the .deb/.rpm package or the " +
		"Docker image for local mode — see docs/LOCAL.md")
}
