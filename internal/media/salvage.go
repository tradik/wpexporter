package media

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Salvaging the media the library does not list (#30).
//
// The downloader and the rewriter both work from /wp/v2/media, and three kinds of
// file never appear there: page-builder renditions (Elementor writes its own crops
// to uploads/elementor/thumbs/ with no attachment record), attachments whose record
// was deleted while the file is still served, and brand assets declared only in the
// document head. Content keeps pointing at all three, so an export that looks
// complete leaves the migrated site hotlinking the source host — until that host is
// retired and the images vanish.
//
// The salvage pass collects the same-host asset URLs the index cannot resolve,
// fetches them, and registers them so the ordinary rewrite pass reaches them.

// salvageExtensions is what the pass is willing to fetch. A same-host URL is not
// enough on its own: urlTokenPattern also matches page addresses, and following
// those would drag the whole site into media/.
var salvageExtensions = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true,
	".avif": true, ".svg": true, ".bmp": true, ".ico": true, ".tif": true,
	".tiff": true, ".heic": true, ".heif": true,
	".mp4": true, ".webm": true, ".mov": true, ".m4v": true, ".ogv": true,
	".mp3": true, ".wav": true, ".ogg": true, ".flac": true, ".m4a": true,
	".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true,
	".ppt": true, ".pptx": true, ".odt": true, ".ods": true, ".odp": true,
	".zip": true, ".rar": true, ".7z": true, ".tar": true, ".gz": true,
}

// AssetCollector gathers same-host asset URLs that the rewriter cannot resolve.
//
// It deduplicates on the same normalized key the index uses, so the identical file
// referenced as http:// in an old post and https:// in a new one is fetched once.
type AssetCollector struct {
	rewriter *URLRewriter
	seen     map[string]bool
	assets   []SalvageTarget
}

// SalvageTarget is one unresolved asset: the absolute URL to fetch and the
// normalized key every reference to it shares.
type SalvageTarget struct {
	URL string
	Key string
}

// NewAssetCollector starts a collection pass against this rewriter's index.
func (r *URLRewriter) NewAssetCollector() *AssetCollector {
	return &AssetCollector{rewriter: r, seen: map[string]bool{}}
}

// Scan records every same-host asset reference in text that the index cannot
// resolve. Anything already exported, on a foreign host, or not an asset is
// ignored — a CDN image is somebody else's file, and a page address is not media.
func (c *AssetCollector) Scan(text string) {
	if text == "" {
		return
	}

	for _, token := range urlTokenPattern.FindAllString(text, -1) {
		key, ok := normalizeURLKey(token)
		if !ok || c.seen[key] {
			continue
		}

		if _, _, resolved := c.rewriter.index.resolve(key); resolved {
			continue
		}

		if !salvageExtensions[strings.ToLower(path.Ext(key))] {
			continue
		}

		absolute, ok := c.rewriter.absoluteSameHost(token)
		if !ok {
			continue
		}

		c.seen[key] = true
		c.assets = append(c.assets, SalvageTarget{URL: absolute, Key: key})
	}
}

// Assets returns the collected targets in the order they were first seen.
func (c *AssetCollector) Assets() []SalvageTarget {
	return c.assets
}

// absoluteSameHost resolves a reference against the configured site and reports
// whether it belongs to that site. A root-relative path is the site's own by
// definition; an absolute URL must match the host, or it is a third party's.
func (r *URLRewriter) absoluteSameHost(rawURL string) (string, bool) {
	if r.config == nil || r.config.URL == "" {
		return "", false
	}

	base, err := url.Parse(r.config.URL)
	if err != nil {
		return "", false
	}

	ref, err := url.Parse(rawURL)
	if err != nil {
		return "", false
	}

	resolved := base.ResolveReference(ref)
	if !strings.EqualFold(resolved.Hostname(), base.Hostname()) {
		return "", false
	}

	return resolved.String(), true
}

// Register indexes an exported file under the remote key it was fetched from, so
// the ordinary rewrite pass resolves every reference to it.
func (r *URLRewriter) Register(key, localPath string) {
	if key == "" || localPath == "" {
		return
	}

	r.index.add(key, localPath, 0)
}

// SalvageAssets downloads each target and registers it with the rewriter,
// returning how many files were retrieved.
//
// Failures are not fatal: a deleted attachment may be referenced but no longer
// served, and one dead URL should not abandon the rest of the export. The URL then
// simply stays absolute, exactly as before this pass existed.
func (d *Downloader) SalvageAssets(rewriter *URLRewriter, targets []SalvageTarget) int {
	saved := 0

	for _, target := range targets {
		relative := salvageRelativePath(target.Key)
		if relative == "" {
			continue
		}

		if d.excludedSubfolder(path.Dir(relative)) {
			continue
		}

		absolutePath := filepath.Join(d.mediaDir, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(absolutePath), 0750); err != nil {
			continue
		}

		// A previous run already fetched it; register and move on rather than
		// re-downloading on every export.
		if _, err := os.Stat(absolutePath); err != nil {
			if !d.downloadFile(target.URL, absolutePath) {
				if d.config != nil && d.config.Verbose && !d.config.Quiet {
					fmt.Printf("  ⊘ Could not salvage %s\n", target.URL)
				}
				continue
			}
			saved++
		}

		rewriter.Register(target.Key, d.localMediaPath(relative))
	}

	return saved
}

// salvageRelativePath places a salvaged file under media/<kind>/ with a short hash
// of its source path.
//
// The hash is what makes the name unique: Elementor repeats basenames across its
// thumbs directories, so "post-52-copyright.jpg" from two different paths would
// otherwise overwrite each other. It is derived from the normalized path, so the
// same remote file lands on the same name on every run.
func salvageRelativePath(key string) string {
	base := path.Base(key)
	if base == "." || base == "/" || base == "" {
		return ""
	}

	sum := sha256.Sum256([]byte(key))
	prefix := hex.EncodeToString(sum[:4])

	ext := strings.ToLower(path.Ext(base))
	name := sanitizeSalvageName(strings.TrimSuffix(base, path.Ext(base)))

	return path.Join(salvageSubfolder(ext), prefix+"_"+name+ext)
}

// salvageSubfolder mirrors the MIME-based layout the library download uses, so a
// salvaged image sits beside the attachments in media/images/.
func salvageSubfolder(ext string) string {
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".avif", ".svg", ".bmp", ".ico", ".tif", ".tiff", ".heic", ".heif":
		return "images"
	case ".mp4", ".webm", ".mov", ".m4v", ".ogv":
		return "videos"
	case ".mp3", ".wav", ".ogg", ".flac", ".m4a":
		return "audio"
	case ".zip", ".rar", ".7z", ".tar", ".gz":
		return "archives"
	default:
		return "documents"
	}
}

// excludedSubfolder honors --exclude-media-types for salvaged files, so the pass
// cannot reintroduce a category the export was told to skip.
func (d *Downloader) excludedSubfolder(subfolder string) bool {
	if d.config == nil {
		return false
	}

	for _, excluded := range d.config.ExcludeMediaTypes {
		if strings.EqualFold(strings.TrimSpace(excluded), subfolder) {
			return true
		}
	}

	return false
}

// sanitizeSalvageName reduces a basename to filesystem-safe characters and a
// bounded length; the hash prefix already carries the uniqueness.
func sanitizeSalvageName(name string) string {
	var builder strings.Builder

	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			builder.WriteRune(r)
		default:
			builder.WriteRune('_')
		}
	}

	cleaned := strings.Trim(builder.String(), "_")
	if cleaned == "" {
		cleaned = "asset"
	}

	if len(cleaned) > 80 {
		cleaned = cleaned[:80]
	}

	return cleaned
}
