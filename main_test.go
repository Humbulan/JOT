package main

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// setDBPath points the package-level dbFlag at a fresh SQLite database inside
// t.TempDir() and restores the previous value when the test completes.
func setDBPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "jot.db")
	old := dbFlag
	dbFlag = path
	t.Cleanup(func() { dbFlag = old })
	return path
}

// runJotCommand runs a jot command against dbPath, capturing stdout and
// stderr. It returns the captured output and the command's error.
func runJotCommand(t *testing.T, dbPath, command string, args ...string) (string, string, error) {
	t.Helper()
	cmdArgs := append([]string{command}, args...)

	stdOutR, stdOutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	stdErrR, stdErrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stderr pipe: %v", err)
	}

	oldOut, oldErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = stdOutW, stdErrW

	var cmdErr error
	switch command {
	case "add":
		cmdErr = cmdAdd(cmdArgs[1:])
	case "list":
		cmdErr = cmdList(cmdArgs[1:])
	case "search":
		cmdErr = cmdSearch(cmdArgs[1:])
	case "delete":
		cmdErr = cmdDelete(cmdArgs[1:])
	case "edit":
		cmdErr = cmdEdit(cmdArgs[1:])
	default:
		cmdErr = fmt.Errorf("unknown command: %s", command)
	}

	os.Stdout, os.Stderr = oldOut, oldErr
	stdOutW.Close()
	stdErrW.Close()

	outBytes, _ := io.ReadAll(stdOutR)
	errBytes, _ := io.ReadAll(stdErrR)
	stdOutR.Close()
	stdErrR.Close()

	return string(outBytes), string(errBytes), cmdErr
}

func noteArgs(title, content string) []string {
	return []string{"-title", title, "-content", content}
}

func addNote(t *testing.T, dbPath string, title, content string) {
	t.Helper()
	if _, _, err := runJotCommand(t, dbPath, "add", noteArgs(title, content)...); err != nil {
		t.Fatalf("setup add failed: %v", err)
	}
}

// noteUpdatedAt queries the database directly for a note's updated_at timestamp.
func noteUpdatedAt(t *testing.T, dbPath string, id int) string {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	var updatedAt string
	if err := db.QueryRow(`SELECT updated_at FROM notes WHERE id = ?`, id).Scan(&updatedAt); err != nil {
		t.Fatalf("query updated_at: %v", err)
	}
	return updatedAt
}

func TestAdd(t *testing.T) {
	tests := []struct {
		name    string
		preAdd  [][]string // add these notes before running the command under test
		args    []string
		wantErr string
		wantOut string
	}{
		{
			name:    "first note gets id 1",
			args:    noteArgs("Meeting", "Discuss Q3 plans"),
			wantOut: "Added note #1",
		},
		{
			name:    "second note gets id 2",
			preAdd:  [][]string{noteArgs("First", "First note")},
			args:    noteArgs("Second", "Second note"),
			wantOut: "Added note #2",
		},
		{
			name:    "missing title",
			args:    []string{"-content", "no title"},
			wantErr: "add requires both -title and -content",
		},
		{
			name:    "missing content",
			args:    []string{"-title", "No Content"},
			wantErr: "add requires both -title and -content",
		},
		{
			name:    "no args",
			args:    []string{},
			wantErr: "add requires both -title and -content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dbPath := setDBPath(t)

			for _, args := range tt.preAdd {
				if _, _, err := runJotCommand(t, dbPath, "add", args...); err != nil {
					t.Fatalf("setup add failed: %v", err)
				}
			}

			out, _, err := runJotCommand(t, dbPath, "add", tt.args...)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got none", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error to contain %q, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("add failed: %v", err)
			}
			if !strings.Contains(out, tt.wantOut) {
				t.Fatalf("expected output to contain %q, got %q", tt.wantOut, out)
			}
		})
	}
}

func TestList(t *testing.T) {
	tests := []struct {
		name            string
		preAdd          [][]string
		wantErr         string
		wantContains    []string
		wantNotContains []string
	}{
		{
			name:         "empty database reports no notes",
			wantContains: []string{"(no notes)"},
		},
		{
			name:         "lists all notes in descending id order",
			preAdd:       [][]string{noteArgs("First", "First content"), noteArgs("Second", "Second content")},
			wantContains: []string{"Second", "First"},
		},
		{
			name:            "does not list notes from other databases",
			preAdd:          [][]string{noteArgs("Local", "local content")},
			wantContains:    []string{"Local"},
			wantNotContains: []string{"(no notes)"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dbPath := setDBPath(t)

			for _, args := range tt.preAdd {
				if _, _, err := runJotCommand(t, dbPath, "add", args...); err != nil {
					t.Fatalf("setup add failed: %v", err)
				}
			}

			out, _, err := runJotCommand(t, dbPath, "list")
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error %q, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("list failed: %v", err)
			}
			for _, s := range tt.wantContains {
				if !strings.Contains(out, s) {
					t.Errorf("expected output to contain %q, got:\n%s", s, out)
				}
			}
			for _, s := range tt.wantNotContains {
				if strings.Contains(out, s) {
					t.Errorf("expected output to not contain %q, got:\n%s", s, out)
				}
			}
		})
	}
}

func TestSearch(t *testing.T) {
	tests := []struct {
		name            string
		preAdd          [][]string
		query           string
		wantErr         string
		wantContains    []string
		wantNotContains []string
	}{
		{
			name:            "matches title",
			preAdd:          [][]string{noteArgs("Meeting", "Discuss Q3 plans"), noteArgs("Lunch", "Talk about food")},
			query:           "Meeting",
			wantContains:    []string{"Meeting", "Discuss Q3 plans"},
			wantNotContains: []string{"Lunch"},
		},
		{
			name:         "matches content",
			preAdd:       [][]string{noteArgs("Meeting", "Discuss Q3 plans")},
			query:        "Q3",
			wantContains: []string{"Meeting"},
		},
		{
			name:         "matches case insensitively",
			preAdd:       [][]string{noteArgs("Meeting", "Discuss Q3 plans")},
			query:        "meeting",
			wantContains: []string{"Meeting"},
		},
		{
			name:         "no matches reports no matches",
			preAdd:       [][]string{noteArgs("Meeting", "Discuss Q3 plans")},
			query:        "zzznonexistent",
			wantContains: []string{"(no matches)"},
		},
		{
			name:    "empty query errors",
			query:   "",
			wantErr: "search requires -query",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dbPath := setDBPath(t)

			for _, args := range tt.preAdd {
				if _, _, err := runJotCommand(t, dbPath, "add", args...); err != nil {
					t.Fatalf("setup add failed: %v", err)
				}
			}

			out, _, err := runJotCommand(t, dbPath, "search", "-query", tt.query)

			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error %q, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("search failed: %v", err)
			}
			for _, s := range tt.wantContains {
				if !strings.Contains(out, s) {
					t.Errorf("expected output to contain %q, got:\n%s", s, out)
				}
			}
			for _, s := range tt.wantNotContains {
				if strings.Contains(out, s) {
					t.Errorf("expected output to not contain %q, got:\n%s", s, out)
				}
			}
		})
	}
}

func TestDelete(t *testing.T) {
	tests := []struct {
		name            string
		preAdd          [][]string
		args            []string
		wantErr         string
		wantOut         string
		wantContains    []string
		wantNotContains []string
	}{
		{
			name:            "deletes existing note",
			preAdd:          [][]string{noteArgs("Keep Me", "stay"), noteArgs("Remove Me", "gone")},
			args:            []string{"-id", "2"},
			wantOut:         "Deleted note #2",
			wantContains:    []string{"Keep Me"},
			wantNotContains: []string{"Remove Me"},
		},
		{
			name:            "deletes the only note",
			preAdd:          [][]string{noteArgs("Solo", "only one")},
			args:            []string{"-id", "1"},
			wantOut:         "Deleted note #1",
			wantNotContains: []string{"Solo"},
		},
		{
			name:         "deleting nonexistent id reports no note",
			preAdd:       [][]string{noteArgs("Solo", "only one")},
			args:         []string{"-id", "99"},
			wantOut:      "No note with id 99",
			wantContains: []string{"Solo"},
		},
		{
			name:    "missing id",
			args:    []string{},
			wantErr: "delete requires -id",
		},
		{
			name:    "zero id",
			args:    []string{"-id", "0"},
			wantErr: "delete requires -id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dbPath := setDBPath(t)

			for _, args := range tt.preAdd {
				if _, _, err := runJotCommand(t, dbPath, "add", args...); err != nil {
					t.Fatalf("setup add failed: %v", err)
				}
			}

			out, _, err := runJotCommand(t, dbPath, "delete", tt.args...)

			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error %q, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("delete failed: %v", err)
			}
			if tt.wantOut != "" && !strings.Contains(out, tt.wantOut) {
				t.Errorf("expected output to contain %q, got:\n%s", tt.wantOut, out)
			}

			// List to verify the delete persisted.
			listOut, _, err := runJotCommand(t, dbPath, "list")
			if err != nil {
				t.Fatalf("list after delete failed: %v", err)
			}
			for _, s := range tt.wantContains {
				if !strings.Contains(listOut, s) {
					t.Errorf("expected list output to contain %q, got:\n%s", s, listOut)
				}
			}
			for _, s := range tt.wantNotContains {
				if strings.Contains(listOut, s) {
					t.Errorf("expected list output to not contain %q, got:\n%s", s, listOut)
				}
			}
		})
	}
}

func TestEdit(t *testing.T) {
	tests := []struct {
		name            string
		preAdd          [][]string
		args            []string
		wantErr         string
		wantOut         string
		wantContains    []string
		wantNotContains []string
		wantUpdatedAt   bool // assert updated_at changed after the edit
	}{
		{
			name:            "updates title and content",
			preAdd:          [][]string{noteArgs("Old Title", "Old content")},
			args:            []string{"-id", "1", "-title", "New Title", "-content", "New content"},
			wantOut:         "Updated note #1",
			wantContains:    []string{"New Title", "New content"},
			wantNotContains: []string{"Old Title", "Old content"},
			wantUpdatedAt:   true,
		},
		{
			name:            "updates only title",
			preAdd:          [][]string{noteArgs("Keep Content", "original content")},
			args:            []string{"-id", "1", "-title", "New Title Only"},
			wantOut:         "Updated note #1",
			wantContains:    []string{"New Title Only", "original content"},
			wantNotContains: []string{"Keep Content"},
			wantUpdatedAt:   true,
		},
		{
			name:            "updates only content",
			preAdd:          [][]string{noteArgs("Keep Title", "original content")},
			args:            []string{"-id", "1", "-content", "new content only"},
			wantOut:         "Updated note #1",
			wantContains:    []string{"Keep Title", "new content only"},
			wantNotContains: []string{"original content"},
			wantUpdatedAt:   true,
		},
		{
			name:          "bumps updated_at to a later RFC3339 timestamp",
			preAdd:        [][]string{noteArgs("Timed", "original")},
			args:          []string{"-id", "1", "-content", "updated later"},
			wantOut:       "Updated note #1",
			wantUpdatedAt: true,
		},
		{
			name:         "editing nonexistent id reports no note",
			preAdd:       [][]string{noteArgs("Solo", "only one")},
			args:         []string{"-id", "99", "-title", "Nope"},
			wantOut:      "No note with id 99",
			wantContains: []string{"Solo"},
		},
		{
			name:    "missing id",
			args:    []string{"-title", "T"},
			wantErr: "edit requires -id",
		},
		{
			name:    "no fields to update",
			args:    []string{"-id", "1"},
			wantErr: "edit requires -title and/or -content",
		},
		{
			name:    "no args at all",
			args:    []string{},
			wantErr: "edit requires -id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dbPath := setDBPath(t)

			for _, args := range tt.preAdd {
				if _, _, err := runJotCommand(t, dbPath, "add", args...); err != nil {
					t.Fatalf("setup add failed: %v", err)
				}
			}

			var before string
			if tt.wantUpdatedAt {
				before = noteUpdatedAt(t, dbPath, 1)
				// updated_at is stored at second precision, so ensure the
				// edit lands in a later second.
				time.Sleep(time.Second + 500*time.Millisecond)
			}

			out, _, err := runJotCommand(t, dbPath, "edit", tt.args...)

			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error %q, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("edit failed: %v", err)
			}
			if tt.wantOut != "" && !strings.Contains(out, tt.wantOut) {
				t.Errorf("expected output to contain %q, got:\n%s", tt.wantOut, out)
			}

			// List to verify the edit persisted.
			listOut, _, err := runJotCommand(t, dbPath, "list")
			if err != nil {
				t.Fatalf("list after edit failed: %v", err)
			}
			for _, s := range tt.wantContains {
				if !strings.Contains(listOut, s) {
					t.Errorf("expected list output to contain %q, got:\n%s", s, listOut)
				}
			}
			for _, s := range tt.wantNotContains {
				if strings.Contains(listOut, s) {
					t.Errorf("expected list output to not contain %q, got:\n%s", s, listOut)
				}
			}

			if tt.wantUpdatedAt {
				after := noteUpdatedAt(t, dbPath, 1)
				if after == before {
					t.Errorf("expected updated_at to change after edit, got %q both times", after)
				}
				if _, err := time.Parse(time.RFC3339, after); err != nil {
					t.Errorf("updated_at %q is not valid RFC3339: %v", after, err)
				}
			}
		})
	}
}
