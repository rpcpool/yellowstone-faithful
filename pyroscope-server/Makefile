# Makefile for installing and running the Grafana Pyroscope binary.

# --- Configuration ---
# You can change the version and architecture to fit your needs.
PYROSCOPE_VERSION ?= 1.14.0
OS              ?= $(shell uname -s | tr '[:upper:]' '[:lower:]')
ARCH            ?= $(shell uname -m | sed 's/x86_64/amd64/' | sed 's/aarch64/arm64/')
TARBALL         := pyroscope_$(PYROSCOPE_VERSION)_$(OS)_$(ARCH).tar.gz
DOWNLOAD_URL    := https://github.com/grafana/pyroscope/releases/download/v$(PYROSCOPE_VERSION)/$(TARBALL)
TMP_TARBALL     := /tmp/$(TARBALL)

# Define the installation directory.
# This should be in your system's PATH.
INSTALL_DIR ?= /usr/local/bin

# --- Targets ---

.PHONY: all install run clean help

all: install

# The 'help' target provides information about the available commands.
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  install    Downloads and installs the Pyroscope binary to $(INSTALL_DIR)."
	@echo "  run        Starts the Pyroscope server (assumes it is installed)."
	@echo "  clean      Removes downloaded Pyroscope files."
	@echo "  help       Shows this help message."

# The 'install' target downloads, extracts, and installs the pyroscope binary.
# It requires sudo privileges to move the binary to the install directory.
install:
	@echo "--- Starting Pyroscope Installation ---"
	@echo "Step 1: Downloading Pyroscope archive from GitHub..."
	@echo "   - URL: $(DOWNLOAD_URL)"
	@echo "   - Destination: $(TMP_TARBALL)"
	@if command -v aria2c >/dev/null 2>&1; then \
		aria2c -d /tmp -o $(TARBALL) $(DOWNLOAD_URL); \
	elif command -v wget >/dev/null 2>&1; then \
		wget -q -O $(TMP_TARBALL) $(DOWNLOAD_URL); \
	elif command -v curl >/dev/null 2>&1; then \
		curl -sL -o $(TMP_TARBALL) $(DOWNLOAD_URL); \
	else \
		echo "Error: 'aria2c', 'wget', or 'curl' is required to download files." >&2; \
		exit 1; \
	fi
	@echo "Step 2: Extracting the 'pyroscope' binary..."
	@echo "   - From: $(TMP_TARBALL)"
	@echo "   - To:   /tmp/"
	@tar -xzf $(TMP_TARBALL) -C /tmp pyroscope
	@echo "Step 3: Installing the binary to $(INSTALL_DIR)..."
	@echo "   - This requires administrator privileges (sudo)."
	@sudo mv /tmp/pyroscope $(INSTALL_DIR)/pyroscope
	@echo "--- Installation Complete ---"
	@echo "Pyroscope installed successfully!"
	@echo "You can now run 'make run' or 'pyroscope server'"

# The 'run' target starts the pyroscope server.
# It assumes that 'pyroscope' is in the system's PATH.
run:
	@echo "Starting Pyroscope server..."
	@echo "View the UI at http://localhost:4040"
	@pyroscope server

# The 'clean' target removes the downloaded tarball and the extracted binary.
clean:
	@echo "Cleaning up..."
	@rm -f $(TMP_TARBALL)
	@rm -f /tmp/pyroscope
	@echo "Cleanup complete."

