package origin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	cases := []struct {
		raw     string
		want    Origin
		wantErr string
	}{
		{raw: "", want: Origin{}},
		{raw: "   ", want: Origin{}},
		{raw: "http://127.0.0.1:8080", want: Origin{Scheme: "http", Host: "127.0.0.1:8080"}},
		{raw: "https://127.0.0.1", want: Origin{Scheme: "https", Host: "127.0.0.1:443"}},
		{raw: "http://localhost", want: Origin{Scheme: "http", Host: "localhost:80"}},
		{raw: "HTTPS://[::1]:8443/", want: Origin{Scheme: "https", Host: "[::1]:8443"}},
		{raw: "http://[::1]", want: Origin{Scheme: "http", Host: "[::1]:80"}},
		{raw: "ftp://127.0.0.1", wantErr: "scheme"},
		{raw: "127.0.0.1:8080", wantErr: "--origin"},
		{raw: "localhost:8080", wantErr: "scheme"},
		{raw: "http://", wantErr: "host"},
		{raw: "http://127.0.0.1/wordpress", wantErr: "path"},
		{raw: "http://127.0.0.1?x=1", wantErr: "path"},
		{raw: "http://user:pw@127.0.0.1", wantErr: "credentials"},
		{raw: "http://127.0.0.1:99999", wantErr: "port"},
		{raw: "http://127.0.0.1:abc", wantErr: "--origin"},
		{raw: "http://%zz", wantErr: "--origin"},
	}

	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			got, err := Parse(tc.raw)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestOriginEnabledAndString(t *testing.T) {
	assert.False(t, Origin{}.Enabled())
	assert.Equal(t, "", Origin{}.String())

	o := Origin{Scheme: "http", Host: "127.0.0.1:8080"}
	assert.True(t, o.Enabled())
	assert.Equal(t, "http://127.0.0.1:8080", o.String())
}
