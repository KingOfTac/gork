# -------------------------------------------------
# Project configuration
# -------------------------------------------------

MODULE_PATH := github.com/kingoftac/gork/internal/version
CMD_DIR := cmd
BIN_DIR := bin

# Binary names mapped to cmd folders
BINS := gorkctl gorkd gorktui

# Map binary names to their cmd directories
gorkctl_DIR := cli
gorkd_DIR := daemon
gorktui_DIR := tui

# -------------------------------------------------
# Build metadata
# -------------------------------------------------

GOOS ?= $(shell go env GOOS)
EXT :=
ifeq ($(GOOS),windows)
EXT := .exe
endif

VERSION ?= dev
COMMIT ?= none
DATE ?= unknown

LDFLAGS := -X $(MODULE_PATH).Version=$(VERSION) \
	-X $(MODULE_PATH).Commit=$(COMMIT) \
	-X $(MODULE_PATH).Date=$(DATE)

# -------------------------------------------------
# Go environment
# -------------------------------------------------

GO ?= go
GO_FLAGS ?= 

# -------------------------------------------------
# Targets
# -------------------------------------------------

.PHONY: build run docker-build clean test

all: build

build: $(BINS:%=$(BIN_DIR)/%$(EXT))

$(BIN_DIR):
	mkdir $(BIN_DIR)

$(BIN_DIR)/%$(EXT): | $(BIN_DIR)
	$(GO) build $(GO_FLAGS) \
		-ldflags "$(LDFLAGS)" \
		-o $@ \
		./$(CMD_DIR)/$($*_DIR)

run:
	./$(BIN_DIR)/gorkctl$(EXT) list

docker-build:
	docker build -t go-workflow .

clean:
	rmdir $(BIN_DIR) /s /q

test:
	go test ./...
