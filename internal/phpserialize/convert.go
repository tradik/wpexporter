package phpserialize

import "strconv"

// Strings returns the string values of a decoded array, in order, skipping
// anything else. Not an array: nil.
func Strings(v any) []string {
	arr, ok := v.(Array)
	if !ok {
		return nil
	}
	var out []string
	for _, val := range arr.Values() {
		if s, ok := val.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// Ints returns the integer values of a decoded array — integers, and strings
// holding one — in order, skipping anything else. Not an array: nil.
func Ints(v any) []int {
	arr, ok := v.(Array)
	if !ok {
		return nil
	}
	var out []int
	for _, val := range arr.Values() {
		switch x := val.(type) {
		case int64:
			out = append(out, int(x))
		case string:
			if n, err := strconv.Atoi(x); err == nil {
				out = append(out, n)
			}
		}
	}
	return out
}

// Int converts a decoded scalar to an int: integers, floats (truncated) and
// numeric strings. Anything else is 0.
func Int(v any) int {
	switch x := v.(type) {
	case int64:
		return int(x)
	case float64:
		return int(x)
	case string:
		n, _ := strconv.Atoi(x)
		return n
	}
	return 0
}
