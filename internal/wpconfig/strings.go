package wpconfig

import "strings"

// stringLiteral decodes one complete PHP string literal. Single quotes know two
// escapes (\' and \\); double quotes know the usual control escapes and \$.
// A double-quoted string that interpolates a variable, or uses a numeric
// escape, is refused rather than half-decoded.
func stringLiteral(t string) (value, error) {
	if len(t) < 2 || (t[0] != '\'' && t[0] != '"') || skipString(t, 0) != len(t) || t[len(t)-1] != t[0] {
		return value{}, errComputed
	}
	body := t[1 : len(t)-1]

	if t[0] == '\'' {
		var b strings.Builder
		for i := 0; i < len(body); i++ {
			if body[i] == '\\' && i+1 < len(body) && (body[i+1] == '\'' || body[i+1] == '\\') {
				i++
			}
			b.WriteByte(body[i])
		}
		return value{str: b.String()}, nil
	}

	return doubleQuoted(body)
}

// doubleEscapes maps the escapes a double-quoted PHP string decodes.
var doubleEscapes = map[byte]byte{
	'n': '\n', 't': '\t', 'r': '\r', 'v': '\v', 'f': '\f', 'e': 0x1b,
	'\\': '\\', '$': '$', '"': '"',
}

// doubleQuoted decodes the body of a double-quoted PHP string.
func doubleQuoted(body string) (value, error) {
	var b strings.Builder
	for i := 0; i < len(body); i++ {
		c := body[i]
		switch {
		case c == '$':
			return value{}, errComputed // variable interpolation
		case c == '\\' && i+1 < len(body):
			next := body[i+1]
			if decoded, ok := doubleEscapes[next]; ok {
				b.WriteByte(decoded)
				i++
				continue
			}
			if next == 'x' || next == 'u' || (next >= '0' && next <= '7') {
				return value{}, errComputed // numeric escape: not decoded here
			}
			b.WriteByte(c) // PHP keeps an unknown escape as written
		default:
			b.WriteByte(c)
		}
	}
	return value{str: b.String()}, nil
}
