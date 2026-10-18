# Makefile for the jot CLI
#
# Builds a pure-Go SQLite notes tool (no CGO required).

BIN   := jot
SRC   := .
GO    ?= go
PREFIX ?= /usr/local
GOBIN ?= $(HOME)/go/bin

.PHONY: all build clean run test install help

all: build

build:
	$(GO) build -o $(BIN) $(SRC)

run: build
	./$(BIN)

test:
	$(GO) test ./...

install: build
	install -m 0755 $(BIN) $(GOBIN)/$(BIN)

clean:
	rm -f $(BIN) jot.db

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build    Build the jot binary"
	@echo "  run      Build and run jot"
	@echo "  test     Run Go tests"
	@echo "  install  Install jot to \$$(GOBIN)"
	@echo "  clean    Remove the binary and default database"
	@echo "  help     Show this help"
