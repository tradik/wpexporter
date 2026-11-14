package wpconfig

import "strings"

// callForm splits name(args) into the function name and its arguments.
func callForm(t string) (string, []string, bool) {
	open := strings.IndexByte(t, '(')
	if open <= 0 || !strings.HasSuffix(t, ")") {
		return "", nil, false
	}
	name := strings.ToLower(strings.TrimSpace(t[:open]))
	for i := 0; i < len(name); i++ {
		if !isIdent(name[i]) {
			return "", nil, false
		}
	}
	args, next, ok := scanArgs(t, open)
	if !ok || next != len(t) {
		return "", nil, false
	}
	if strings.TrimSpace(args) == "" {
		return name, nil, true
	}
	return name, splitTopLevel(args), true
}

// splitConcat splits expr on the "." operator outside strings and brackets.
func splitConcat(expr string) []string {
	var parts []string
	depth, start := 0, 0
	for i := 0; i < len(expr); i++ {
		switch expr[i] {
		case '\'', '"':
			i = skipString(expr, i) - 1
		case '(', '[':
			depth++
		case ')', ']':
			depth--
		case '.':
			if depth == 0 && !digitAt(expr, i+1) {
				parts = append(parts, expr[start:i])
				start = i + 1
			}
		}
	}
	return append(parts, expr[start:])
}

// isNumber reports whether t, which is never empty, is a plain decimal integer.
func isNumber(t string) bool {
	for i := 0; i < len(t); i++ {
		if t[i] < '0' || t[i] > '9' {
			return false
		}
	}
	return true
}

// digitAt reports whether s[i] exists and is a decimal digit: "1.5" is a
// number, not a concatenation.
func digitAt(s string, i int) bool {
	return i < len(s) && s[i] >= '0' && s[i] <= '9'
}
