package api

// Benchmarks for the work this program spends most of its time on: turning
// pages of the REST API into records. Kept in the tree because the Go 1.27
// upgrade was justified by this number, and a later change that undoes it
// should have to argue with a measurement rather than with a memory.

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/tradik/wpexporter/pkg/models"
)

// postsPayload is what /wp/v2/posts?per_page=100 actually answers with: a
// hundred records, each carrying rendered HTML, terms, dates and a link.
func postsPayload() []byte {
	records := make([]string, 0, 100)

	for i := 1; i <= 100; i++ {
		records = append(records, fmt.Sprintf(`{"id":%d,"date":"2026-03-04T10:00:00",`+
			`"modified":"2026-03-05T11:00:00","slug":"wpis-%d","status":"publish","type":"post",`+
			`"link":"https://x.test/2026/03/wpis-%d/",`+
			`"title":{"rendered":"Tytul wpisu numer %d"},`+
			`"content":{"rendered":"%s"},`+
			`"excerpt":{"rendered":"<p>Zajawka wpisu numer %d.</p>"},`+
			`"author":3,"featured_media":%d,"categories":[5,9],"tags":[11,12,13],"sticky":false}`,
			i, i, i, i, strings.Repeat(`<p>Akapit tresci wpisu, ktory ma dosc slow by cos wazyc.</p>`, 12), i, i))
	}

	return []byte("[" + strings.Join(records, ",") + "]")
}

// BenchmarkDecodePostsPage measures the work this program spends most of its
// time on: turning a page of the REST API into records.
func BenchmarkDecodePostsPage(b *testing.B) {
	payload := postsPayload()
	b.ReportAllocs()
	b.SetBytes(int64(len(payload)))

	for b.Loop() {
		var posts []models.WordPressPost
		if err := json.Unmarshal(payload, &posts); err != nil {
			b.Fatal(err)
		}
	}
}
