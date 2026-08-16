package api

// One type's archive slug cost a site every custom type it had (#53).

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// typesBody is the document from the issue: a product archive registered under
// its own slug, beside types whose has_archive is an ordinary boolean.
const typesBody = `{
  "post":            {"slug":"post","name":"Posts","rest_base":"posts","has_archive":false},
  "product":         {"slug":"product","name":"Products","rest_base":"product","has_archive":"shop"},
  "mec-events":      {"slug":"mec-events","name":"Events","rest_base":"mec-events","has_archive":true},
  "avada_portfolio": {"slug":"avada_portfolio","name":"Portfolio","rest_base":"avada_portfolio","has_archive":true}
}`

// TestArchiveSlugDoesNotCostTheOtherTypes: the bug. Decoding stopped at the
// first string, and every type on the site went with it.
func TestArchiveSlugDoesNotCostTheOtherTypes(t *testing.T) {
	client := newRetryingClient(t, 0, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(typesBody))
	})

	types, err := client.GetPostTypes()
	require.NoError(t, err)
	require.Len(t, types, 4, "every type the site registers")

	byslug := map[string]PostType{}
	for _, postType := range types {
		byslug[postType.Slug] = postType
	}

	assert.True(t, byslug["product"].HasArchive)
	assert.Equal(t, "shop", byslug["product"].ArchiveSlug,
		"the address the archive is published at, which a migration needs to avoid 404ing it")
	assert.True(t, byslug["mec-events"].HasArchive)
	assert.Empty(t, byslug["mec-events"].ArchiveSlug, "true means the type's own slug")
	assert.False(t, byslug["post"].HasArchive)

	custom := CustomPostTypes(types)
	assert.Len(t, custom, 2, "the two the export treats as content")
}

// TestOneUnreadableTypeCostsOnlyItself: the hardening the issue asks for. A
// single unexpected scalar anywhere used to drop everything; now it drops the
// type it is in, and says which.
func TestOneUnreadableTypeCostsOnlyItself(t *testing.T) {
	broken := `{
	  "good":   {"slug":"good","rest_base":"good","has_archive":true},
	  "broken": {"slug":["not","a","string"],"rest_base":"broken"}
	}`

	client := newRetryingClient(t, 0, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(broken))
	})

	types, err := client.GetPostTypes()
	require.NoError(t, err, "one bad type is not a failed discovery")
	require.Len(t, types, 1)
	assert.Equal(t, "good", types[0].Slug)
}

// TestHasArchiveReadsEveryForm: the three WordPress returns, and the shapes a
// plugin might invent — none of which may fail the document.
func TestHasArchiveReadsEveryForm(t *testing.T) {
	for _, testCase := range []struct {
		raw     string
		enabled bool
		slug    string
	}{
		{`true`, true, ""},
		{`false`, false, ""},
		{`"shop"`, true, "shop"},
		{`""`, false, ""},
		{`null`, false, ""},
		{`{"weird":1}`, false, ""},
		{`42`, false, ""},
	} {
		var archive hasArchive
		require.NoError(t, json.Unmarshal([]byte(testCase.raw), &archive), "raw %s", testCase.raw)
		assert.Equal(t, testCase.enabled, archive.Enabled, "raw %s", testCase.raw)
		assert.Equal(t, testCase.slug, archive.Slug, "raw %s", testCase.raw)
	}
}

// TestTypesDocumentThatIsNotAnObject: a body that is not the document at all is
// still an error — there is nothing to salvage from it.
func TestTypesDocumentThatIsNotAnObject(t *testing.T) {
	client := newRetryingClient(t, 0, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`"not a document"`))
	})

	_, err := client.GetPostTypes()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse post types")
}

// TestDecodePostTypesSkipsWhatItCannotFetch: a type with no rest_base is not
// reachable over REST, so it is not a type this export can walk.
func TestDecodePostTypesSkipsWhatItCannotFetch(t *testing.T) {
	types, unreadable := decodePostTypes(map[string]json.RawMessage{
		"reachable":   json.RawMessage(`{"rest_base":"reachable"}`),
		"unreachable": json.RawMessage(`{"rest_base":""}`),
		"broken":      json.RawMessage(`{"rest_base":{}}`),
	})

	require.Len(t, types, 1)
	assert.Equal(t, "reachable", types[0].Slug, "the key stands in for a missing slug")
	assert.Equal(t, []string{"broken"}, unreadable)
}

// TestArchiveSlugSurvivesTheCache: the slug is part of what discovery found, so
// a cached run must know it too.
func TestArchiveSlugSurvivesTheCache(t *testing.T) {
	var encoded strings.Builder
	require.NoError(t, json.NewEncoder(&encoded).Encode([]PostType{
		{Slug: "product", RestBase: "product", HasArchive: true, ArchiveSlug: "shop"},
	}))

	var restored []PostType
	require.NoError(t, json.Unmarshal([]byte(encoded.String()), &restored))
	assert.Equal(t, "shop", restored[0].ArchiveSlug)
}
