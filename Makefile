GO ?= go
BIN ?= codex-manager
BINDIR ?= bin
BUILD_OUTPUT ?= $(BINDIR)/$(BIN)
INSTALL_DIR ?= /usr/local/bin
RUN_ARGS ?= -ts

.PHONY: build run install clean test

build:
	@mkdir -p $(BINDIR)
	$(GO) build -o $(BUILD_OUTPUT) ./cmd/codex-manager

run: build
	$(BUILD_OUTPUT) $(RUN_ARGS)

install: build
	@mkdir -p $(INSTALL_DIR)
	install -m 0755 $(BUILD_OUTPUT) $(INSTALL_DIR)/$(BIN)

clean:
	rm -rf $(BINDIR)

test:
	$(GO) test ./...
