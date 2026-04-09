APP := yanker
BIN_DIR := bin
BUILD_BIN := $(BIN_DIR)/$(APP)
PREFIX ?= $(HOME)/.local
BINDIR ?= $(PREFIX)/bin

.PHONY: help build run fmt test install install-user uninstall clean

help:
	@printf "Targets:\n"
	@printf "  make build         Build $(BUILD_BIN)\n"
	@printf "  make run           Run the app with go run\n"
	@printf "  make fmt           Format Go code\n"
	@printf "  make test          Run Go tests\n"
	@printf "  make install       Install to $(BINDIR)/$(APP)\n"
	@printf "  make install-user  Install to $(HOME)/.local/bin/$(APP)\n"
	@printf "  make uninstall     Remove $(BINDIR)/$(APP)\n"
	@printf "  make clean         Remove build output\n"

build:
	@mkdir -p "$(BIN_DIR)"
	go build -o "$(BUILD_BIN)" .

run:
	go run .

fmt:
	go fmt ./...

test:
	go test ./...

install: build
	@mkdir -p "$(BINDIR)"
	install -m 0755 "$(BUILD_BIN)" "$(BINDIR)/$(APP)"
	@printf "Installed %s to %s\n" "$(APP)" "$(BINDIR)/$(APP)"

install-user: build
	@mkdir -p "$(HOME)/.local/bin"
	install -m 0755 "$(BUILD_BIN)" "$(HOME)/.local/bin/$(APP)"
	@printf "Installed %s to %s\n" "$(APP)" "$(HOME)/.local/bin/$(APP)"

uninstall:
	rm -f "$(BINDIR)/$(APP)"
	@printf "Removed %s\n" "$(BINDIR)/$(APP)"

clean:
	rm -rf "$(BIN_DIR)"
