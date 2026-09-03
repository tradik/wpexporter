package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/config"
)

// apiRootBody mirrors what a public WordPress serves at /wp-json/, taken from
// the site reported in issue #15.
const apiRootBody = `{
	"name": "Zonqor.com",
	"description": "A tagline",
	"url": "http://zonqor.com",
	"home": "https://zonqor.com",
	"gmt_offset": "1",
	"timezone_string": "",
	"namespaces": ["wp/v2"],
	"routes": {"/wp/v2/posts": {}}
}`

// wpV2IndexBody is the route index the old code fell back to. It unmarshals
// cleanly into any struct, which is why the failure was silent.
const wpV2IndexBody = `{"namespace":"wp/v2","routes":{"/wp/v2/posts":{}}}`

func newSiteInfoClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client, err := NewClient(&config.Config{URL: server.URL, Timeout: 5, Retries: 0})
	require.NoError(t, err)

	return client, server
}

// TestGetSiteInfoFromAPIRoot covers issue #15: an unauthenticated site returns
// 401 for /wp/v2/settings, and the identity fields must come from /wp-json/.
func TestGetSiteInfoFromAPIRoot(t *testing.T) {
	client, server := newSiteInfoClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/wp-json/wp/v2/settings":
			w.WriteHeader(http.StatusUnauthorized)
		case "/wp-json":
			_, _ = w.Write([]byte(apiRootBody))
		default:
			_, _ = w.Write([]byte(wpV2IndexBody))
		}
	})

	info, err := client.GetSiteInfo()
	require.NoError(t, err)

	assert.Equal(t, "Zonqor.com", info.Name)
	assert.Equal(t, "A tagline", info.Description)
	assert.Equal(t, "http://zonqor.com", info.URL)
	assert.Equal(t, "https://zonqor.com", info.HomeURL)
	assert.Equal(t, "UTC+1", info.Timezone, "a site with no named zone reports only an offset")

	_ = server
}

// TestGetSiteInfoSettingsOverlay pins that the authenticated settings document
// wins where it has a value — including `title`, which the old code read as
// `name` and therefore always dropped.
func TestGetSiteInfoSettingsOverlay(t *testing.T) {
	client, _ := newSiteInfoClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/wp-json/wp/v2/settings":
			_, _ = w.Write([]byte(`{
				"title": "Authenticated &amp; Titled",
				"email": "admin@example.com",
				"timezone": "Europe/Warsaw",
				"date_format": "F j, Y",
				"time_format": "H:i",
				"start_of_week": 1,
				"language": "en_GB"
			}`))
		case "/wp-json":
			_, _ = w.Write([]byte(apiRootBody))
		default:
			_, _ = w.Write([]byte(wpV2IndexBody))
		}
	})

	info, err := client.GetSiteInfo()
	require.NoError(t, err)

	assert.Equal(t, "Authenticated & Titled", info.Name,
		"settings.title wins over root.name, and it is entity-encoded there too")
	assert.Equal(t, "admin@example.com", info.AdminEmail)
	assert.Equal(t, "Europe/Warsaw", info.Timezone)
	assert.Equal(t, "F j, Y", info.DateFormat)
	assert.Equal(t, "H:i", info.TimeFormat)
	assert.Equal(t, 1, info.StartOfWeek)
	assert.Equal(t, "en_GB", info.Language)

	// Fields the settings document omits keep the root's value.
	assert.Equal(t, "https://zonqor.com", info.HomeURL)
}

// TestGetSiteInfoFallsBackToConfiguredURL covers a site whose API root is
// unreachable: the export still records which URL it was pointed at, and says
// that nothing described the site rather than presenting the blanks as the
// site's own answer (#79).
func TestGetSiteInfoFallsBackToConfiguredURL(t *testing.T) {
	client, server := newSiteInfoClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	info, err := client.GetSiteInfo()

	// A gap, not a failure: the collections are read separately and the export
	// goes on.
	description, unread := Gap(err)
	require.True(t, unread, "an unread root has to be reported as a gap")
	assert.Contains(t, description, "site info")
	assert.Contains(t, description, "500")

	assert.Equal(t, server.URL, info.URL)
	assert.Empty(t, info.Name)
}

// TestGetSiteInfoIgnoresMalformedDocuments pins that unreadable JSON from either
// endpoint degrades to the configured URL rather than failing the export — and
// is still named, since the record it leaves behind is indistinguishable from a
// site that genuinely has no name.
func TestGetSiteInfoIgnoresMalformedDocuments(t *testing.T) {
	client, server := newSiteInfoClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{not json`))
	})

	info, err := client.GetSiteInfo()

	description, unread := Gap(err)
	require.True(t, unread)
	assert.Contains(t, description, "other than a REST document")

	assert.Equal(t, server.URL, info.URL)
}

// TestGetSiteInfoIsDescribedBySettingsAlone: a root that 404s is not a hole
// when the settings document answered. The site was described, just not by the
// endpoint asked first.
func TestGetSiteInfoIsDescribedBySettingsAlone(t *testing.T) {
	client, _ := newSiteInfoClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/wp/v2/settings") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"title":"Zonqor","language":"en_GB"}`))

			return
		}

		w.WriteHeader(http.StatusNotFound)
	})

	info, err := client.GetSiteInfo()
	require.NoError(t, err)

	assert.Equal(t, "Zonqor", info.Name)
}

// TestGetSiteInfoNumericGMTOffset is the regression from bociany.pl (#32):
// WordPress core writes gmt_offset as a NUMBER, and reading it as a string
// failed the whole root document — so an export of a 235-post site recorded no
// name, no tagline and no timezone, while every other endpoint answered fine.
// The fixture above quotes the offset, which is why the tests never saw it.
func TestGetSiteInfoNumericGMTOffset(t *testing.T) {
	client, _ := newSiteInfoClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/wp-json/wp/v2/settings":
			w.WriteHeader(http.StatusUnauthorized)
		case "/wp-json":
			_, _ = w.Write([]byte(`{
				"name": "bociany.pl",
				"description": "Fundacja Przyrodnicza &quot;pro Natura&quot;",
				"url": "https://bociany.pl",
				"home": "https://bociany.pl",
				"gmt_offset": 2,
				"timezone_string": "Europe/Warsaw"
			}`))
		default:
			_, _ = w.Write([]byte(wpV2IndexBody))
		}
	})

	info, err := client.GetSiteInfo()
	require.NoError(t, err)

	assert.Equal(t, "bociany.pl", info.Name)
	assert.Equal(t, `Fundacja Przyrodnicza "pro Natura"`, info.Description,
		"the tagline is entity-encoded at the source and plain text everywhere after")
	assert.Equal(t, "https://bociany.pl", info.HomeURL)
	assert.Equal(t, "Europe/Warsaw", info.Timezone)
}

// TestGetSiteInfoNumericOffsetWithoutNamedZone pins the other half: a numeric
// offset must still produce a zone when the site never named one.
func TestGetSiteInfoNumericOffsetWithoutNamedZone(t *testing.T) {
	client, _ := newSiteInfoClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/wp-json/wp/v2/settings":
			w.WriteHeader(http.StatusUnauthorized)
		case "/wp-json":
			_, _ = w.Write([]byte(`{"name":"N","gmt_offset":-5,"timezone_string":""}`))
		default:
			_, _ = w.Write([]byte(wpV2IndexBody))
		}
	})

	info, err := client.GetSiteInfo()
	require.NoError(t, err)

	assert.Equal(t, "UTC-5", info.Timezone)
}

func TestGMTOffsetSuffix(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"1", "+1"},
		{"+2", "+2"},
		{"-5", "-5"},
		{" 3 ", "+3"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			assert.Equal(t, tt.want, gmtOffsetSuffix(tt.in))
		})
	}
}

func TestOverlay(t *testing.T) {
	value := "original"

	overlay(&value, "")
	assert.Equal(t, "original", value, "a blank source must not clear an existing value")

	overlay(&value, "   ")
	assert.Equal(t, "original", value)

	overlay(&value, "replacement")
	assert.Equal(t, "replacement", value)
}

// TestUnansweredErrorSaysWhichSilence: the three ways a document fails to be
// read each get their own sentence, because "site info: " followed by nothing
// tells a user no more than the empty record did.
func TestUnansweredErrorSaysWhichSilence(t *testing.T) {
	tests := []struct {
		name string
		err  *UnansweredError
		want string
	}{
		{
			name: "a reason to give",
			err:  &UnansweredError{Endpoint: "site info", Err: errors.New("connection reset")},
			want: "site info: connection reset",
		},
		{
			name: "a status and nothing else",
			err:  &UnansweredError{Endpoint: "site info", Status: 403},
			want: "site info: site answered 403",
		},
		{
			name: "neither",
			err:  &UnansweredError{Endpoint: "site info"},
			want: "site info: site answered nothing a reader could use",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.err.Error())

			description, unread := Gap(tc.err)
			assert.True(t, unread)
			assert.Equal(t, tc.want, description)
		})
	}
}
