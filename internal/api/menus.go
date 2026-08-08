package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"

	"github.com/tradik/wpexporter/internal/cache"
	"github.com/tradik/wpexporter/pkg/models"
)

// ErrMenusNotAccessible reports that the site did not let us read its menus.
//
// WordPress core gates /wp/v2/menus behind edit_theme_options, so a public site
// answers 401 however its menus are configured. This is a normal outcome rather
// than a failure: the caller warns and carries on.
var ErrMenusNotAccessible = fmt.Errorf("menus are not readable without authentication")

// restMenu is the /wp/v2/menus payload.
type restMenu struct {
	ID          int      `json:"id"`
	Name        string   `json:"name"`
	Slug        string   `json:"slug"`
	Description string   `json:"description"`
	Locations   []string `json:"locations"`
}

// restMenuItem is the /wp/v2/menu-items payload. Its title is a rendered object
// rather than a plain string.
type restMenuItem struct {
	ID          int      `json:"id"`
	Title       rendered `json:"title"`
	URL         string   `json:"url"`
	Parent      int      `json:"parent"`
	MenuOrder   int      `json:"menu_order"`
	Type        string   `json:"type"`
	Object      string   `json:"object"`
	ObjectID    int      `json:"object_id"`
	Target      string   `json:"target"`
	Description string   `json:"description"`
	Classes     []string `json:"classes"`
	Menus       int      `json:"menus"`
}

// rendered is WordPress's {"rendered": "..."} wrapper.
type rendered struct {
	Rendered string `json:"rendered"`
}

// GetMenus retrieves the site's navigation menus and their items.
//
// It returns ErrMenusNotAccessible when the site refuses the request, which is
// the common case: menus need authentication even on an otherwise public API.
func (c *Client) GetMenus() ([]models.WordPressMenu, error) {
	cacheKey := cache.GenerateAPIKey("menus", 0)

	var cachedMenus []models.WordPressMenu
	if c.getFromCache(cacheKey, &cachedMenus) {
		return cachedMenus, nil
	}

	restMenus, err := c.fetchMenuDefinitions()
	if err != nil {
		return nil, err
	}

	menus := make([]models.WordPressMenu, 0, len(restMenus))

	for _, definition := range restMenus {
		c.applyRateLimit()

		items, err := c.fetchMenuItems(definition.ID)
		if err != nil {
			return nil, err
		}

		menus = append(menus, models.WordPressMenu{
			ID:          definition.ID,
			Name:        definition.Name,
			Slug:        definition.Slug,
			Description: definition.Description,
			Locations:   definition.Locations,
			Items:       items,
		})
	}

	c.saveToCache(cacheKey, menus)

	return menus, nil
}

// fetchMenuDefinitions reads the menu list.
func (c *Client) fetchMenuDefinitions() ([]restMenu, error) {
	resp, err := c.httpClient.R().Get(fmt.Sprintf("%s/menus?per_page=100", c.baseURL))
	if err != nil {
		return nil, fmt.Errorf("failed to get menus: %w", err)
	}

	if resp.StatusCode() == http.StatusUnauthorized || resp.StatusCode() == http.StatusForbidden {
		return nil, ErrMenusNotAccessible
	}

	// A site with menus not exposed to REST answers 404 for the route itself.
	if resp.StatusCode() == http.StatusNotFound {
		return nil, ErrMenusNotAccessible
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d for menus", resp.StatusCode())
	}

	var menus []restMenu
	if err := json.Unmarshal(resp.Body(), &menus); err != nil {
		return nil, fmt.Errorf("failed to parse menus response: %w", err)
	}

	return menus, nil
}

// fetchMenuItems reads one menu's items, ordered as the site presents them.
func (c *Client) fetchMenuItems(menuID int) ([]models.WordPressMenuItem, error) {
	url := fmt.Sprintf("%s/menu-items?menus=%d&per_page=100", c.baseURL, menuID)

	resp, err := c.httpClient.R().Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get items for menu %d: %w", menuID, err)
	}

	if resp.StatusCode() == http.StatusUnauthorized || resp.StatusCode() == http.StatusForbidden {
		return nil, ErrMenusNotAccessible
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d for items of menu %d", resp.StatusCode(), menuID)
	}

	var restItems []restMenuItem
	if err := json.Unmarshal(resp.Body(), &restItems); err != nil {
		return nil, fmt.Errorf("failed to parse items for menu %d: %w", menuID, err)
	}

	items := make([]models.WordPressMenuItem, 0, len(restItems))
	for _, item := range restItems {
		items = append(items, models.WordPressMenuItem{
			ID:          item.ID,
			Title:       item.Title.Rendered,
			URL:         item.URL,
			Parent:      item.Parent,
			Order:       item.MenuOrder,
			Type:        item.Type,
			Object:      item.Object,
			ObjectID:    item.ObjectID,
			Target:      item.Target,
			Description: item.Description,
			Classes:     nonEmptyClasses(item.Classes),
		})
	}

	// The API does not guarantee order; menu_order is what the site renders by.
	sort.SliceStable(items, func(i, j int) bool { return items[i].Order < items[j].Order })

	return items, nil
}

// nonEmptyClasses drops the empty string WordPress emits for an item with no
// custom classes, so the field is absent rather than [""].
func nonEmptyClasses(classes []string) []string {
	kept := make([]string, 0, len(classes))
	for _, class := range classes {
		if class != "" {
			kept = append(kept, class)
		}
	}

	if len(kept) == 0 {
		return nil
	}

	return kept
}
