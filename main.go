package main

import (
"database/sql"
"encoding/json"
"flag"
"fmt"
"log"
"os"
"path/filepath"
"text/tabwriter"
"time"

_ "modernc.org/sqlite"
)

func getDBPath() string {
if dbPath := os.Getenv("JOT_DB"); dbPath != "" {
return dbPath
}
home, err := os.UserHomeDir()
if err == nil {
dir := filepath.Join(home, ".jot")
os.MkdirAll(dir, 0755)
return filepath.Join(dir, "jot.db")
}
return "jot.db"
}

func main() {
if len(os.Args) < 2 {
printHelp()
os.Exit(1)
}

db, err := initDB()
if err != nil {
log.Fatalf("Failed to initialize database: %v", err)
}
defer db.Close()

cmd := os.Args[1]
args := os.Args[2:]

switch cmd {
case "add":
handleAdd(db, args)
case "list":
handleList(db, args)
case "search":
handleSearch(db, args)
case "delete":
handleDelete(db, args)
case "edit":
handleEdit(db, args)
case "export":
handleExport(db, args)
case "--help", "-h", "help":
printHelp()
default:
fmt.Printf("Unknown command: %s\n\n", cmd)
printHelp()
os.Exit(1)
}
}

func initDB() (*sql.DB, error) {
db, err := sql.Open("sqlite", getDBPath())
if err != nil {
return nil, err
}
// Added updated_at to preserve the edit command logic
query := `
CREATE TABLE IF NOT EXISTS notes (
id INTEGER PRIMARY KEY AUTOINCREMENT,
title TEXT NOT NULL,
content TEXT NOT NULL,
tag TEXT DEFAULT 'general',
created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);`
_, err = db.Exec(query)
return db, err
}

func handleAdd(db *sql.DB, args []string) {
fs := flag.NewFlagSet("add", flag.ExitOnError)
title := fs.String("title", "", "Note title")
content := fs.String("content", "", "Note content")
tag := fs.String("tag", "general", "Note category/tag")
fs.Parse(args)

if *title == "" || *content == "" {
fmt.Println("Error: --title and --content are required.")
fmt.Println("Usage: jot add --title 'Title' --content 'Content' [--tag 'tag']")
os.Exit(1)
}

_, err := db.Exec("INSERT INTO notes (title, content, tag) VALUES (?, ?, ?)", *title, *content, *tag)
if err != nil {
log.Fatalf("Failed to add note: %v", err)
}
fmt.Println("Note added successfully.")
}

func handleList(db *sql.DB, args []string) {
fs := flag.NewFlagSet("list", flag.ExitOnError)
filterTag := fs.String("tag", "", "Filter by tag")
fs.Parse(args)

var rows *sql.Rows
var err error
if *filterTag != "" {
rows, err = db.Query("SELECT id, title, content, tag, created_at FROM notes WHERE tag = ? ORDER BY created_at DESC", *filterTag)
} else {
rows, err = db.Query("SELECT id, title, content, tag, created_at FROM notes ORDER BY created_at DESC")
}
if err != nil {
log.Fatalf("Failed to query notes: %v", err)
}
defer rows.Close()

w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
fmt.Fprintln(w, "ID\tTAG\tTITLE\tCREATED\tCONTENT")
fmt.Fprintln(w, "--\t---\t-----\t-------\t-------")

hasNotes := false
for rows.Next() {
hasNotes = true
var id int
var title, content, tag, createdAt string
if err := rows.Scan(&id, &title, &content, &tag, &createdAt); err != nil {
continue
}
if len(content) > 35 {
content = content[:32] + "..."
}
fmt.Fprintf(w, "%d\t[%s]\t%s\t%s\t%s\n", id, tag, title, createdAt, content)
}
w.Flush()
if !hasNotes {
fmt.Println("No notes found.")
}
}

func handleSearch(db *sql.DB, args []string) {
fs := flag.NewFlagSet("search", flag.ExitOnError)
query := fs.String("query", "", "Search term")
fs.Parse(args)

if *query == "" {
fmt.Println("Usage: jot search --query <term>")
os.Exit(1)
}

term := "%" + *query + "%"
rows, err := db.Query("SELECT id, title, content, tag, created_at FROM notes WHERE title LIKE ? OR content LIKE ? OR tag LIKE ? ORDER BY created_at DESC", term, term, term)
if err != nil {
log.Fatalf("Failed to search notes: %v", err)
}
defer rows.Close()

w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
fmt.Fprintln(w, "ID\tTAG\tTITLE\tCREATED\tCONTENT")
fmt.Fprintln(w, "--\t---\t-----\t-------\t-------")

hasNotes := false
for rows.Next() {
hasNotes = true
var id int
var title, content, tag, createdAt string
if err := rows.Scan(&id, &title, &content, &tag, &createdAt); err != nil {
continue
}
fmt.Fprintf(w, "%d\t[%s]\t%s\t%s\t%s\n", id, tag, title, createdAt, content)
}
w.Flush()
if !hasNotes {
fmt.Println("No matching notes found.")
}
}

func handleDelete(db *sql.DB, args []string) {
fs := flag.NewFlagSet("delete", flag.ExitOnError)
id := fs.String("id", "", "Note ID to delete")
fs.Parse(args)

if *id == "" {
fmt.Println("Usage: jot delete --id <id>")
os.Exit(1)
}

res, err := db.Exec("DELETE FROM notes WHERE id = ?", *id)
if err != nil {
log.Fatalf("Failed to delete note: %v", err)
}
rowsAffected, _ := res.RowsAffected()
if rowsAffected == 0 {
fmt.Printf("Note with ID %s not found.\n", *id)
} else {
fmt.Printf("Note %s deleted successfully.\n", *id)
}
}

func handleEdit(db *sql.DB, args []string) {
fs := flag.NewFlagSet("edit", flag.ExitOnError)
id := fs.Int("id", 0, "Note ID to edit")
title := fs.String("title", "", "New title")
content := fs.String("content", "", "New content")
fs.Parse(args)

if *id == 0 {
fmt.Println("Usage: jot edit -id <id> [-title <title>] [-content <content>]")
os.Exit(1)
}

query := "UPDATE notes SET updated_at = ?"
params := []interface{}{time.Now().Format(time.RFC3339)}

if *title != "" {
query += ", title = ?"
params = append(params, *title)
}
if *content != "" {
query += ", content = ?"
params = append(params, *content)
}

if len(params) == 1 {
fmt.Println("Error: must provide -title and/or -content")
os.Exit(1)
}

query += " WHERE id = ?"
params = append(params, *id)

res, err := db.Exec(query, params...)
if err != nil {
log.Fatalf("Failed to update note: %v", err)
}

rowsAffected, _ := res.RowsAffected()
if rowsAffected == 0 {
fmt.Printf("No note with id %d\n", *id)
} else {
fmt.Printf("Updated note #%d\n", *id)
}
}

func handleExport(db *sql.DB, args []string) {
rows, err := db.Query("SELECT id, title, content, tag, created_at FROM notes ORDER BY created_at DESC")
if err != nil {
log.Fatalf("Failed to export notes: %v", err)
}
defer rows.Close()

type Note struct {
ID        int    `json:"id"`
Title     string `json:"title"`
Content   string `json:"content"`
Tag       string `json:"tag"`
CreatedAt string `json:"created_at"`
}

var notes []Note
for rows.Next() {
var n Note
rows.Scan(&n.ID, &n.Title, &n.Content, &n.Tag, &n.CreatedAt)
notes = append(notes, n)
}

data, _ := json.MarshalIndent(notes, "", "  ")
fmt.Println(string(data))
}

func printHelp() {
fmt.Println("jot - A pure-Go SQLite Notes & Audit CLI")
fmt.Println("\nUsage:")
fmt.Println("  jot <command> [options]")
fmt.Println("\nCommands:")
fmt.Println("  add     --title TITLE --content CONTENT [--tag TAG]  Add note")
fmt.Println("  list    [--tag TAG]                                  List notes")
fmt.Println("  search  --query QUERY                                Search notes")
fmt.Println("  delete  --id ID                                      Delete note")
fmt.Println("  edit    --id ID [--title TITLE] [--content CONTENT]  Edit note")
fmt.Println("  export                                               Export notes as JSON")
}
