package localdb

import (
	"database/sql"
	"fmt"
	"sort"

	"github.com/tradik/wpexporter/internal/phpserialize"
	"github.com/tradik/wpexporter/pkg/models"
)

// menuItemsQuery reads every published menu item with the menu it belongs to.
const menuItemsQuery = "SELECT p.ID, p.post_title, p.post_content, p.menu_order, tt.term_id FROM {posts} p " +
	"JOIN {term_relationships} tr ON tr.object_id = p.ID " +
	"JOIN {term_taxonomy} tt ON tt.term_taxonomy_id = tr.term_taxonomy_id " +
	"WHERE p.post_type = 'nav_menu_item' AND p.post_status = 'publish' AND tt.taxonomy = 'nav_menu' " +
	"ORDER BY tt.term_id, p.menu_order, p.ID"

// menuMetaQuery reads the item meta that says what each item points at.
const menuMetaQuery = "SELECT post_id, meta_key, meta_value FROM {postmeta} WHERE post_id IN (?) AND meta_key IN " +
	"('_menu_item_type', '_menu_item_menu_item_parent', '_menu_item_object', '_menu_item_object_id', " +
	"'_menu_item_target', '_menu_item_url', '_menu_item_classes')"

// GetMenus returns the navigation menus, their items and the theme locations
// they are assigned to. An item pointing at something that is not published is
// left out, as WordPress leaves it out of the page.
func (s *Source) GetMenus() ([]models.WordPressMenu, error) {
	// The linker needs the options, so after it they are all at hand.
	l, err := s.linker()
	if err != nil {
		return nil, err
	}

	menus := []models.WordPressMenu{}
	err = s.q.each(func(r *sql.Rows) error {
		var m models.WordPressMenu
		if err := r.Scan(&m.ID, &m.Name, &m.Slug, &m.Description); err != nil {
			return err
		}
		menus = append(menus, m)
		return nil
	}, "SELECT t.term_id, t.name, t.slug, tt.description FROM {terms} t JOIN {term_taxonomy} tt "+
		"ON tt.term_id = t.term_id WHERE tt.taxonomy = 'nav_menu' ORDER BY t.term_id")
	if err != nil {
		return nil, s.fail(fmt.Errorf("reading menus: %w", err))
	}
	if len(menus) == 0 {
		return menus, nil
	}

	items, owner, err := s.menuItems()
	if err != nil {
		return nil, err
	}
	targets, err := s.menuTargets(l)
	if err != nil {
		return nil, err
	}
	locations, err := s.menuLocations()
	if err != nil {
		return nil, err
	}

	for i := range menus {
		menus[i].Locations = locations[menus[i].ID]
		menus[i].Items = []models.WordPressMenuItem{}
		for _, it := range items {
			if owner[it.ID] != menus[i].ID {
				continue
			}
			if resolved, ok := targets.resolve(it); ok {
				menus[i].Items = append(menus[i].Items, resolved)
			}
		}
	}
	return menus, nil
}

// menuItems reads every item with its meta, and which menu owns each.
func (s *Source) menuItems() ([]models.WordPressMenuItem, map[int]int, error) {
	var items []models.WordPressMenuItem
	owner := map[int]int{}
	err := s.q.each(func(r *sql.Rows) error {
		var it models.WordPressMenuItem
		var menuID int
		if err := r.Scan(&it.ID, &it.Title, &it.Description, &it.Order, &menuID); err != nil {
			return err
		}
		owner[it.ID] = menuID
		items = append(items, it)
		return nil
	}, menuItemsQuery)
	if err != nil || len(items) == 0 {
		return items, owner, s.fail(err)
	}

	index := make(map[int]int, len(items))
	itemIDs := make([]int, len(items))
	for i, it := range items {
		index[it.ID], itemIDs[i] = i, it.ID
	}
	err = s.q.each(func(r *sql.Rows) error {
		var id int
		var key string
		var value sql.NullString
		if err := r.Scan(&id, &key, &value); err != nil {
			return err
		}
		applyMenuMeta(&items[index[id]], key, value.String)
		return nil
	}, menuMetaQuery, list(itemIDs))
	return items, owner, s.fail(err)
}

// applyMenuMeta sets the field one meta row describes.
func applyMenuMeta(it *models.WordPressMenuItem, key, value string) {
	switch key {
	case "_menu_item_type":
		it.Type = value
	case "_menu_item_menu_item_parent":
		it.Parent = phpserialize.Int(value)
	case "_menu_item_object":
		it.Object = value
	case "_menu_item_object_id":
		it.ObjectID = phpserialize.Int(value)
	case "_menu_item_target":
		it.Target = value
	case "_menu_item_url":
		it.URL = value
	default:
		decoded, _ := phpserialize.Decode([]byte(value))
		for _, class := range phpserialize.Strings(decoded) {
			if class != "" {
				it.Classes = append(it.Classes, class)
			}
		}
	}
}

// menuLocations maps each menu to the theme locations it fills, from the
// active theme's theme_mods_{stylesheet} option.
func (s *Source) menuLocations() (map[int][]string, error) {
	stylesheet := s.options["stylesheet"]
	var raw sql.NullString
	err := s.q.each(func(r *sql.Rows) error { return r.Scan(&raw) },
		"SELECT option_value FROM {options} WHERE option_name = ?", "theme_mods_"+stylesheet)
	if err != nil {
		return nil, s.fail(fmt.Errorf("reading theme menu locations: %w", err))
	}

	decoded, _ := phpserialize.Decode([]byte(raw.String))
	mods, _ := decoded.(phpserialize.Array)
	assigned, _ := mods.Get("nav_menu_locations").(phpserialize.Array)
	out := map[int][]string{}
	for _, e := range assigned {
		if location, ok := e.Key.(string); ok {
			id := phpserialize.Int(e.Value)
			out[id] = append(out[id], location)
		}
	}
	for id := range out {
		sort.Strings(out[id])
	}
	return out, nil
}
