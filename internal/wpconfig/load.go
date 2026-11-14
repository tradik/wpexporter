package wpconfig

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// maxFileBytes bounds how much of wp-config.php is read. A real one is a few
// kilobytes; anything near this is not a configuration file.
const maxFileBytes = 1 << 20

// Load finds and reads the wp-config.php of the WordPress installed in
// absPath, applies the overrides and returns the settings. The password comes,
// in order, from PasswordEnv (read through getenv) and from the file.
func Load(absPath string, ov Overrides, getenv func(string) string) (*Settings, error) {
	abs, err := filepath.Abs(absPath)
	if err != nil {
		return nil, fmt.Errorf("--local %q: %w", absPath, err)
	}

	file, err := locate(abs)
	if err != nil {
		return nil, err
	}

	src, err := readBounded(file)
	if err != nil {
		return nil, err
	}

	if ov.DBPassword == "" {
		ov.DBPassword = getenv(PasswordEnv)
	}

	return parse(src, abs, file, ov, getenv)
}

// locate returns the wp-config.php WordPress itself would load for absPath: the
// one in the directory, else the one directly above it — unless that directory
// holds a WordPress of its own (a wp-settings.php), exactly as wp-load.php
// decides.
func locate(absPath string) (string, error) {
	here := filepath.Join(absPath, "wp-config.php")
	if exists(here) {
		return here, nil
	}

	parent := filepath.Dir(absPath)
	above := filepath.Join(parent, "wp-config.php")
	if exists(above) && !exists(filepath.Join(parent, "wp-settings.php")) {
		return above, nil
	}

	return "", fmt.Errorf("no wp-config.php in %s or the directory above it — is --local the WordPress directory?", absPath)
}

// exists reports whether path exists.
func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// readBounded reads path, refusing anything larger than maxFileBytes.
func readBounded(path string) ([]byte, error) {
	f, err := os.Open(path) // #nosec G304 -- the operator named this WordPress directory with --local
	if err != nil {
		return nil, fmt.Errorf("cannot open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	src, err := io.ReadAll(io.LimitReader(f, maxFileBytes+1))
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", path, err)
	}
	if len(src) > maxFileBytes {
		return nil, fmt.Errorf("%s is larger than %d bytes; that is not a wp-config.php", path, maxFileBytes)
	}
	return src, nil
}
