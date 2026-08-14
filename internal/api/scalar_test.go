package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONScalarUnmarshal(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"quoted string", `"Europe/Warsaw"`, "Europe/Warsaw"},
		{"bare integer", `2`, "2"},
		{"negative integer", `-5`, "-5"},
		{"decimal", `5.5`, "5.5"},
		{"empty string", `""`, ""},
		{"null is absent, not an error", `null`, ""},
		{"object is absent, not an error", `{"a":1}`, ""},
		{"bool is absent, not an error", `true`, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got jsonScalar
			require.NoError(t, json.Unmarshal([]byte(tt.in), &got))
			assert.Equal(t, tt.want, got.String())
		})
	}
}

// TestJSONScalarKeepsSiblingFields is the property that mattered: one field of
// an unexpected type must not cost the rest of the document (#32).
func TestJSONScalarKeepsSiblingFields(t *testing.T) {
	var root apiRootInfo
	require.NoError(t, json.Unmarshal([]byte(`{"name":"S","gmt_offset":2}`), &root))

	assert.Equal(t, "S", root.Name)
	assert.Equal(t, "2", root.GMTOffset.String())
}
