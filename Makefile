.PHONY: build test install

PREFIX ?= $(HOME)/.local
DATA_DIR ?= $(if $(XDG_DATA_HOME),$(XDG_DATA_HOME),$(HOME)/.local/share)
BIN_DIR ?= bin

build:
	go build -trimpath -o "$(BIN_DIR)/eyeloveu" ./cmd/eyeloveu
	CGO_ENABLED=0 go build -trimpath -o "$(BIN_DIR)/eyeloveu-daemon" ./cmd/eyeloveu-daemon

test:
	go test ./...

install: build
	install -Dm755 "$(BIN_DIR)/eyeloveu" "$(DESTDIR)$(PREFIX)/bin/eyeloveu"
	install -Dm755 "$(BIN_DIR)/eyeloveu-daemon" "$(DESTDIR)$(PREFIX)/bin/eyeloveu-daemon"
	install -Dm644 data/local.eyeloveu.EyeLoveU.desktop "$(DESTDIR)$(DATA_DIR)/applications/local.eyeloveu.EyeLoveU.desktop"
