package phpserialize

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeScalars(t *testing.T) {
	cases := map[string]any{
		`s:5:"hello";`:  "hello",
		`s:0:"";`:       "",
		`s:3:"a"b";`:    `a"b`,
		`s:5:"zół";`:    "zół",
		`i:42;`:         int64(42),
		`i:-7;`:         int64(-7),
		`d:1.5;`:        1.5,
		`d:-0.25;`:      -0.25,
		`b:1;`:          true,
		`b:0;`:          false,
		`N;`:            nil,
		`s:2:";}";`:     ";}",
		`s:6:"s:1:""";`: `s:1:""`,
	}
	for in, want := range cases {
		got, err := Decode([]byte(in))
		require.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}
}

func TestDecodeArray(t *testing.T) {
	got, err := Decode([]byte(`a:3:{i:0;s:4:"menu";i:1;s:6:"button";s:4:"open";b:1;}`))
	require.NoError(t, err)

	arr, ok := got.(Array)
	require.True(t, ok)
	require.Len(t, arr, 3)
	assert.Equal(t, Entry{Key: int64(0), Value: "menu"}, arr[0])
	assert.Equal(t, []any{"menu", "button", true}, arr.Values())
	assert.Equal(t, true, arr.Get("open"))
	assert.Equal(t, "button", arr.Get("1"))
	assert.Nil(t, arr.Get("missing"))
}

func TestDecodeNestedAttachmentMetadata(t *testing.T) {
	src := `a:4:{s:5:"width";i:1200;s:6:"height";i:800;s:4:"file";s:18:"2024/05/photo.jpeg";` +
		`s:5:"sizes";a:1:{s:9:"thumbnail";a:4:{s:4:"file";s:18:"photo-150x150.jpeg";s:5:"width";i:150;` +
		`s:6:"height";i:150;s:9:"mime-type";s:10:"image/jpeg";}}}`
	got, err := Decode([]byte(src))
	require.NoError(t, err)

	meta := got.(Array)
	assert.Equal(t, int64(1200), meta.Get("width"))
	sizes := meta.Get("sizes").(Array)
	thumb := sizes.Get("thumbnail").(Array)
	assert.Equal(t, "photo-150x150.jpeg", thumb.Get("file"))
	assert.Equal(t, "image/jpeg", thumb.Get("mime-type"))
}

func TestDecodeRefusals(t *testing.T) {
	cases := map[string]string{
		``:                        "unexpected end",
		`x:1;`:                    "unsupported",
		`O:8:"stdClass":0:{}`:     "object",
		`C:3:"Foo":0:{}`:          "object",
		`r:1;`:                    "reference",
		`R:1;`:                    "reference",
		`s:10:"short";`:           "length",
		`s:-1:"";`:                "length",
		`s:5:"hello"`:             "expected",
		`s:5:"hello";extra`:       "trailing",
		`s:5x"hello";`:            "expected",
		`s:abc:"x";`:              "length",
		`i:;`:                     "integer",
		`i:12`:                    "unexpected end",
		`i:99999999999999999999;`: "integer",
		`d:abc;`:                  "float",
		`b:2;`:                    "boolean",
		`b:1`:                     "expected",
		`a:0:x`:                   "expected",
		`s5:"hello";`:             "expected",
		`d:1.5`:                   "unexpected end",
		`s:5`:                     "unexpected end",
		`s:1:x;`:                  "expected",
		`s:1:"ab";`:               "expected",
		`a:2:{i:0;i:1;}`:          "expected",
		`a:1:{i:0;i:1;`:           "unexpected end",
		`a:1:{d:1.5;i:1;}`:        "key",
		`a:999999:{}`:             "count",
		`a:1:{i:0;O:1:"A":0:{}}`:  "object",
		`a:1x`:                    "expected",
		`a:1:{i:0;i:1;]`:          "expected",
		strings.Repeat("a:1:{i:0;", 40) + `N;` + strings.Repeat("}", 40): "deep",
	}
	for in, want := range cases {
		_, err := Decode([]byte(in))
		require.Error(t, err, in)
		assert.Contains(t, err.Error(), want, in)
	}
}

func TestDecodeTooLarge(t *testing.T) {
	_, err := Decode(make([]byte, MaxInput+1))
	assert.ErrorContains(t, err, "larger than")
}

func TestStrings(t *testing.T) {
	got, err := Decode([]byte(`a:3:{i:0;s:4:"menu";i:1;i:5;i:2;s:0:"";}`))
	require.NoError(t, err)
	assert.Equal(t, []string{"menu", ""}, Strings(got))
	assert.Nil(t, Strings("not an array"))
}

func TestInts(t *testing.T) {
	got, err := Decode([]byte(`a:4:{i:0;i:12;i:1;s:2:"34";i:2;s:1:"x";i:3;b:1;}`))
	require.NoError(t, err)
	assert.Equal(t, []int{12, 34}, Ints(got))
	assert.Nil(t, Ints(int64(1)))
}

func TestInt(t *testing.T) {
	assert.Equal(t, 5, Int(int64(5)))
	assert.Equal(t, 7, Int("7"))
	assert.Equal(t, 3, Int(3.9))
	assert.Equal(t, 0, Int("x"))
	assert.Equal(t, 0, Int(nil))
}

func FuzzDecode(f *testing.F) {
	for _, seed := range []string{`a:1:{s:1:"a";a:1:{i:0;d:1.5;}}`, `s:3:"abc";`, `b:1;`, `N;`, `i:-3;`} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = Decode(data)
	})
}
