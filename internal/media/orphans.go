package media

// Media referenced by content but absent from the media library (#30).
//
// The library is not the whole story. Elementor writes its own cropped
// renditions to /wp-content/uploads/elementor/thumbs/, page builders inline
// background images, and older posts point at files whose attachment record was
// deleted. None of those appear in /wp/v2/media, so the rewriter has nothing to
// resolve them to and they stay absolute — a "migrated" site that still fetches
// hundreds of images from the source host, and breaks the day it goes away.
//
// These are found by looking at what the rewriter could NOT resolve, downloaded
// alongside the library, and registered so the same rewrite pass reaches them.
// Only same-host URLs: a CDN or third-party image is somebody else's file and
// correctly stays where it is.

import (
	"crypto/sha1" // #nosec G505 -- a short, stable filename discriminator, not a security primitive
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// orphanExtensions are the file types worth carrying over. A same-host URL to a
// .php endpoint or a stylesheet is not media.
var orphanExtensions = map[string]string{
	".jpg": "images", ".jpeg": "images", ".png": "images", ".gif": "images",
	".webp": "images", ".avif": "images", ".svg": "images", ".bmp": "images",
	".ico": "images", ".heic": "images",
	".mp4": "videos", ".webm": "videos", ".mov": "videos", ".m4v": "videos",
	".avi": "videos", ".ogv": "videos",
	".mp3": "audio", ".wav": "audio", ".ogg": "audio", ".flac": "audio",
	".m4a": "audio", ".aac": "audio",
	".pdf": "documents", ".doc": "documents", ".docx": "documents",
	".xls": "documents", ".xlsx": "documents", ".ppt": "documents",
	".pptx": "documents", ".zip": "documents", ".txt": "documents",
}

// UnresolvedURLs returns the same-host media URLs in text that this rewriter
// cannot map to an exported file, deduplicated and in a stable order.
func (r *URLRewriter) UnresolvedURLs(texts ...string) []string {
	seen := map[string]bool{}

	for _, text := range texts {
		for _, token := range urlTokenPattern.FindAllString(text, -1) {
			absolute := r.absoluteURL(token)
			if absolute == "" || seen[absolute] {
				continue
			}
			if key, ok := normalizeURLKey(token); ok {
				if _, _, resolved := r.index.resolve(key); resolved {
					continue // the library already covers it
				}
			}
			seen[absolute] = true
		}
	}

	out := make([]string, 0, len(seen))
	for u := range seen {
		out = append(out, u)
	}
	sort.Strings(out)
	return out
}

// absoluteURL turns a token into an absolute same-host media URL, or "" when it
// is not one: a different host, or a path that does not name a media file.
func (r *URLRewriter) absoluteURL(token string) string {
	if r.config == nil || r.config.URL == "" {
		return ""
	}
	base, err := url.Parse(r.config.URL)
	if err != nil {
		return ""
	}

	ref, err := url.Parse(token)
	if err != nil {
		return ""
	}
	resolved := base.ResolveReference(ref)
	if resolved.Host != base.Host {
		return ""
	}
	if orphanSubfolder(resolved.Path) == "" {
		return ""
	}
	resolved.Fragment = ""
	return resolved.String()
}

// orphanSubfolder maps a path's extension to the media subfolder it belongs in,
// or "" when the path does not name a media file.
func orphanSubfolder(urlPath string) string {
	decoded, err := url.PathUnescape(urlPath)
	if err != nil {
		decoded = urlPath
	}
	return orphanExtensions[strings.ToLower(path.Ext(decoded))]
}

// DownloadOrphans fetches media that no attachment record covers and returns
// each source URL mapped to its exported path. A download that fails is left
// out rather than recorded as a local file that does not exist.
func (d *Downloader) DownloadOrphans(urls []string) map[string]string {
	if d.config == nil || !d.config.DownloadMedia || len(urls) == 0 {
		return nil
	}

	mapping := make(map[string]string, len(urls))
	taken := map[string]bool{}

	for _, rawURL := range urls {
		subfolder := orphanSubfolder(rawURL)
		if subfolder == "" {
			continue
		}
		name := d.orphanFilename(rawURL, taken)
		dir := filepath.Join(d.mediaDir, subfolder)
		// #nosec G301 -- media directories are web content, mirroring DownloadMedia
		if err := os.MkdirAll(dir, 0755); err != nil {
			continue
		}
		if !d.downloadFile(rawURL, filepath.Join(dir, name)) {
			continue
		}
		taken[name] = true
		mapping[rawURL] = d.localMediaPath(path.Join(subfolder, name))
	}
	return mapping
}

// orphanFilename derives a stable, collision-free name from the URL. The path
// hash prefix keeps two files that share a basename apart — Elementor's cropped
// renditions repeat a name across directories — while staying identical between
// runs, so a re-export does not churn every filename.
func (d *Downloader) orphanFilename(rawURL string, taken map[string]bool) string {
	parsed, err := url.Parse(rawURL)
	base := "file"
	if err == nil && parsed.Path != "" {
		if decoded, decErr := url.PathUnescape(parsed.Path); decErr == nil {
			base = path.Base(decoded)
		} else {
			base = path.Base(parsed.Path)
		}
	}
	base = d.sanitizeFilename(base)

	sum := sha1.Sum([]byte(rawURL)) // #nosec G401 -- filename discriminator, not a security primitive
	name := hex.EncodeToString(sum[:])[:8] + "_" + base
	if !taken[name] {
		return name
	}
	// The same URL cannot appear twice (input is deduplicated), so a clash here
	// means two URLs hashed alike; number the second one rather than overwrite.
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%d_%s", i, name)
		if !taken[candidate] {
			return candidate
		}
	}
}

// AddOrphans registers downloaded orphan media so Rewrite resolves them like
// any attachment.
func (r *URLRewriter) AddOrphans(mapping map[string]string) {
	if len(mapping) == 0 {
		return
	}
	if r.index == nil {
		r.index = &urlIndex{exact: map[string]string{}, variants: map[string][]sizeVariant{}}
	}
	for sourceURL, localPath := range mapping {
		if key, ok := normalizeURLKey(sourceURL); ok {
			r.index.exact[key] = localPath
		}
	}
}
