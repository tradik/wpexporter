package exportcli

import (
	"fmt"
	"time"

	"github.com/tradik/wpexporter/internal/api"
	"github.com/tradik/wpexporter/internal/localdb"
	"github.com/tradik/wpexporter/pkg/models"
)

// fetchLocal reads every collection from the database, with the same skips,
// reports and failure rules as the REST export.
func (r *exportRun) fetchLocal(src *localdb.Source, originUp bool) error {
	cfg, c := r.cfg, &r.c

	logln("\nReading site information...")
	siteInfo, err := src.GetSiteInfo()
	if err != nil {
		return fmt.Errorf("failed to get site info: %w", err)
	}
	c.siteInfo = siteInfo

	read := func(skip bool, label, flag string, get func() ([]models.WordPressPost, error)) ([]models.WordPressPost, error) {
		if skip {
			logf("Skipping %s (--%s)\n", label, flag)
			return nil, nil
		}
		docs, err := get()
		if err := noteIncomplete(&c.gaps, label, err); err != nil {
			return nil, err
		}
		logf("Found %d %s\n", len(docs), label)
		return docs, nil
	}
	for _, kind := range []struct {
		dst         *[]models.WordPressPost
		skip        bool
		label, flag string
		get         func() ([]models.WordPressPost, error)
	}{
		{&c.posts, cfg.NoPosts, "posts", "no-posts", src.GetPosts},
		{&c.pages, cfg.NoPages, "pages", "no-pages", src.GetPages},
	} {
		if *kind.dst, err = read(kind.skip, kind.label, kind.flag, kind.get); err != nil {
			return err
		}
	}

	if err := r.localProducts(src, originUp); err != nil {
		return err
	}
	c.customTypes = fetchCustomTypes(src, cfg)

	if err := r.localTaxonomy(); err != nil {
		return err
	}
	c.menus = fetchMenus(src, cfg)
	c.comments = fetchComments(src, cfg)
	return nil
}

// localTaxonomy reads categories, tags and users.
func (r *exportRun) localTaxonomy() error {
	cfg, c := r.cfg, &r.c
	var err error
	c.categories, err = r.src.GetCategories()
	if err := noteIncomplete(&c.gaps, "categories", err); err != nil {
		return err
	}
	logf("Found %d categories\n", len(c.categories))

	if cfg.NoTags {
		logln("Skipping tags (--no-tags)")
	} else {
		c.tags, err = r.src.GetTags()
		if err := noteIncomplete(&c.gaps, "tags", err); err != nil {
			return err
		}
		logf("Found %d tags\n", len(c.tags))
	}

	if cfg.NoUsers {
		logln("Skipping users (--no-users)")
	} else if c.users, err = r.src.GetUsers(); err != nil {
		logf("Warning: could not read users: %v\n", err)
		c.users = []models.WordPressUser{}
	} else {
		logf("Found %d users\n", len(c.users))
	}
	return nil
}

// localProducts reads the WooCommerce catalog over the REST API through the
// origin — products are not read from the database in this release — or, with
// no origin, says how many were left out.
func (r *exportRun) localProducts(src *localdb.Source, originUp bool) error {
	if r.cfg.NoProducts {
		logln("Skipping products (--no-products)")
		return nil
	}
	if originUp {
		client, err := api.NewClient(r.cfg)
		if err != nil {
			return fmt.Errorf("failed to create API client: %w", err)
		}
		client.ShareLimits(r.limits)
		if r.fileCache != nil {
			client.SetCache(r.fileCache)
		}
		r.apiClient = client
		r.fetchRemoteProducts()
		return nil
	}

	count, err := src.PublishedProducts()
	if err != nil {
		return err
	}
	if count > 0 {
		r.c.productsNotice = fmt.Sprintf("Products: %d published WooCommerce products were not exported — they are "+
			"not read from the database in this release, and no web server answered to read them over the "+
			"REST API (give --origin)", count)
		logln(r.c.productsNotice)
	}
	return nil
}

// seconds converts a --timeout value.
func seconds(n int) time.Duration { return time.Duration(n) * time.Second }
