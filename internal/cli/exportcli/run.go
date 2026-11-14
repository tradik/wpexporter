package exportcli

// One export, from configuration to summary.
//
// runExport used to be a single 660-line function. It is now a short list of
// stages over one exportRun: the stages that read the site belong to a source
// (remote.go for the REST API, local.go for the database), and everything
// after reading — filtering, crawling, media, conversion, writing, the
// summary — is shared (stages.go, finish.go). The REST stages run in exactly
// the order they always did.

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/tradik/wpexporter/internal/api"
	"github.com/tradik/wpexporter/internal/bruteforce"
	"github.com/tradik/wpexporter/internal/cache"
	"github.com/tradik/wpexporter/internal/checkpoint"
	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/internal/export"
	"github.com/tradik/wpexporter/pkg/models"
)

// collected is everything a source read, and what the run learned about it.
type collected struct {
	siteInfo       *models.SiteInfo
	posts, pages   []models.WordPressPost
	products       []models.WooCommerceProduct
	productsNotice string
	customTypes    []models.CustomTypeSet
	media          []models.WordPressMedia
	categories     []models.WordPressCategory
	tags           []models.WordPressTag
	users          []models.WordPressUser
	menus          []models.WordPressMenu
	comments       []models.WordPressComment
	// gaps are collections the site would not read to the end; notices are
	// facts about the site. Both reach the summary and metadata.json.
	gaps, notices   []string
	bruteForceFound int
	analytics       *models.Analytics
	marketing       *models.SiteMarketing
}

// statedCounts answers the summary's "how many did the site say it had".
type statedCounts interface {
	StatedTotal(kind string) int
	Limited() bool
}

// exportRun is the state of one export.
type exportRun struct {
	cfg       *config.Config
	fileCache *cache.FileCache
	exporter  *export.Exporter
	startTime time.Time
	stated    statedCounts
	src       contentSource
	c         collected

	// Local only: the budget the database and the products client share.
	limits *api.Limits

	// REST only (in local mode, apiClient reads products through the origin).
	apiClient      *api.Client
	scanner        contentScanner
	checkpointMgr  *checkpoint.Manager
	saveCheckpoint func() error
}

func runExport(cmd *cobra.Command, args []string) error {
	cfg, err := prepareConfig(cmd)
	if err != nil {
		return err
	}
	if cfg.LocalPath != "" {
		return exportLocal(cmd, cfg)
	}
	if err := checkOutput(cfg); err != nil {
		return err
	}
	return exportRemote(cfg)
}

// exportRemote runs an export over the REST API.
func exportRemote(cfg *config.Config) error {
	r, closeRun, err := newRemoteRun(cfg)
	if err != nil {
		return err
	}
	defer closeRun()
	return r.runRemote()
}

// runRemote runs the REST stages, in the order they have always run.
func (r *exportRun) runRemote() error {
	if err := r.fetchRemoteDocuments(); err != nil {
		return err
	}
	r.shapeDocuments()
	if err := r.fetchRemoteMedia(); err != nil {
		return err
	}
	r.convertContent()
	if err := r.fetchRemoteRest(); err != nil {
		return err
	}

	exportData := r.assemble()
	r.checkInventory(exportData)
	zipPath, err := r.write(exportData)
	if err != nil {
		return err
	}
	r.summarize(exportData, zipPath)
	r.finishCheckpoint()
	return nil
}

// prepareConfig builds the run's configuration — defaults, config file, flags —
// and refuses one that cannot work before anything is fetched.
func prepareConfig(cmd *cobra.Command) (*config.Config, error) {
	// Start with default configuration
	cfg := config.DefaultConfig()

	// Load configuration file if specified or found
	if cfgFile != "" || configFileExists() {
		loadedCfg, err := config.LoadConfig(cfgFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load config: %w", err)
		}
		cfg = loadedCfg
	}

	// Override config with command line flags
	if err := applyFlagOverrides(cmd, cfg); err != nil {
		return nil, err
	}

	// Set global quiet mode for log functions
	quietMode = cfg.Quiet

	// Validate --no-files requires --zip
	if cfg.NoFiles && !cfg.CreateZip {
		return nil, fmt.Errorf("--no-files requires --zip flag")
	}

	// Validate --flat-html and --basic-html are mutually exclusive
	if cfg.FlatHTML && cfg.BasicHTML {
		return nil, fmt.Errorf("--flat-html and --basic-html are mutually exclusive")
	}

	return cfg, nil
}

// checkOutput validates the configuration and the output location.
func checkOutput(cfg *config.Config) error {
	// Generate default output path if not specified
	if err := cfg.GenerateDefaultOutput(); err != nil {
		return fmt.Errorf("failed to generate default output path: %w", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}
	if err := validateSource(cfg); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Fail before the export rather than after: a snap writing to /tmp reports
	// success into a private namespace the user cannot reach (issue #19).
	if err := snapPrivateTmpError(cfg.Output); err != nil {
		return err
	}

	// Check output directory permissions before starting expensive operations
	if err := checkOutputPermissions(cfg.Output); err != nil {
		return fmt.Errorf("output directory check failed: %w", err)
	}
	return nil
}

// newRemoteRun creates the API client, cache, scanner and checkpoint for a
// REST export, and announces the run.
func newRemoteRun(cfg *config.Config) (*exportRun, func(), error) {
	noop := func() {
		// Nothing was opened yet, so a failed start has nothing to close.
	}

	// Create API client
	apiClient, err := api.NewClient(cfg)
	if err != nil {
		return nil, noop, fmt.Errorf("failed to create API client: %w", err)
	}

	// Initialize cache if enabled
	fileCache, err := initializeCache(cfg, apiClient)
	if err != nil {
		return nil, noop, err
	}
	closeRun := noop
	if fileCache != nil {
		closeRun = func() { _ = fileCache.Close() }
	}

	r := &exportRun{
		cfg: cfg, apiClient: apiClient, src: apiClient, fileCache: fileCache, stated: apiClient,
		// Create exporter
		exporter: export.NewExporter(cfg),
		// Create brute force scanner
		scanner: bruteforce.NewScanner(cfg, apiClient),
		// Create checkpoint manager
		checkpointMgr: checkpoint.NewManager(cfg.Output, cfg.Resume),
	}

	// Load or create checkpoint state (the manager owns the state; see GetState()).
	if r.checkpointMgr.IsEnabled() {
		if _, err := r.checkpointMgr.Load(cfg.URL); err != nil {
			closeRun()
			return nil, noop, fmt.Errorf("failed to load checkpoint: %w", err)
		}
		if r.checkpointMgr.Exists() {
			logf("Resuming from checkpoint: %s\n", r.checkpointMgr.GetFilePath())
			logln(r.checkpointMgr.GetState().Summary())
		}
	}

	// Progress callback to save checkpoint after each page
	r.saveCheckpoint = func() error {
		return r.checkpointMgr.Save()
	}

	r.announce()
	return r, closeRun, nil
}

// announce prints what the run is about to do.
func (r *exportRun) announce() {
	cfg := r.cfg
	logf("Starting WordPress export from: %s\n", cfg.URL)
	logf("Output: %s (format: %s)\n", cfg.Output, cfg.Format)

	if cfg.BruteForce {
		logf("Brute force enabled (max ID: %d)\n", cfg.MaxID)
	}

	if cfg.DownloadMedia {
		logf("Media download enabled (concurrent: %d)\n", cfg.Concurrent)
	}

	r.startTime = time.Now()
}
