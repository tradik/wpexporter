package exportcli

// Shaping a preview (#62).
//
// A preview has a shape, not only a size: five posts say what a blog is, and
// five media items say almost nothing about a gallery site. So `--limit-per-type`
// takes a map as well as a number, and the two most common kinds have
// shortcuts of their own.
//
// Two ways of saying the same thing is two places that can disagree, so the
// rule is stated once and reported when it bites: a dedicated flag beats a map
// entry for the kind it names, and the run says which value it used. A silent
// pick between two things the operator asked for is the failure mode this
// project keeps closing.

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/tradik/wpexporter/internal/api"
	"github.com/tradik/wpexporter/internal/config"
)

// perTypeShortcuts are the kinds that earned a flag of their own, because they
// are what a preview is usually shaped around.
var perTypeShortcuts = []string{
	api.CollectionPosts, api.CollectionPages, api.CollectionMedia, api.CollectionProducts,
}

// parsePerType reads the value of --limit-per-type, which is either a bare
// number, a comma-separated list of kind=N, or both:
//
//	5                 five of every kind
//	posts=5,media=10  five posts, ten media, everything else unbounded
//	5,media=10        five of every kind, but ten media
//
// A kind is a collection name or a custom type's slug, since there can be any
// number of those and they cannot each have a flag.
func parsePerType(raw string) (fallback int, byType map[string]int, err error) {
	byType = map[string]int{}

	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		name, value, named := strings.Cut(part, "=")
		if !named {
			count, convErr := strconv.Atoi(part)
			if convErr != nil || count < 0 {
				return 0, nil, fmt.Errorf("--limit-per-type: %q is neither a count nor kind=count", part)
			}

			fallback = count

			continue
		}

		count, convErr := strconv.Atoi(strings.TrimSpace(value))
		if convErr != nil || count < 0 {
			return 0, nil, fmt.Errorf("--limit-per-type: %q is not a count for %q",
				strings.TrimSpace(value), strings.TrimSpace(name))
		}

		byType[strings.ToLower(strings.TrimSpace(name))] = count
	}

	return fallback, byType, nil
}

// applyPerTypeLimits folds the map form and the shortcut flags into the config,
// and returns what to tell the operator about any kind they named twice.
func applyPerTypeLimits(cfg *config.Config, raw string, shortcuts map[string]int) ([]string, error) {
	if raw != "" {
		fallback, byType, err := parsePerType(raw)
		if err != nil {
			return nil, err
		}

		cfg.LimitPerType = fallback
		cfg.LimitByType = byType
	}

	if cfg.LimitByType == nil && len(shortcuts) > 0 {
		cfg.LimitByType = map[string]int{}
	}

	var conflicts []string

	for _, kind := range perTypeShortcuts {
		count, given := shortcuts[kind]
		if !given {
			continue
		}

		// The dedicated flag wins, and says so. Choosing silently between two
		// numbers the operator asked for is exactly what this project keeps
		// filing issues about.
		if previous, named := cfg.LimitByType[kind]; named && previous != count {
			conflicts = append(conflicts, fmt.Sprintf(
				"--limit-%s %d overrides --limit-per-type %s=%d", kind, count, kind, previous))
		}

		cfg.LimitByType[kind] = count
	}

	sort.Strings(conflicts)

	return conflicts, nil
}

// limitsActive reports whether the run was capped in any way, which is what
// decides that media should follow the documents rather than the library.
func limitsActive(cfg *config.Config) bool {
	return cfg.Limit > 0 || cfg.LimitPerType > 0 || len(cfg.LimitByType) > 0
}
