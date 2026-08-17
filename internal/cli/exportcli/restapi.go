package exportcli

// What the site's API turned out to be (#66, #68).
//
// Two facts about a site change the shape of everything that follows and belong
// in the report rather than in the reader's guesswork: a REST API that answers
// only at `?rest_route=`, and a WordPress old enough to have no content API at
// all. Neither is a gap — nothing was skipped for them — but an export that
// came back thin because of one of them looks identical to an empty site, and
// that silence is what #68 was reported as.
//
// Both are learned lazily, while fetching: the first is a spelling the run
// switched to and carried on with, the second is the reason it could not. So
// the notice is collected after the collections have been asked for, said once
// on the console, and written to metadata.json where a scrolled console cannot
// lose it.

import (
	"github.com/tradik/wpexporter/internal/api"
	"github.com/tradik/wpexporter/internal/config"
)

// restRouteNotice is what the run says about a site serving its API at
// ?rest_route= — nothing failed, but the address in the report is not the one a
// reader would try by hand.
const restRouteNotice = "This site serves its REST API at /?rest_route= rather than /wp-json/ — " +
	"the fallback spelling WordPress uses when permalinks are plain or a plugin hides the " +
	"pretty route. The export used it and is complete."

// noteSiteAPI records, at most once, what the client learned about this site's
// REST API. It is called after the collections rather than before them because
// nothing is probed until a request actually fails.
func noteSiteAPI(client *api.Client, notices *[]string) {
	notice := siteAPINotice(client)
	if notice == "" {
		return
	}

	for _, said := range *notices {
		if said == notice {
			return
		}
	}

	*notices = append(*notices, notice)
	logf("\n%s\n", notice)
}

// siteAPINotice is the sentence this site earned, or none at all — which is the
// answer for nearly every site, and the one that costs nothing.
func siteAPINotice(client *api.Client) string {
	switch {
	case client.RestAPIAbsent():
		return api.RestAPINotice()
	case client.UsesRestRouteFallback():
		return restRouteNotice
	default:
		return ""
	}
}

// fallBackToFeed turns on feed recovery for a site that has no content API.
//
// --from-sitemap is normally the operator saying they already know the API is
// not answering (#40). On a WordPress older than the content API there is
// nothing for them to have known in advance, and the alternative is an export
// of nothing at all, so the run turns it on for itself (#68). Every other site
// is left exactly as the operator asked for it.
func fallBackToFeed(client *api.Client, cfg *config.Config) {
	if !client.RestAPIAbsent() || cfg.FromSitemap || cfg.NoInventoryCheck {
		return
	}

	cfg.FromSitemap = true
	logln("Reading the site's feed instead — there is no content API to read (--from-sitemap).")
}
