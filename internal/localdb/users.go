package localdb

import (
	"database/sql"
	"fmt"

	"github.com/tradik/wpexporter/pkg/models"
)

// usersQuery reads the people who published something visible. Only these four
// columns: the users table also holds the password hash, the login, the e-mail
// address and the reset key, and none of them is selected anywhere.
const usersQuery = "SELECT ID, display_name, user_nicename, user_url FROM {users} WHERE ID IN " +
	"(SELECT DISTINCT post_author FROM {posts} WHERE post_type IN (?) AND " + published + ") ORDER BY ID"

// GetUsers returns the authors of the published documents, as the public REST
// API lists them. AvatarURLs stay empty: they are a hash of the e-mail address,
// which this source does not read.
func (s *Source) GetUsers() ([]models.WordPressUser, error) {
	types, err := s.documentTypes()
	if err != nil {
		return nil, err
	}

	l, err := s.linker()
	if err != nil {
		return nil, err
	}

	users := []models.WordPressUser{}
	err = s.q.each(s.ctx, func(r *sql.Rows) error {
		var u models.WordPressUser
		if err := r.Scan(&u.ID, &u.Name, &u.Slug, &u.URL); err != nil {
			return err
		}
		u.Link = l.author(u.Slug, u.ID)
		users = append(users, u)
		return nil
	}, usersQuery, list(types))
	if err != nil {
		return nil, s.fail(fmt.Errorf("reading users: %w", err))
	}
	if len(users) == 0 {
		return users, nil
	}

	index := make(map[int]int, len(users))
	userIDs := make([]int, len(users))
	for i, u := range users {
		index[u.ID], userIDs[i] = i, u.ID
	}
	// From usermeta, the biography and nothing else: the same table holds
	// session tokens and capabilities.
	err = s.q.each(s.ctx, func(r *sql.Rows) error {
		var id int
		var bio sql.NullString
		if err := r.Scan(&id, &bio); err != nil {
			return err
		}
		users[index[id]].Description = bio.String
		return nil
	}, "SELECT user_id, meta_value FROM {usermeta} WHERE meta_key = 'description' AND user_id IN (?)", list(userIDs))
	if err != nil {
		return nil, s.fail(fmt.Errorf("reading user descriptions: %w", err))
	}
	return users, nil
}

// authorSlugs maps author IDs to their nicename, for %author% permalinks.
func (s *Source) authorSlugs(ids []int) (map[int]string, error) {
	out := map[int]string{}
	err := s.q.each(s.ctx, func(r *sql.Rows) error {
		var id int
		var slug string
		if err := r.Scan(&id, &slug); err != nil {
			return err
		}
		out[id] = slug
		return nil
	}, "SELECT ID, user_nicename FROM {users} WHERE ID IN (?)", list(ids))
	if err != nil {
		return nil, s.fail(fmt.Errorf("reading author slugs: %w", err))
	}
	return out, nil
}
