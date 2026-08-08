package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/cache"
	"github.com/tradik/wpexporter/internal/config"
)

const menusBody = `[{
	"id": 3, "name": "Categories", "slug": "categories",
	"description": "Sidebar", "locations": ["primary"]
}]`

// menuItemsBody returns items out of order, as the API may.
const menuItemsBody = `[
	{"id": 42, "title": {"rendered": "About Us"}, "url": "https://x.test/about-us",
	 "parent": 0, "menu_order": 2, "type": "post_type", "object": "page",
	 "object_id": 7, "classes": [""], "menus": 3},
	{"id": 41, "title": {"rendered": "Malta"}, "url": "https://x.test/malta/",
	 "parent": 0, "menu_order": 1, "type": "taxonomy", "object": "category",
	 "object_id": 5, "target": "_blank", "classes": ["highlight", ""], "menus": 3}
]`

func newMenuClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client, err := NewClient(&config.Config{URL: server.URL, Timeout: 5, Retries: 0})
	require.NoError(t, err)

	return client
}

func TestGetMenus(t *testing.T) {
	client := newMenuClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch {
		case strings.HasPrefix(r.URL.Path, "/wp-json/wp/v2/menu-items"):
			assert.Equal(t, "3", r.URL.Query().Get("menus"))
			_, _ = w.Write([]byte(menuItemsBody))
		case strings.HasPrefix(r.URL.Path, "/wp-json/wp/v2/menus"):
			_, _ = w.Write([]byte(menusBody))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	menus, err := client.GetMenus()
	require.NoError(t, err)
	require.Len(t, menus, 1)

	menu := menus[0]
	assert.Equal(t, 3, menu.ID)
	assert.Equal(t, "Categories", menu.Name)
	assert.Equal(t, "categories", menu.Slug)
	assert.Equal(t, []string{"primary"}, menu.Locations)

	require.Len(t, menu.Items, 2)
	assert.Equal(t, "Malta", menu.Items[0].Title, "sorted by menu_order, not API order")
	assert.Equal(t, "About Us", menu.Items[1].Title)

	assert.Equal(t, "taxonomy", menu.Items[0].Type)
	assert.Equal(t, "category", menu.Items[0].Object)
	assert.Equal(t, 5, menu.Items[0].ObjectID)
	assert.Equal(t, "_blank", menu.Items[0].Target)
	assert.Equal(t, []string{"highlight"}, menu.Items[0].Classes, "the empty class is dropped")
	assert.Nil(t, menu.Items[1].Classes, "an item with no classes has none, not [\"\"]")
}

// TestGetMenusUnauthorized covers the common case: WordPress gates menus behind
// edit_theme_options, so a public site refuses them however they are configured.
func TestGetMenusUnauthorized(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			client := newMenuClient(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(status)
			})

			_, err := client.GetMenus()
			assert.True(t, errors.Is(err, ErrMenusNotAccessible),
				"caller must be able to tell this apart from a real failure")
		})
	}
}

// TestGetMenusItemsUnauthorized covers a site that lists menus but refuses their
// items.
func TestGetMenusItemsUnauthorized(t *testing.T) {
	client := newMenuClient(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/wp-json/wp/v2/menu-items") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(menusBody))
	})

	_, err := client.GetMenus()
	assert.True(t, errors.Is(err, ErrMenusNotAccessible))
}

func TestGetMenusServerError(t *testing.T) {
	client := newMenuClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, err := client.GetMenus()

	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrMenusNotAccessible), "a 500 is a real failure, not a permission outcome")
}

func TestGetMenusMalformedResponses(t *testing.T) {
	t.Run("menus", func(t *testing.T) {
		client := newMenuClient(t, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{not json`))
		})

		_, err := client.GetMenus()
		require.Error(t, err)
	})

	t.Run("items", func(t *testing.T) {
		client := newMenuClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if strings.HasPrefix(r.URL.Path, "/wp-json/wp/v2/menu-items") {
				_, _ = w.Write([]byte(`{not json`))
				return
			}
			_, _ = w.Write([]byte(menusBody))
		})

		_, err := client.GetMenus()
		require.Error(t, err)
	})
}

func TestGetMenusEmpty(t *testing.T) {
	client := newMenuClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	})

	menus, err := client.GetMenus()

	require.NoError(t, err)
	assert.Empty(t, menus)
}

// TestGetMenusTransportError covers a site that is unreachable, which is a real
// failure rather than a permission outcome.
func TestGetMenusTransportError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := server.URL
	server.Close() // nothing is listening any more

	client, err := NewClient(&config.Config{URL: url, Timeout: 2, Retries: 0})
	require.NoError(t, err)

	_, err = client.GetMenus()

	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrMenusNotAccessible))
}

// TestGetMenusItemsTransportError covers the same for the items request, which
// runs after the menu list succeeded.
func TestGetMenusItemsTransportError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/wp-json/wp/v2/menu-items") {
			// Break the connection mid-request.
			if hijacker, ok := w.(http.Hijacker); ok {
				if conn, _, hijackErr := hijacker.Hijack(); hijackErr == nil && conn != nil {
					_ = conn.Close()
				}
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(menusBody))
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(&config.Config{URL: server.URL, Timeout: 2, Retries: 0})
	require.NoError(t, err)

	_, err = client.GetMenus()
	require.Error(t, err)
}

// TestGetMenusServedFromCache pins that a second call does not hit the network.
func TestGetMenusServedFromCache(t *testing.T) {
	requests := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")

		if strings.HasPrefix(r.URL.Path, "/wp-json/wp/v2/menu-items") {
			_, _ = w.Write([]byte(menuItemsBody))
			return
		}
		_, _ = w.Write([]byte(menusBody))
	}))
	t.Cleanup(server.Close)

	client, err := NewClient(&config.Config{URL: server.URL, Timeout: 5, Retries: 0})
	require.NoError(t, err)

	fileCache, err := cache.NewFileCache(t.TempDir(), time.Hour, server.URL)
	require.NoError(t, err)
	client.SetCache(fileCache)

	first, err := client.GetMenus()
	require.NoError(t, err)
	require.Len(t, first, 1)

	after := requests

	second, err := client.GetMenus()
	require.NoError(t, err)

	assert.Equal(t, first, second)
	assert.Equal(t, after, requests, "the cached call must not reach the network")
}

func TestNonEmptyClasses(t *testing.T) {
	assert.Equal(t, []string{"a", "b"}, nonEmptyClasses([]string{"a", "", "b"}))
	assert.Nil(t, nonEmptyClasses([]string{""}))
	assert.Nil(t, nonEmptyClasses(nil))
}
