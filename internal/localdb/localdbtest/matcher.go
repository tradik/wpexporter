// Package localdbtest is a canned WordPress database for tests: a sqlmock
// whose matcher enforces the local source's safety rules on every statement,
// and a small site to read. It is imported by tests only.
package localdbtest

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/DATA-DOG/go-sqlmock"
)

// forbiddenColumns must never appear in any statement (FEAT-001 §4).
var forbiddenColumns = regexp.MustCompile(`(?i)\b(user_pass|user_email|user_login|user_activation_key|` +
	`comment_author_email|comment_author_IP|comment_agent|session_tokens|\w*_capabilities)\b`)

// writeKeywords must never start or appear in any statement.
var writeKeywords = regexp.MustCompile(`(?i)\b(INSERT|UPDATE|DELETE|ALTER|LOCK|UNLOCK|DROP|CREATE|REPLACE|` +
	`TRUNCATE|GRANT|RENAME|HANDLER|LOAD|CALL)\b`)

// Recorder keeps every statement the code under test sent, and every one that
// broke a rule.
type Recorder struct {
	mu         sync.Mutex
	statements map[string]bool
	violations []string
}

// Statements returns the distinct statements seen, sorted.
func (r *Recorder) Statements() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(r.statements))
	for s := range r.statements {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// Violations returns the statements that broke a rule.
func (r *Recorder) Violations() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.violations...)
}

// Check reports why a statement must never be sent, or nil.
func Check(statement string) error {
	if m := forbiddenColumns.FindString(statement); m != "" {
		return fmt.Errorf("statement reads forbidden column %s", m)
	}
	if m := writeKeywords.FindString(statement); m != "" {
		return fmt.Errorf("statement contains write keyword %s", m)
	}
	return nil
}

// Matcher matches a statement containing the expected fragment, and refuses —
// recording a violation — any statement Check rejects.
func (r *Recorder) Matcher() sqlmock.QueryMatcher {
	return sqlmock.QueryMatcherFunc(func(expected, actual string) error {
		r.mu.Lock()
		if r.statements == nil {
			r.statements = map[string]bool{}
		}
		first := !r.statements[actual]
		r.statements[actual] = true
		err := Check(actual)
		if err != nil && first {
			r.violations = append(r.violations, actual)
		}
		r.mu.Unlock()
		if err != nil {
			return err
		}
		if !strings.Contains(actual, expected) {
			return fmt.Errorf("statement does not contain %q", expected)
		}
		return nil
	})
}
