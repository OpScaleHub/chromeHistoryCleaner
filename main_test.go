package main

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestLikePattern(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"example.com", `%example.com%`},
		{"a_b", `%a\_b%`},
		{"50%", `%50\%%`},
		{`a\b`, `%a\\b%`},
		{"", `%%`},
	}
	for _, c := range cases {
		if got := likePattern(c.in); got != c.want {
			t.Errorf("likePattern(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestLikePatternMatchesLiterally verifies that wildcard characters in user
// input are treated literally by SQLite once the ESCAPE clause is applied,
// so a deletion targeting "a_b" cannot accidentally match "axb".
func TestLikePatternMatchesLiterally(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE urls (url TEXT)`); err != nil {
		t.Fatal(err)
	}
	for _, u := range []string{"http://a_b.com", "http://axb.com", "http://other.com"} {
		if _, err := db.Exec(`INSERT INTO urls (url) VALUES (?)`, u); err != nil {
			t.Fatal(err)
		}
	}

	n, err := scanCount(db, `SELECT COUNT(*) FROM urls WHERE url LIKE ? ESCAPE '\'`, likePattern("a_b"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("expected 1 literal match for %q, got %d", "a_b", n)
	}
}

func TestGetBaseDirNonEmpty(t *testing.T) {
	dir, err := getBaseDir()
	if err != nil {
		t.Fatal(err)
	}
	if dir == "" {
		t.Error("getBaseDir returned an empty path")
	}
}
