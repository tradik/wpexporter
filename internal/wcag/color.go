// Package wcag reads CSS colors and measures the contrast between them, to the
// WCAG 2.2 definition.
//
// It lives on its own because two parts of the exporter need the same answer
// from the same arithmetic: the accessibility report, which finds body copy the
// site itself renders unreadable, and palette extraction, which refuses to
// record a background and a text color that cannot be the pair a page actually
// used. Two copies of a luminance formula are two chances to disagree about
// whether a site passes.
package wcag

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// MinContrastRatio is WCAG 2.2 SC 1.4.3 (Contrast Minimum) for normal-size
// text: the ratio below which a foreground and background pair is a failure.
const MinContrastRatio = 4.5

var (
	// hexPattern matches #rgb and #rrggbb.
	hexPattern = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)
	// rgbPattern matches rgb(r, g, b) and rgba(r, g, b, a).
	rgbPattern = regexp.MustCompile(`^rgba?\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*(?:,[^)]*)?\)$`)
)

// namedColors covers the palette the classic WordPress editor offered, which is
// where most of these contrast problems come from.
var namedColors = map[string]Color{
	"black": {0, 0, 0}, "white": {255, 255, 255}, "red": {255, 0, 0},
	"lime": {0, 255, 0}, "blue": {0, 0, 255}, "yellow": {255, 255, 0},
	"cyan": {0, 255, 255}, "aqua": {0, 255, 255}, "magenta": {255, 0, 255},
	"fuchsia": {255, 0, 255}, "green": {0, 128, 0}, "silver": {192, 192, 192},
	"gray": {128, 128, 128}, "grey": {128, 128, 128}, "maroon": {128, 0, 0},
	"olive": {128, 128, 0}, "navy": {0, 0, 128}, "purple": {128, 0, 128},
	"teal": {0, 128, 128},
}

// Color is an 8-bit-per-channel sRGB color.
type Color struct {
	R, G, B uint8
}

// Hex renders the color the way a stylesheet would.
func (c Color) Hex() string {
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}

// Parse reads a CSS color in hex, rgb()/rgba() or named form, and reports
// whether the value was one at all. Anything else — a var() reference, a
// gradient, a keyword like `inherit` — is not a color and is refused rather
// than guessed at.
func Parse(value string) (Color, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return Color{}, false
	}

	if named, ok := namedColors[value]; ok {
		return named, true
	}

	if match := hexPattern.FindStringSubmatch(value); match != nil {
		return parseHex(match[1]), true
	}

	if match := rgbPattern.FindStringSubmatch(value); match != nil {
		return parseChannels(match[1:4])
	}

	return Color{}, false
}

// parseHex expands a 3- or 6-digit hex body into a color.
//
// Each channel is parsed on its own rather than shifted out of a wider integer,
// so every value provably fits the 8 bits it is stored in.
func parseHex(digits string) Color {
	if len(digits) == 3 {
		digits = string([]byte{
			digits[0], digits[0],
			digits[1], digits[1],
			digits[2], digits[2],
		})
	}

	return Color{
		R: parseHexChannel(digits[0:2]),
		G: parseHexChannel(digits[2:4]),
		B: parseHexChannel(digits[4:6]),
	}
}

// parseHexChannel reads one two-digit hex channel. The caller's pattern has
// already established that the digits are valid hex.
func parseHexChannel(pair string) uint8 {
	value, err := strconv.ParseUint(pair, 16, 8)
	if err != nil {
		return 0
	}

	return uint8(value)
}

// parseChannels reads the three channels of an rgb()/rgba() color.
func parseChannels(channels []string) (Color, bool) {
	values := make([]uint8, 0, 3)

	for _, channel := range channels {
		parsed, err := strconv.Atoi(channel)
		if err != nil || parsed < 0 || parsed > 255 {
			return Color{}, false
		}

		values = append(values, uint8(parsed))
	}

	return Color{R: values[0], G: values[1], B: values[2]}, true
}

// ContrastRatio computes the WCAG contrast ratio between two colors. It is
// symmetric: the lighter of the pair is always the numerator.
func ContrastRatio(a, b Color) float64 {
	lighter, darker := relativeLuminance(a), relativeLuminance(b)
	if lighter < darker {
		lighter, darker = darker, lighter
	}

	return (lighter + 0.05) / (darker + 0.05)
}

// relativeLuminance implements the WCAG 2.2 relative luminance formula.
func relativeLuminance(c Color) float64 {
	return 0.2126*channelLuminance(c.R) + 0.7152*channelLuminance(c.G) + 0.0722*channelLuminance(c.B)
}

// channelLuminance linearises one sRGB channel.
func channelLuminance(value uint8) float64 {
	scaled := float64(value) / 255.0
	if scaled <= 0.04045 {
		return scaled / 12.92
	}

	return math.Pow((scaled+0.055)/1.055, 2.4)
}
