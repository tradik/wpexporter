package rx

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetReturnsTheSameCompiledPattern: the point of the package. A second ask
// for a pattern must not compile it again, which is what makes the per-element
// call sites affordable.
func TestGetReturnsTheSameCompiledPattern(t *testing.T) {
	first := Get(`(?i)<div\b[^>]*>`)
	second := Get(`(?i)<div\b[^>]*>`)

	assert.Same(t, first, second)
	assert.True(t, first.MatchString(`<DIV class="x">`))
}

// TestGetIsSafeUnderConcurrency: the crawler asks from several goroutines at
// once, one per page in flight.
func TestGetIsSafeUnderConcurrency(t *testing.T) {
	var wg sync.WaitGroup

	results := make([]string, 16)

	for i := range results {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			results[i] = Get(`\bclass\s*=\s*"x"`).String()
		}(i)
	}

	wg.Wait()

	for _, result := range results {
		require.Equal(t, `\bclass\s*=\s*"x"`, result)
	}
}

// TestGetPanicsOnABadPattern: every pattern here is built by this program from
// its own literals, so one that will not compile is a bug to fix rather than
// input to handle — and it must say so loudly.
func TestGetPanicsOnABadPattern(t *testing.T) {
	assert.Panics(t, func() { Get(`(unclosed`) })
}
