# jot

`jot` is a small command-line notes manager that stores notes in a SQLite
database. It is written in pure Go and uses [modernc.org/sqlite](https://github.com/modernc/sqlite),
so there is **no CGO** and no C compiler is required.

## Commands

| Command | Description |
|---------|-------------|
| `jot add -title T -content C` | Add a new note |
| `jot list` | List all notes (newest first) |
| `jot search -query Q` | Search notes by title or content |
| `jot delete -id N` | Delete a note by ID |
| `jot --help` | Show help |

## Database location

By default notes are stored in `jot.db` in the current directory. Override
with the `JOT_DB` environment variable or the `--db PATH` flag:

```
JOT_DB=~/.local/share/jot/notes.db jot add -title "Hello" -content "World"
jot --db /tmp/jot.db list
```

## Build

```
make
```

or

```
go build -o jot .
```

## Install

```
make install   # copies the binary to $GOPATH/bin (or $HOME/go/bin)
```

## License

MIT
