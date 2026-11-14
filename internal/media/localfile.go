package media

// Media read from disk, for local mode (FEAT-001).
//
// When the export runs on the WordPress server, the media library is a
// directory on the same disk: reading it beats fetching every file back
// through the web server. The download path, limits and file naming are
// unchanged — only where the bytes come from is.

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// localSource is the outcome of looking for a URL on disk.
type localSource int

const (
	// notLocal: the URL is not under the uploads address; fetch it.
	notLocal localSource = iota
	// onDisk: the file is in the uploads directory; copy it.
	onDisk
	// refused: the URL is under the uploads address but resolves outside
	// the uploads directory (.., a symlink out); neither copy nor fetch it.
	refused
)

// locate maps a media URL to its file in the uploads directory. The result is
// checked after symlinks are resolved: it must stay under the uploads
// directory, itself resolved, so neither ../ nor a planted symlink can read a
// file the operator did not mean to export.
func (d *Downloader) locate(mediaURL string) (string, localSource) {
	base, dir := strings.TrimRight(d.config.UploadsURL, "/"), d.config.UploadsDir
	if base == "" || dir == "" || !strings.HasPrefix(mediaURL, base+"/") {
		return "", notLocal
	}

	rel := strings.TrimPrefix(mediaURL, base+"/")
	if i := strings.IndexAny(rel, "?#"); i >= 0 {
		rel = rel[:i]
	}
	rel, err := url.PathUnescape(rel)
	if err != nil {
		return "", refused
	}

	root, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", notLocal // no uploads directory here after all
	}
	resolved, err := filepath.EvalSymlinks(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return "", notLocal // not on disk: the web server may still have it
	}
	inside, err := filepath.Rel(root, resolved)
	if err != nil || inside == ".." || strings.HasPrefix(inside, ".."+string(filepath.Separator)) {
		return "", refused
	}
	return resolved, onDisk
}

// copyLocal copies src to filePath under the same size limit a download has.
func (d *Downloader) copyLocal(src, filePath string) bool {
	in, err := os.Open(src) // #nosec G304 -- resolved and confined to the uploads directory by locate
	if err != nil {
		return false
	}
	defer func() { _ = in.Close() }()

	if info, err := in.Stat(); err != nil || !info.Mode().IsRegular() {
		return false
	}

	out, err := os.Create(filepath.Clean(filePath))
	if err != nil {
		return false
	}
	defer func() { _ = out.Close() }()

	// Bounded exactly as a download is, and a partial file is not kept.
	limit := d.maxMediaBytes()
	n, err := io.Copy(out, io.LimitReader(in, limit+1))
	if err != nil || n > limit {
		_ = os.Remove(filepath.Clean(filePath))
		d.logf("Media exceeds size limit (%d bytes) or could not be read, skipped: %s\n", limit, src)
		return false
	}
	return true
}

// logf prints when the run is verbose.
func (d *Downloader) logf(format string, args ...any) {
	if d.config.Verbose {
		fmt.Printf(format, args...)
	}
}
