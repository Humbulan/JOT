package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	_ "modernc.org/sqlite"
)

// Note represents a stored note.
type Note struct {
	ID        int
	Title     string
	Content   string
	CreatedAt string
	UpdatedAt string
}

var dbFlag string

func init() {
	const usage = `jot - a SQLite-backed notes CLI

Usage:
    jot <command> [options]

Commands:
    add      -title TITLE -content CONTENT  Add a new note
    list                               List all notes
    search   -query QUERY              Search notes by title or content
    delete   -id ID                    Delete a note by ID

Global options:
    --db PATH          Path to SQLite database (default: jot.db or \$JOT_DB)
    -h, --help         Show this help

Examples:
    jot add -title "Meeting" -content "Discuss Q3 plans"
    jot list
    jot search -query meeting
    jot delete -id 3
`
	_ = usage
}

func usage() {
	fmt.Println(`jot - a SQLite-backed notes CLI

Usage:
    jot <command> [options]

Commands:
    add      -title TITLE -content CONTENT  Add a new note
    list                               List all notes
    search   -query QUERY              Search notes by title or content
    delete   -id ID                    Delete a note by ID

Global options:
    --db PATH          Path to SQLite database (default: jot.db or \$JOT_DB)
    -h, --help         Show this help

Examples:
    jot add -title "Meeting" -content "Discuss Q3 plans"
    jot list
    jot search -query meeting
    jot delete -id 3`)
}

func resolveDBPath(args []string) string {
	// Honor --db PATH (must appear before the subcommand), then $JOT_DB, then default.
	path := ""
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "--db" {
			path = args[i+1]
			break
		}
	}
	if path == "" {
		path = os.Getenv("JOT_DB")
	}
	if path == "" {
		path = "jot.db"
	}
	return path
}

func stripGlobalFlags(args []string) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		if args[i] == "--db" && i+1 < len(args) {
			i++ // skip the value
			continue
		}
		out = append(out, args[i])
	}
	return out
}

func openDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return db, nil
}

func initDB(db *sql.DB) error {
	schema := `
CREATE TABLE IF NOT EXISTS notes (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    title      TEXT    NOT NULL,
    content    TEXT    NOT NULL,
    created_at TEXT    NOT NULL,
    updated_at TEXT    NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_notes_title   ON notes(title);
CREATE INDEX IF NOT EXISTS idx_notes_content ON notes(content);
`
	_, err := db.Exec(schema)
	return err
}

func cmdAdd(args []string) error {
	fs := flag.NewFlagSet("add", flag.ExitOnError)
	title := fs.String("title", "", "Note title")
	content := fs.String("content", "", "Note content")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *title == "" || *content == "" {
		return fmt.Errorf("add requires both -title and -content")
	}

	db, err := openDB(dbFlag)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := initDB(db); err != nil {
		return err
	}

	now := time.Now().Format(time.RFC3339)
	res, err := db.Exec(
		`INSERT INTO notes (title, content, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		*title, *content, now, now,
	)
	if err != nil {
		return fmt.Errorf("insert note: %w", err)
	}
	id, _ := res.LastInsertId()
	fmt.Printf("Added note #%d\n", id)
	return nil
}

func cmdList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	db, err := openDB(dbFlag)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := initDB(db); err != nil {
		return err
	}

	rows, err := db.Query(
		`SELECT id, title, content, created_at, updated_at FROM notes ORDER BY id DESC`,
	)
	if err != nil {
		return fmt.Errorf("query notes: %w", err)
	}
	defer rows.Close()

	fmt.Println("ID  Title              Content               Created")
	fmt.Println("--  -----              -------               -------")
	count := 0
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.Title, &n.Content, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return err
		}
		fmt.Printf("%-3d %-18s %-19s %s\n", n.ID, truncate(n.Title, 16), truncate(n.Content, 17), n.CreatedAt[:10])
		count++
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if count == 0 {
		fmt.Println("  (no notes)")
	}
	return nil
}

func cmdSearch(args []string) error {
	fs := flag.NewFlagSet("search", flag.ExitOnError)
	query := fs.String("query", "", "Search query")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *query == "" {
		return fmt.Errorf("search requires -query")
	}

	db, err := openDB(dbFlag)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := initDB(db); err != nil {
		return err
	}

	like := "%" + *query + "%"
	rows, err := db.Query(
		`SELECT id, title, content, created_at, updated_at FROM notes
		 WHERE title LIKE ? OR content LIKE ?
		 ORDER BY id DESC`,
		like, like,
	)
	if err != nil {
		return fmt.Errorf("search notes: %w", err)
	}
	defer rows.Close()

	fmt.Printf("Search results for %q:\n", *query)
	fmt.Println("ID  Title              Content               Created")
	fmt.Println("--  -----              -------               -------")
	count := 0
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.Title, &n.Content, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return err
		}
		fmt.Printf("%-3d %-18s %-19s %s\n", n.ID, truncate(n.Title, 16), truncate(n.Content, 17), n.CreatedAt[:10])
		count++
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if count == 0 {
		fmt.Println("  (no matches)")
	}
	return nil
}

func cmdDelete(args []string) error {
	fs := flag.NewFlagSet("delete", flag.ExitOnError)
	id := fs.Int("id", 0, "Note ID to delete")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *id == 0 {
		return fmt.Errorf("delete requires -id")
	}

	db, err := openDB(dbFlag)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := initDB(db); err != nil {
		return err
	}

	res, err := db.Exec(`DELETE FROM notes WHERE id = ?`, *id)
	if err != nil {
		return fmt.Errorf("delete note: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		fmt.Printf("No note with id %d\n", *id)
		return nil
	}
	fmt.Printf("Deleted note #%d\n", *id)
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "\u2026"
}

func main() {
	args := os.Args[1:]

	// Handle global help before any subcommand parsing.
	for _, a := range args {
		if a == "-h" || a == "--help" || a == "help" {
			usage()
			return
		}
	}

	if len(args) == 0 {
		usage()
		os.Exit(2)
	}

	dbFlag = resolveDBPath(args)
	args = stripGlobalFlags(args)

	switch args[0] {
	case "add":
		if err := cmdAdd(args[1:]); err != nil {
			log.Fatal(err)
		}
	case "list":
		if err := cmdList(args[1:]); err != nil {
			log.Fatal(err)
		}
	case "search":
		if err := cmdSearch(args[1:]); err != nil {
			log.Fatal(err)
		}
	case "delete":
		if err := cmdDelete(args[1:]); err != nil {
			log.Fatal(err)
		}
	default:
		fmt.Printf("Unknown command: %s\n\n", args[0])
		usage()
		os.Exit(2)
	}
}
