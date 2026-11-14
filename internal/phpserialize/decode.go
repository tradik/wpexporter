// Package phpserialize decodes the PHP serialize() format WordPress stores in
// its options and meta tables — read-only, and only the plain-data subset.
//
// The data comes from a database this tool does not control. Objects (O:, C:)
// are refused outright: they are how PHP deserialisation attacks are built, and
// nothing an export needs is one. References are refused too. Nesting depth,
// element counts and the input size are bounded, and a declared length longer
// than what remains is an error rather than a read past the end.
package phpserialize

import (
	"errors"
	"fmt"
	"strconv"
)

// MaxInput is the largest serialized value decoded.
const MaxInput = 4 << 20

// maxDepth bounds array nesting.
const maxDepth = 32

// Entry is one key/value pair of a PHP array. Key is int64 or string.
type Entry struct {
	Key   any
	Value any
}

// Array is a PHP array in its original order.
type Array []Entry

// Get returns the value stored under key (compared as text, so "1" finds the
// integer key 1), or nil.
func (a Array) Get(key string) any {
	for _, e := range a {
		if fmt.Sprint(e.Key) == key {
			return e.Value
		}
	}
	return nil
}

// Values returns the values in order.
func (a Array) Values() []any {
	out := make([]any, 0, len(a))
	for _, e := range a {
		out = append(out, e.Value)
	}
	return out
}

// Decode decodes one serialized value: string, int64, float64, bool, nil or
// Array. Anything left over after it is an error.
func Decode(data []byte) (any, error) {
	if len(data) > MaxInput {
		return nil, fmt.Errorf("serialized value is larger than %d bytes", MaxInput)
	}
	d := &decoder{data: data}
	v, err := d.value(0)
	if err != nil {
		return nil, err
	}
	if d.pos != len(d.data) {
		return nil, errors.New("phpserialize: trailing data after the value")
	}
	return v, nil
}

// decoder walks the input.
type decoder struct {
	data []byte
	pos  int
}

// value decodes the value starting at the current position.
func (d *decoder) value(depth int) (any, error) {
	if d.pos+1 >= len(d.data) {
		return nil, d.eof()
	}
	kind := d.data[d.pos]
	if kind == 'N' {
		d.pos++
		return nil, d.expect(';')
	}
	d.pos++
	if err := d.expect(':'); err != nil {
		return nil, err
	}

	switch kind {
	case 's':
		return d.str()
	case 'i':
		return d.integer()
	case 'd':
		return d.float()
	case 'b':
		return d.boolean()
	case 'a':
		return d.array(depth)
	case 'O', 'C':
		return nil, errors.New("phpserialize: objects are refused")
	case 'r', 'R':
		return nil, errors.New("phpserialize: references are refused")
	}
	return nil, fmt.Errorf("phpserialize: unsupported type %q", kind)
}

// str decodes the rest of s:LEN:"...";
func (d *decoder) str() (any, error) {
	n, err := d.length()
	if err != nil {
		return nil, err
	}
	if err := d.expect('"'); err != nil {
		return nil, err
	}
	if n > len(d.data)-d.pos {
		return nil, errors.New("phpserialize: string length exceeds the remaining data")
	}
	s := string(d.data[d.pos : d.pos+n])
	d.pos += n
	if err := d.expect('"'); err != nil {
		return nil, err
	}
	return s, d.expect(';')
}

// integer decodes the rest of i:N;
func (d *decoder) integer() (any, error) {
	text, err := d.until(';')
	if err != nil {
		return nil, err
	}
	n, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("phpserialize: bad integer %q", text)
	}
	return n, nil
}

// float decodes the rest of d:F;
func (d *decoder) float() (any, error) {
	text, err := d.until(';')
	if err != nil {
		return nil, err
	}
	f, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return nil, fmt.Errorf("phpserialize: bad float %q", text)
	}
	return f, nil
}

// boolean decodes the rest of b:0; or b:1;
func (d *decoder) boolean() (any, error) {
	if d.pos >= len(d.data) || (d.data[d.pos] != '0' && d.data[d.pos] != '1') {
		return nil, errors.New("phpserialize: bad boolean")
	}
	b := d.data[d.pos] == '1'
	d.pos++
	return b, d.expect(';')
}
