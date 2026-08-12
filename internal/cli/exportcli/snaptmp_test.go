package exportcli

import (
	"strings"
	"testing"
)

// TestSnapPrivateTmpError covers issue #19: inside a snap, an output path under
// /tmp lands in the snap's private tmp namespace and is invisible to the user, so
// the export must fail before it starts rather than report a phantom success.
func TestSnapPrivateTmpError(t *testing.T) {
	tests := []struct {
		name    string
		snapEnv string
		output  string
		wantErr bool
	}{
		{"snap + /tmp subdir", "/snap/wpexporter/current", "/tmp/some/dir", true},
		{"snap + /tmp itself", "/snap/wpexporter/current", "/tmp", true},
		{"snap + home", "/snap/wpexporter/current", "/home/user/export", false},
		{"no snap + /tmp", "", "/tmp/some/dir", false},
		{"no snap + home", "", "/home/user/export", false},
		{"snap + path merely starting with tmp", "/snap/wpexporter/current", "/tmpdata/export", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SNAP", tt.snapEnv)
			t.Setenv("SNAP_NAME", "")

			err := snapPrivateTmpError(tt.output)
			if (err != nil) != tt.wantErr {
				t.Fatalf("snapPrivateTmpError(%q) err = %v, wantErr %v", tt.output, err, tt.wantErr)
			}
			if tt.wantErr && !strings.Contains(err.Error(), "snap-private-tmp") {
				t.Errorf("error should name the private tmp path, got: %v", err)
			}
		})
	}
}

// TestSnapPrivateTmpErrorUsesSnapName confirms the message names the actual snap.
func TestSnapPrivateTmpErrorUsesSnapName(t *testing.T) {
	t.Setenv("SNAP", "")
	t.Setenv("SNAP_NAME", "wpexporter")

	err := snapPrivateTmpError("/tmp/export")
	if err == nil {
		t.Fatal("expected an error when SNAP_NAME is set and output is under /tmp")
	}
	if !strings.Contains(err.Error(), "snap.wpexporter") {
		t.Errorf("error should include snap.wpexporter, got: %v", err)
	}
}
