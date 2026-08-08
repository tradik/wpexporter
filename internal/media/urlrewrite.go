package media

import (
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/tradik/wpexporter/pkg/models"
)

// Media path styles for rewritten URLs in exported content.
const (
	// PathStyleRoot emits root-relative paths ("/media/images/1_x.jpg"). Default,
	// because such a path resolves identically from any URL depth.
	PathStyleRoot = "root"
	// PathStyleRelative emits paths relative to the export root ("media/images/1_x.jpg").
	// Only correct for content served from the site root; kept for backwards compatibility.
	PathStyleRelative = "relative"
)

// urlTokenPattern matches URL-ish tokens inside rendered HTML: absolute
// ("https://host/p"), protocol-relative ("//host/p") and root-relative WordPress
// paths ("/wp-content/uploads/..."). Commas terminate a token so that srcset
// candidate lists split correctly.
var urlTokenPattern = regexp.MustCompile(`(?i)(?:https?:)?//[^\s"'<>()\\,]+|/wp-(?:content|includes)/[^\s"'<>()\\,]+`)

// sizeSuffixPattern matches the WordPress "-<width>x<height>" thumbnail suffix.
var sizeSuffixPattern = regexp.MustCompile(`^(.*)-(\d+)x(\d+)$`)

// sizeVariant is one exported rendition of an attachment, used to remap
// references to size variants WordPress has since regenerated away.
type sizeVariant struct {
	width     int
	localPath string
}

// urlIndex resolves a normalized remote upload path to its exported local path.
//
// Matching is deliberately scheme- and host-insensitive: WordPress stores
// post_content with whatever URL form was current when the post was written
// (often http:// or a former domain), while the REST API reports source_url in
// the site's current form. Keying on the path alone makes both forms resolve.
type urlIndex struct {
	// exact maps a normalized upload path to its exported local path.
	exact map[string]string
	// variants maps an attachment's base path (size suffix stripped) to every
	// exported rendition of it, so a dead "-300x199" reference can fall back to
	// the closest surviving width.
	variants map[string][]sizeVariant
}

// UpdateMediaPaths rewrites every reference to a downloaded attachment in the
// given content so it points at the exported file instead of the live site.
//
// It replaces src, href, srcset and any other URL occurrence uniformly — the
// whole point of a local export is that it keeps working once the source host
// is retired.
func (d *Downloader) UpdateMediaPaths(content string, mediaItems []models.WordPressMedia) string {
	if !d.config.DownloadMedia {
		return content
	}

	index := d.buildURLIndex(mediaItems)
	if len(index.exact) == 0 {
		return content
	}

	return urlTokenPattern.ReplaceAllStringFunc(content, func(token string) string {
		key, ok := normalizeURLKey(token)
		if !ok {
			return token
		}

		localPath, remapped, ok := index.resolve(key)
		if !ok {
			return token
		}

		if remapped && d.config.Verbose && !d.config.Quiet {
			fmt.Printf("  ↻ Remapped stale size variant: %s -> %s\n", token, localPath)
		}

		return localPath
	})
}

// buildURLIndex indexes every attachment and each of its size variants.
func (d *Downloader) buildURLIndex(mediaItems []models.WordPressMedia) *urlIndex {
	index := &urlIndex{
		exact:    make(map[string]string),
		variants: make(map[string][]sizeVariant),
	}

	for _, media := range mediaItems {
		if media.SourceURL == "" {
			continue
		}

		parsedURL, err := url.Parse(media.SourceURL)
		if err != nil {
			continue
		}

		key, ok := normalizeURLKey(media.SourceURL)
		if !ok {
			continue
		}

		localPath := d.localMediaPath(d.generateFilename(media, parsedURL))
		index.add(key, localPath, toInt(media.MediaDetails.Width))

		for _, size := range media.MediaDetails.Sizes {
			if size.SourceURL == "" {
				continue
			}

			sizeKey, ok := normalizeURLKey(size.SourceURL)
			if !ok {
				continue
			}

			sizePath := d.localMediaPath(d.generateSizeFilename(media, size, parsedURL))
			index.add(sizeKey, sizePath, toInt(size.Width))
		}
	}

	return index
}

// add records one rendition under both its exact path and its size-stripped base.
func (i *urlIndex) add(key, localPath string, width int) {
	if _, exists := i.exact[key]; !exists {
		i.exact[key] = localPath
	}

	base, suffixWidth, hasSuffix := splitSizeSuffix(key)
	if hasSuffix && width == 0 {
		width = suffixWidth
	}
	if !hasSuffix {
		base = key
	}

	i.variants[base] = append(i.variants[base], sizeVariant{width: width, localPath: localPath})
}

// resolve returns the exported path for a normalized upload path. The second
// return value reports whether the hit came from the nearest-width fallback
// rather than an exact match.
func (i *urlIndex) resolve(key string) (localPath string, remapped, ok bool) {
	if p, exists := i.exact[key]; exists {
		return p, false, true
	}

	// The reference points at a size WordPress no longer generates — a
	// registered-size change regenerates thumbnails but never rewrites the
	// markup that already links to the old dimensions. Fall back to the closest
	// surviving width instead of emitting a dead path.
	base, width, hasSuffix := splitSizeSuffix(key)
	if !hasSuffix {
		return "", false, false
	}

	nearest, exists := nearestVariant(i.variants[base], width)
	if !exists {
		return "", false, false
	}

	return nearest, true, true
}

// nearestVariant picks the rendition whose width is closest to want, preferring
// the larger candidate on a tie so the image is never upscaled by the browser.
func nearestVariant(variants []sizeVariant, want int) (string, bool) {
	best := ""
	bestWidth := -1
	bestDelta := -1

	for _, variant := range variants {
		delta := variant.width - want
		if delta < 0 {
			delta = -delta
		}

		if bestDelta == -1 || delta < bestDelta || (delta == bestDelta && variant.width > bestWidth) {
			best = variant.localPath
			bestWidth = variant.width
			bestDelta = delta
		}
	}

	return best, best != ""
}

// localMediaPath turns a media-relative file path into the URL written to the
// exported content, honoring the configured path style.
func (d *Downloader) localMediaPath(relativePath string) string {
	joined := path.Join("media", filepath.ToSlash(relativePath))

	if d.config != nil && d.config.MediaPathStyle == PathStyleRelative {
		return joined
	}

	return "/" + joined
}

// normalizeURLKey reduces a URL to a host- and scheme-independent lookup key:
// its decoded path, without query or fragment.
func normalizeURLKey(raw string) (string, bool) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Path == "" {
		return "", false
	}

	cleaned := path.Clean(parsed.Path)
	if cleaned == "." || cleaned == "/" {
		return "", false
	}

	return strings.ToLower(cleaned), true
}

// splitSizeSuffix strips a WordPress "-<width>x<height>" suffix from a path,
// returning the base path and the referenced width.
func splitSizeSuffix(key string) (base string, width int, ok bool) {
	ext := path.Ext(key)
	stem := strings.TrimSuffix(key, ext)

	matches := sizeSuffixPattern.FindStringSubmatch(stem)
	if matches == nil {
		return "", 0, false
	}

	width, err := strconv.Atoi(matches[2])
	if err != nil {
		return "", 0, false
	}

	return matches[1] + ext, width, true
}

// toInt reads a width/height field, which the WordPress REST API delivers as a
// JSON number (float64) but which may also arrive as a string or int.
func toInt(value interface{}) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case float32:
		return int(typed)
	case string:
		parsed, err := strconv.Atoi(typed)
		if err != nil {
			return 0
		}
		return parsed
	default:
		return 0
	}
}
