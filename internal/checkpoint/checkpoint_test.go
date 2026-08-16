package checkpoint

// The state behind --resume, which had no tests at all.
//
// It is the one part of the exporter whose whole job is to be correct after a
// crash: an export of a large site that dies on page 40 is resumable only if
// what was written to disk describes exactly what had been fetched. A mistake
// here is not a wrong file — it is a second run that silently skips a section
// it never actually read.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const siteURL = "https://x.test"

// TestNewCheckpointStartsAtPageOne: a fresh state describes an export that has
// fetched nothing, and pagination starts where WordPress does.
func TestNewCheckpointStartsAtPageOne(t *testing.T) {
	manager := NewManager(t.TempDir(), true)

	state, err := manager.Load(siteURL)
	require.NoError(t, err)
	require.NotNil(t, state)

	assert.Equal(t, siteURL, state.SiteURL)
	assert.Equal(t, 1, state.GetPostsPage())
	assert.Equal(t, 1, state.GetPagesPage())
	assert.Equal(t, 1, state.GetProductsPage())
	assert.Equal(t, 1, state.GetMediaPage())
	assert.False(t, state.IsPostsCompleted())
	assert.False(t, manager.Exists(), "nothing is written until the first save")
	assert.True(t, manager.IsEnabled())
	assert.Equal(t, state, manager.GetState())
}

// TestDisabledManagerKeepsNothing: without --resume the manager still hands out
// a state to write into, and writes none of it to disk.
func TestDisabledManagerKeepsNothing(t *testing.T) {
	dir := t.TempDir()
	manager := NewManager(dir, false)

	state, err := manager.Load(siteURL)
	require.NoError(t, err)
	require.NotNil(t, state)

	require.NoError(t, manager.Save())
	assert.NoFileExists(t, filepath.Join(dir, ".wpexport_checkpoint.json"))
	assert.False(t, manager.IsEnabled())
	require.NoError(t, manager.Delete(), "there is nothing to delete, and that is not an error")
}

// TestCheckpointSurvivesARestart: the whole point. What one run recorded is
// what the next run reads back.
func TestCheckpointSurvivesARestart(t *testing.T) {
	dir := t.TempDir()
	manager := NewManager(dir, true)

	state, err := manager.Load(siteURL)
	require.NoError(t, err)

	state.SetPostsPage(4)
	state.AddPostIDs([]int{1, 2, 3})
	state.SetPostsCompleted()
	state.SetPagesPage(2)
	state.AddPageIDs([]int{7})
	state.SetProductsPage(3)
	state.AddProductIDs([]int{11, 12})
	state.SetMediaPage(5)
	state.AddMediaIDs([]int{21})
	state.MarkMediaDownloaded("https://x.test/a.jpg")
	state.SetLastError("500 on page 5")

	require.NoError(t, manager.Save())
	require.FileExists(t, manager.GetFilePath())
	assert.True(t, manager.Exists())

	resumed, err := NewManager(dir, true).Load(siteURL)
	require.NoError(t, err)

	assert.Equal(t, 4, resumed.GetPostsPage())
	assert.Equal(t, []int{1, 2, 3}, resumed.PostIDs)
	assert.True(t, resumed.IsPostsCompleted())
	assert.Equal(t, 2, resumed.GetPagesPage())
	assert.Equal(t, 3, resumed.GetProductsPage())
	assert.Equal(t, 5, resumed.GetMediaPage())
	assert.True(t, resumed.IsMediaDownloaded("https://x.test/a.jpg"))
	assert.False(t, resumed.IsMediaDownloaded("https://x.test/b.jpg"))
	assert.Equal(t, "500 on page 5", resumed.LastError)
}

// TestCompletionFlagsRoundTrip: every collection records its own completion,
// because resuming means skipping only what is genuinely finished.
func TestCompletionFlagsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	manager := NewManager(dir, true)

	state, err := manager.Load(siteURL)
	require.NoError(t, err)

	state.SetPagesCompleted()
	state.SetProductsCompleted()
	state.SetMediaCompleted()
	state.SetCategoriesCompleted()
	state.SetTagsCompleted()
	state.SetUsersCompleted()
	require.NoError(t, manager.Save())

	resumed, err := NewManager(dir, true).Load(siteURL)
	require.NoError(t, err)

	assert.False(t, resumed.IsPostsCompleted(), "the one collection never marked stays unfinished")
	assert.True(t, resumed.IsPagesCompleted())
	assert.True(t, resumed.IsProductsCompleted())
	assert.True(t, resumed.IsMediaCompleted())
	assert.True(t, resumed.IsCategoriesCompleted())
	assert.True(t, resumed.IsTagsCompleted())
	assert.True(t, resumed.IsUsersCompleted())
}

// TestCheckpointRefusesAnotherSite: resuming one site's export into another's
// output would merge two sites into one export, silently.
func TestCheckpointRefusesAnotherSite(t *testing.T) {
	dir := t.TempDir()
	manager := NewManager(dir, true)

	_, err := manager.Load(siteURL)
	require.NoError(t, err)
	require.NoError(t, manager.Save())

	_, err = NewManager(dir, true).Load("https://other.test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "different site")
}

// TestCheckpointRejectsRubbish: a truncated or hand-edited file is an error,
// not an empty export that looks like a fresh start.
func TestCheckpointRejectsRubbish(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".wpexport_checkpoint.json"),
		[]byte("{not json"), 0600))

	_, err := NewManager(dir, true).Load(siteURL)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse checkpoint")
}

// TestCheckpointFileWithoutMediaMap: a checkpoint written before the media map
// existed, or hand-edited, must not panic the run that reads it.
func TestCheckpointFileWithoutMediaMap(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".wpexport_checkpoint.json"),
		[]byte(`{"site_url":"`+siteURL+`","posts_page":2}`), 0600))

	state, err := NewManager(dir, true).Load(siteURL)
	require.NoError(t, err)

	assert.Equal(t, 2, state.GetPostsPage())
	assert.False(t, state.IsMediaDownloaded("https://x.test/a.jpg"))
	state.MarkMediaDownloaded("https://x.test/a.jpg")
	assert.True(t, state.IsMediaDownloaded("https://x.test/a.jpg"))
}

// TestDeleteRemovesTheCheckpoint: a finished export leaves nothing to resume,
// and deleting what is already gone is not a failure.
func TestDeleteRemovesTheCheckpoint(t *testing.T) {
	dir := t.TempDir()
	manager := NewManager(dir, true)

	_, err := manager.Load(siteURL)
	require.NoError(t, err)
	require.NoError(t, manager.Save())
	require.True(t, manager.Exists())

	require.NoError(t, manager.Delete())
	assert.False(t, manager.Exists())
	require.NoError(t, manager.Delete())
}

// TestSaveCreatesTheDirectory: the checkpoint is written beside the export,
// which may not exist yet when the first page comes back.
func TestSaveCreatesTheDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "export")
	manager := NewManager(dir, true)

	_, err := manager.Load(siteURL)
	require.NoError(t, err)
	require.NoError(t, manager.Save())

	assert.FileExists(t, filepath.Join(dir, ".wpexport_checkpoint.json"))
}

// TestSaveWithoutStateIsNoOp: nothing was loaded, so there is nothing to write.
func TestSaveWithoutStateIsNoOp(t *testing.T) {
	dir := t.TempDir()
	manager := NewManager(dir, true)

	require.NoError(t, manager.Save())
	assert.False(t, manager.Exists())

	// SetState is how a caller hands the manager a state it built itself.
	manager.SetState(&State{SiteURL: siteURL, DownloadedMedia: map[string]bool{}})
	require.NoError(t, manager.Save())
	assert.True(t, manager.Exists())
}

// TestSummaryCountsWhatWasFetched: the line a resumed run prints, so an
// operator can see what the previous one got through.
func TestSummaryCountsWhatWasFetched(t *testing.T) {
	state := &State{DownloadedMedia: map[string]bool{}}
	state.AddPostIDs([]int{1, 2})
	state.AddPageIDs([]int{3})
	state.SetPostsCompleted()
	state.MarkMediaDownloaded("https://x.test/a.jpg")

	summary := state.Summary()
	assert.Contains(t, summary, "posts=2 (done=true)")
	assert.Contains(t, summary, "pages=1 (done=false)")
	assert.Contains(t, summary, "downloaded=1")
}

// TestStateIsSafeUnderConcurrency: the fetch loops that write this state run
// concurrently with the media downloader that reads it.
func TestStateIsSafeUnderConcurrency(t *testing.T) {
	state := &State{DownloadedMedia: map[string]bool{}}

	var wait sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wait.Add(1)

		go func(n int) {
			defer wait.Done()

			state.AddPostIDs([]int{n})
			state.SetPostsPage(n)
			state.MarkMediaDownloaded("https://x.test/" + string(rune('a'+n)) + ".jpg")
			_ = state.IsMediaDownloaded("https://x.test/a.jpg")
			_ = state.Summary()
		}(worker)
	}
	wait.Wait()

	assert.Len(t, state.PostIDs, 8)
	assert.Len(t, state.DownloadedMedia, 8)
}

// TestCheckpointIsPlainJSON: the file is meant to be readable and hand-fixable
// when an export goes wrong at three in the morning.
func TestCheckpointIsPlainJSON(t *testing.T) {
	dir := t.TempDir()
	manager := NewManager(dir, true)

	state, err := manager.Load(siteURL)
	require.NoError(t, err)
	state.AddPostIDs([]int{5})
	require.NoError(t, manager.Save())

	raw, err := os.ReadFile(manager.GetFilePath())
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(raw, &decoded))
	assert.Equal(t, siteURL, decoded["site_url"])
	assert.Contains(t, string(raw), "\n  ", "indented, so a person can read it")
}
