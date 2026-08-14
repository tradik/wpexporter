package wcag

// The color arithmetic two callers depend on: the accessibility report, which
// finds body copy a site renders unreadable, and palette extraction, which
// refuses a background and text pair that cannot be what a page used.

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContrastRatio(t *testing.T) {
	tests := []struct {
		name string
		a    Color
		b    Color
		want float64
	}{
		{"black on white", Color{0, 0, 0}, Color{255, 255, 255}, 21},
		{"white on white", Color{255, 255, 255}, Color{255, 255, 255}, 1},
		{"yellow on white", Color{255, 255, 0}, Color{255, 255, 255}, 1.074},
		{"order does not matter", Color{255, 255, 255}, Color{0, 0, 0}, 21},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ContrastRatio(tt.a, tt.b)
			assert.Less(t, math.Abs(got-tt.want), 0.01, "got %.3f, want %.3f", got, tt.want)
		})
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		want  Color
		valid bool
	}{
		{"six digit hex", "#ff0000", Color{255, 0, 0}, true},
		{"three digit hex", "#f00", Color{255, 0, 0}, true},
		{"uppercase hex", "#FF00FF", Color{255, 0, 255}, true},
		{"named", "yellow", Color{255, 255, 0}, true},
		{"named uppercase", "BLACK", Color{0, 0, 0}, true},
		{"rgb", "rgb(1, 2, 3)", Color{1, 2, 3}, true},
		{"rgba", "rgba(1, 2, 3, 0.5)", Color{1, 2, 3}, true},
		{"padded", "  #ff0000  ", Color{255, 0, 0}, true},
		{"empty", "", Color{}, false},
		{"unknown keyword", "chartreuse-ish", Color{}, false},
		{"bad hex length", "#ff00", Color{}, false},
		{"rgb out of range", "rgb(1, 2, 300)", Color{}, false},
		{"rgb non numeric", "rgb(a, b, c)", Color{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := Parse(tt.in)
			assert.Equal(t, tt.valid, ok)
			if tt.valid {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestHex(t *testing.T) {
	assert.Equal(t, "#ffff00", Color{255, 255, 0}.Hex())
	assert.Equal(t, "#000000", Color{0, 0, 0}.Hex())
}

// TestParseHexChannelRejectsNonHex pins the defensive branch: a channel that is
// not valid hex reads as zero rather than panicking.
func TestParseHexChannelRejectsNonHex(t *testing.T) {
	assert.Equal(t, uint8(0), parseHexChannel("zz"))
	assert.Equal(t, uint8(255), parseHexChannel("ff"))
	assert.Equal(t, uint8(0), parseHexChannel("00"))
}
