package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	_ "modernc.org/sqlite"
)

// Version info, injected at build time via ldflags:
//
//	go build -ldflags "-X main.Version=1.0.0 -X main.CommitHash=$(git rev-parse --short HEAD)"
var (
	Version    = "dev"
	CommitHash = "none"
)

// LocalState mirrors the relevant parts of Chrome's "Local State" JSON file.
type LocalState struct {
	Profile struct {
		InfoCache map[string]struct {
			Name  string `json:"name"`
			Email string `json:"user_name"`
		} `json:"info_cache"`
	} `json:"profile"`
}

// getBaseDir returns the platform-specific path to Chrome's user data directory.
func getBaseDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(home, "AppData", "Local", "Google", "Chrome", "User Data"), nil
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Google", "Chrome"), nil
	default:
		return filepath.Join(home, ".config", "google-chrome"), nil
	}
}

// listProfiles reads Chrome's Local State file and prints all detected profiles.
func listProfiles() error {
	baseDir, err := getBaseDir()
	if err != nil {
		return err
	}

	localStatePath := filepath.Join(baseDir, "Local State")
	data, err := os.ReadFile(localStatePath)
	if err != nil {
		return fmt.Errorf("could not read Local State file: %w", err)
	}

	var state LocalState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("could not parse Local State JSON: %w", err)
	}

	fmt.Printf("%-20s | %-20s | %-20s\n", "Directory", "Name", "Email")
	fmt.Println(strings.Repeat("-", 65))
	for dir, info := range state.Profile.InfoCache {
		fmt.Printf("%-20s | %-20s | %-20s\n", dir, info.Name, info.Email)
	}
	return nil
}

// isChromeRunning checks whether any Chrome process is currently running.
// It uses platform-appropriate process-listing commands.
func isChromeRunning() bool {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("tasklist", "/FI", "IMAGENAME eq chrome.exe", "/NH")
	case "darwin":
		cmd = exec.Command("pgrep", "-x", "Google Chrome")
	default: // Linux and other Unix-like systems
		// Match the process name exactly ("chrome") rather than scanning the
		// full command line with -f, which would also match this tool's own
		// "chrome-cleaner" binary and report a false positive.
		cmd = exec.Command("pgrep", "-x", "chrome")
	}
	output, err := cmd.Output()
	if err != nil {
		// pgrep returns exit code 1 when no processes match; that's not an error for us.
		return false
	}
	if runtime.GOOS == "windows" {
		return strings.Contains(string(output), "chrome.exe")
	}
	return len(strings.TrimSpace(string(output))) > 0
}

// openDB opens a SQLite database for exclusive sequential use by this CLI.
// It pins the pool to a single connection (SQLite tolerates only one writer and
// concurrent connections invite "database is locked"), verifies connectivity,
// and checkpoints any write-ahead log so that rows still pending in a "-wal"
// file from Chrome's last session are visible to our counts and deletes.
func openDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	// Best-effort: not all profiles use WAL mode, so ignore the error.
	_, _ = db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	return db, nil
}

// execTx runs fn inside a transaction. It commits on success and rolls back on error.
func execTx(db *sql.DB, fn func(tx *sql.Tx) error) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback() // best-effort rollback
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

// likePattern wraps a user-supplied substring in '%...%' for a SQL LIKE clause,
// escaping the LIKE metacharacters ('%', '_') and the escape character itself so
// that input such as "a_b" or "50%" matches literally instead of acting as a
// wildcard. All queries using the returned pattern MUST include `ESCAPE '\'`.
func likePattern(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + r.Replace(s) + "%"
}

// scanCount is a helper that runs a COUNT(*) query and returns the result.
func scanCount(db *sql.DB, query string, args ...any) (int, error) {
	var n int
	if err := db.QueryRow(query, args...).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func main() {
	// --- Flag definitions ---
	site := flag.String("site", "", "The domain or keyword to target for deletion")
	profile := flag.String("profile", "Default", "Chrome profile directory name (use -list-profiles to see available profiles)")
	dryRun := flag.Bool("dry-run", false, "Show impact report without deleting any data")
	showProfiles := flag.Bool("list-profiles", false, "List all detected Chrome profiles")
	showVersion := flag.Bool("version", false, "Print version information and exit")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Chrome History Cleaner — selectively remove browsing data from Chrome's local databases.\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  %s -site <domain> [-profile <name>] [-dry-run]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -list-profiles\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -version\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	// --- Version ---
	if *showVersion {
		fmt.Printf("chrome-cleaner %s (commit: %s)\n", Version, CommitHash)
		return
	}

	// --- List profiles ---
	if *showProfiles {
		if err := listProfiles(); err != nil {
			log.Fatalf("Error listing profiles: %v", err)
		}
		return
	}

	// --- Validate required flags ---
	if *site == "" {
		flag.Usage()
		os.Exit(1)
	}

	// --- Safety check ---
	if isChromeRunning() {
		log.Fatal("Error: Chrome is running. Please close it first.")
	}

	// --- Open databases ---
	baseDir, err := getBaseDir()
	if err != nil {
		log.Fatal(err)
	}
	profilePath := filepath.Join(baseDir, *profile)
	if fi, err := os.Stat(profilePath); err != nil || !fi.IsDir() {
		log.Fatalf("Profile %q not found at %s (use -list-profiles to see available profiles)", *profile, profilePath)
	}

	pattern := likePattern(*site)

	historyPath := filepath.Join(profilePath, "History")
	webDataPath := filepath.Join(profilePath, "Web Data")
	for _, p := range []string{historyPath, webDataPath} {
		if _, err := os.Stat(p); err != nil {
			// Bail out before sql.Open, which would otherwise silently create
			// an empty database file at this path.
			log.Fatalf("Required database not found: %s (%v)", p, err)
		}
	}

	historyDB, err := openDB(historyPath)
	if err != nil {
		log.Fatalf("Error opening History DB: %v", err)
	}
	defer historyDB.Close()

	webDB, err := openDB(webDataPath)
	if err != nil {
		log.Fatalf("Error opening Web Data DB: %v", err)
	}
	defer webDB.Close()

	// --- Gather statistics ---
	urlCount, err := scanCount(historyDB, `SELECT COUNT(*) FROM urls WHERE url LIKE ? ESCAPE '\'`, pattern)
	if err != nil {
		log.Fatalf("Error counting URLs: %v", err)
	}

	visitCount, err := scanCount(historyDB,
		`SELECT COUNT(*) FROM visits v JOIN urls u ON v.url = u.id WHERE u.url LIKE ? ESCAPE '\'`, pattern)
	if err != nil {
		log.Fatalf("Error counting visits: %v", err)
	}

	segmentCount, err := scanCount(historyDB,
		`SELECT COUNT(*) FROM segments WHERE url_id IN (SELECT id FROM urls WHERE url LIKE ? ESCAPE '\')`, pattern)
	if err != nil {
		log.Fatalf("Error counting segments: %v", err)
	}

	keywordSearchCount, err := scanCount(historyDB,
		`SELECT COUNT(*) FROM keyword_search_terms WHERE url_id IN (SELECT id FROM urls WHERE url LIKE ? ESCAPE '\')`, pattern)
	if err != nil {
		log.Fatalf("Error counting keyword search terms: %v", err)
	}

	keywordCount, err := scanCount(webDB,
		`SELECT COUNT(*) FROM keywords WHERE short_name LIKE ? ESCAPE '\' OR url LIKE ? ESCAPE '\'`, pattern, pattern)
	if err != nil {
		log.Fatalf("Error counting keywords: %v", err)
	}

	autofillCount, err := scanCount(webDB,
		`SELECT COUNT(*) FROM autofill WHERE value LIKE ? ESCAPE '\'`, pattern)
	if err != nil {
		log.Fatalf("Error counting autofill entries: %v", err)
	}

	fmt.Printf("--- Impact Report for '%s' (Profile: %s) ---\n", *site, *profile)
	fmt.Printf("History URLs:          %d\n", urlCount)
	fmt.Printf("Visit Records:         %d\n", visitCount)
	fmt.Printf("Segments:              %d\n", segmentCount)
	fmt.Printf("Keyword Search Terms:  %d\n", keywordSearchCount)
	fmt.Printf("Search Keywords:       %d\n", keywordCount)
	fmt.Printf("Autofill entries:      %d\n", autofillCount)

	total := urlCount + visitCount + segmentCount + keywordSearchCount + keywordCount + autofillCount
	if total == 0 {
		fmt.Println("No records found.")
		return
	}

	if *dryRun {
		fmt.Println("\nDry run completed. No data was modified.")
		return
	}

	// --- Confirm with user ---
	fmt.Print("\nProceed with deletion? (y/N): ")
	var confirm string
	fmt.Scanln(&confirm)
	if strings.ToLower(confirm) != "y" {
		fmt.Println("Operation cancelled.")
		return
	}

	// --- Delete from History DB (transactional) ---
	if err := execTx(historyDB, func(tx *sql.Tx) error {
		// Delete keyword_search_terms referencing matching URLs.
		if _, err := tx.Exec(
			`DELETE FROM keyword_search_terms WHERE url_id IN (SELECT id FROM urls WHERE url LIKE ? ESCAPE '\')`, pattern); err != nil {
			return fmt.Errorf("delete keyword_search_terms: %w", err)
		}
		// Delete segment_usage for segments referencing matching URLs.
		if _, err := tx.Exec(
			`DELETE FROM segment_usage WHERE segment_id IN (SELECT id FROM segments WHERE url_id IN (SELECT id FROM urls WHERE url LIKE ? ESCAPE '\'))`, pattern); err != nil {
			return fmt.Errorf("delete segment_usage: %w", err)
		}
		// Delete segments referencing matching URLs.
		if _, err := tx.Exec(
			`DELETE FROM segments WHERE url_id IN (SELECT id FROM urls WHERE url LIKE ? ESCAPE '\')`, pattern); err != nil {
			return fmt.Errorf("delete segments: %w", err)
		}
		// Delete visit records.
		if _, err := tx.Exec(
			`DELETE FROM visits WHERE url IN (SELECT id FROM urls WHERE url LIKE ? ESCAPE '\')`, pattern); err != nil {
			return fmt.Errorf("delete visits: %w", err)
		}
		// Delete URL entries.
		if _, err := tx.Exec(
			`DELETE FROM urls WHERE url LIKE ? ESCAPE '\'`, pattern); err != nil {
			return fmt.Errorf("delete urls: %w", err)
		}
		return nil
	}); err != nil {
		log.Fatalf("History cleanup failed: %v", err)
	}

	// --- Delete from Web Data DB (transactional) ---
	if err := execTx(webDB, func(tx *sql.Tx) error {
		if _, err := tx.Exec(
			`DELETE FROM keywords WHERE short_name LIKE ? ESCAPE '\' OR url LIKE ? ESCAPE '\'`, pattern, pattern); err != nil {
			return fmt.Errorf("delete keywords: %w", err)
		}
		if _, err := tx.Exec(
			`DELETE FROM autofill WHERE value LIKE ? ESCAPE '\'`, pattern); err != nil {
			return fmt.Errorf("delete autofill: %w", err)
		}
		return nil
	}); err != nil {
		log.Fatalf("Web Data cleanup failed: %v", err)
	}

	// --- VACUUM to reclaim disk space ---
	if _, err := historyDB.Exec("VACUUM"); err != nil {
		log.Printf("Warning: VACUUM on History DB failed: %v", err)
	}
	if _, err := webDB.Exec("VACUUM"); err != nil {
		log.Printf("Warning: VACUUM on Web Data DB failed: %v", err)
	}

	fmt.Println("Cleanup complete. Please restart Chrome.")
}
