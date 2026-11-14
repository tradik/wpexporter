package phpserialize

import (
	"errors"
	"fmt"
	"strconv"
)

// array decodes the rest of a:COUNT:{key;value;...}
func (d *decoder) array(depth int) (any, error) {
	if depth >= maxDepth {
		return nil, fmt.Errorf("phpserialize: nesting deeper than %d", maxDepth)
	}
	n, err := d.length()
	if err != nil {
		return nil, err
	}
	// Every element takes at least four bytes ("N;N;" is the smallest pair),
	// so a count the remaining input cannot hold is refused before allocating.
	if n > (len(d.data)-d.pos)/4 {
		return nil, errors.New("phpserialize: array count exceeds the remaining data")
	}
	if err := d.expect('{'); err != nil {
		return nil, err
	}

	out := make(Array, 0, n)
	for i := 0; i < n; i++ {
		key, err := d.value(depth + 1)
		if err != nil {
			return nil, err
		}
		switch key.(type) {
		case int64, string:
		default:
			return nil, errors.New("phpserialize: array key must be an integer or a string")
		}
		val, err := d.value(depth + 1)
		if err != nil {
			return nil, err
		}
		out = append(out, Entry{Key: key, Value: val})
	}
	return out, d.expect('}')
}

// length reads a non-negative decimal followed by ':'.
func (d *decoder) length() (int, error) {
	text, err := d.until(':')
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(text)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("phpserialize: bad length %q", text)
	}
	return n, nil
}

// until returns the text up to (not including) stop and moves past stop.
func (d *decoder) until(stop byte) (string, error) {
	for i := d.pos; i < len(d.data); i++ {
		if d.data[i] == stop {
			text := string(d.data[d.pos:i])
			d.pos = i + 1
			return text, nil
		}
	}
	return "", d.eof()
}

// expect consumes c or fails.
func (d *decoder) expect(c byte) error {
	if d.pos >= len(d.data) {
		return d.eof()
	}
	if d.data[d.pos] != c {
		return fmt.Errorf("phpserialize: expected %q at offset %d", c, d.pos)
	}
	d.pos++
	return nil
}

// eof is the error for input that ends mid-value.
func (d *decoder) eof() error {
	return errors.New("phpserialize: unexpected end of data")
}
