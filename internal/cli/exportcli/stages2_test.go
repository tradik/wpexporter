package exportcli

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tradik/wpexporter/internal/api"
	"github.com/tradik/wpexporter/internal/bruteforce"
	"github.com/tradik/wpexporter/internal/checkpoint"
	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/internal/export"
	"github.com/tradik/wpexporter/pkg/models"
)

// clientBreaking is the real REST client with one collection broken.
type clientBreaking struct {
	*api.Client
	broken string
}

func (c clientBreaking) GetSiteInfo() (*models.SiteInfo, error) {
	if c.broken == "site" {
		return nil, errors.New("source broke")
	}
	return c.Client.GetSiteInfo()
}

func (c clientBreaking) GetMedia() ([]models.WordPressMedia, error) {
	if c.broken == "media" {
		return nil, errors.New("source broke")
	}
	return c.Client.GetMedia()
}

func (c clientBreaking) GetCategories() ([]models.WordPressCategory, error) {
	if c.broken == "categories" {
		return nil, errors.New("source broke")
	}
	return c.Client.GetCategories()
}

// Every stage's hard failure ends runRemote, with nothing written.
func TestRunRemoteStopsAtTheBrokenStage(t *testing.T) {
	quietMode = true
	t.Cleanup(func() { quietMode = false })
	srv := fakeWordPress(t)

	for _, broken := range []string{"site", "media", "categories"} {
		cfg := config.DefaultConfig()
		cfg.URL, cfg.Format, cfg.Retries, cfg.Output = srv.URL, "json", 0, filepath.Join(t.TempDir(), "out")
		client, err := api.NewClient(cfg)
		require.NoError(t, err)
		r := &exportRun{cfg: cfg, apiClient: client, src: clientBreaking{client, broken}, stated: client,
			exporter: export.NewExporter(cfg), scanner: bruteforce.NewScanner(cfg, client),
			checkpointMgr: checkpoint.NewManager(cfg.Output, false)}

		err = r.runRemote()
		require.Error(t, err, broken)
		assert.NoFileExists(t, filepath.Join(cfg.Output, "export.json"))
	}

	// And a write that fails is the run's failure too.
	cfg := config.DefaultConfig()
	cfg.URL, cfg.Format, cfg.Retries, cfg.NoInventoryCheck = srv.URL, "nope", 0, true
	cfg.Output = filepath.Join(t.TempDir(), "out")
	client, _ := api.NewClient(cfg)
	r := &exportRun{cfg: cfg, apiClient: client, src: client, stated: client, exporter: export.NewExporter(cfg),
		scanner: bruteforce.NewScanner(cfg, client), checkpointMgr: checkpoint.NewManager(cfg.Output, false)}
	assert.Error(t, r.runRemote())
}

// failingScanner is a scanner whose discovery fails.
type failingScanner struct{}

func (failingScanner) ScanForContent([]models.WordPressPost, []models.WordPressPost,
	[]models.WordPressMedia) (*bruteforce.ScanResult, error) {
	return nil, errors.New("scan broke")
}
func (failingScanner) ScanSpecificRange(string, int, int) (interface{}, error) { return nil, nil }

func TestDiscoverFailures(t *testing.T) {
	quietMode = true
	t.Cleanup(func() { quietMode = false })

	cfg := config.DefaultConfig()
	cfg.BruteForce = true
	r := &exportRun{cfg: cfg, scanner: failingScanner{}}
	assert.ErrorContains(t, r.discover(), "brute force scan failed")

	cfg.BruteForce, cfg.ScanRange = false, "9-1"
	assert.ErrorContains(t, r.discover(), "scan-range failed")

	srv := fakeWordPressWith(t, map[string]http.HandlerFunc{"/wp-json/wp/v2/categories": answer(200, "[]")})
	cfg.URL, cfg.ScanRange = srv.URL, ""
	client, _ := api.NewClient(cfg)
	r = &exportRun{cfg: cfg, apiClient: client, src: client, scanner: failingScanner{}}
	cfg.BruteForce = true
	assert.ErrorContains(t, r.fetchRemoteRest(), "brute force")
}

func TestFinishCheckpointCannotDelete(t *testing.T) {
	dir := t.TempDir()
	// The checkpoint's path is a directory with something in it.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".wpexport_checkpoint.json", "x"), 0o750))
	cfg := config.DefaultConfig()
	cfg.Resume = true
	r := &exportRun{cfg: cfg, checkpointMgr: checkpoint.NewManager(dir, true)}
	log := captureStdout(t, r.finishCheckpoint)
	assert.Contains(t, log, "failed to delete checkpoint")
}

func TestWriteCannotRemoveFiles(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root removes anything")
	}
	quietMode = true
	t.Cleanup(func() { quietMode = false })

	cfg := config.DefaultConfig()
	cfg.Format, cfg.DownloadMedia, cfg.CreateZip, cfg.NoFiles = "json", false, true, true
	cfg.Output = filepath.Join(t.TempDir(), "out")
	locked := filepath.Join(cfg.Output, "locked")
	require.NoError(t, os.MkdirAll(locked, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(locked, "f"), nil, 0o600))
	require.NoError(t, os.Chmod(locked, 0o500))       // #nosec G302 -- a read-only directory is the point
	t.Cleanup(func() { _ = os.Chmod(locked, 0o750) }) // #nosec G302 -- restored so the temp dir can go

	r := &exportRun{cfg: cfg, exporter: export.NewExporter(cfg)}
	_, err := r.write(&models.ExportData{})
	assert.ErrorContains(t, err, "remove export files")
}
