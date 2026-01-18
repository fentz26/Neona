#!/bin/bash
set -e

# Neona Installer
# Usage: curl -fsSL https://neona.app/install.sh | bash

REPO="Neona-AI/Neona"
BINARY_NAME="neona"
INSTALL_DIR="/usr/local/bin"
GO_BIN_DIR="$HOME/go/bin"

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[0;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${BLUE}🚀 Installing Neona Control Plane...${NC}"

# Detect OS and Architecture
detect_platform() {
    OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
    ARCH="$(uname -m)"
    
    case "$OS" in
        linux)  OS="linux" ;;
        darwin) OS="darwin" ;;
        mingw*|msys*|cygwin*) OS="windows" ;;
        *)
            echo -e "${RED}✗ Unsupported operating system: $OS${NC}"
            exit 1
            ;;
    esac
    
    case "$ARCH" in
        x86_64|amd64)  ARCH="amd64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        armv7l)        ARCH="arm" ;;
        i386|i686)     ARCH="386" ;;
        *)
            echo -e "${RED}✗ Unsupported architecture: $ARCH${NC}"
            exit 1
            ;;
    esac
    
    # Set binary extension for Windows
    BINARY_EXT=""
    if [ "$OS" = "windows" ]; then
        BINARY_EXT=".exe"
    fi
    
    echo -e "${BLUE}ℹ Detected platform: ${OS}/${ARCH}${NC}"
}

# Get latest release version from GitHub API
get_latest_version() {
    LATEST_VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" 2>/dev/null | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
    
    if [ -z "$LATEST_VERSION" ]; then
        echo -e "${YELLOW}⚠ Could not fetch latest version. Trying to find any release...${NC}"
        # Fallback: get latest pre-release
        LATEST_VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases" 2>/dev/null | grep '"tag_name"' | head -1 | sed -E 's/.*"([^"]+)".*/\1/')
    fi
    
    if [ -z "$LATEST_VERSION" ]; then
        echo -e "${RED}✗ No releases found.${NC}"
        return 1
    fi
    
    echo -e "${BLUE}ℹ Latest version: ${LATEST_VERSION}${NC}"
}

# Download and install pre-built binary
install_from_release() {
    detect_platform
    
    if ! get_latest_version; then
        echo -e "${RED}✗ No releases available. Please install Go and try again.${NC}"
        echo -e "${YELLOW}Install Go from: https://go.dev/dl/${NC}"
        exit 1
    fi
    
    # Asset naming: neona-{os}-{arch} (matches CI output)
    ASSET_NAME="neona-${OS}-${ARCH}${BINARY_EXT}"
    DOWNLOAD_URL="https://github.com/$REPO/releases/download/${LATEST_VERSION}/${ASSET_NAME}"
    
    echo -e "${BLUE}⬇ Downloading: ${ASSET_NAME}${NC}"
    echo -e "${BLUE}  From: ${DOWNLOAD_URL}${NC}"
    
    # Create temporary directory
    TMP_DIR=$(mktemp -d)
    trap "rm -rf $TMP_DIR" EXIT
    
    # Download binary
    if ! curl -fsSL "$DOWNLOAD_URL" -o "$TMP_DIR/$BINARY_NAME$BINARY_EXT"; then
        echo -e "${RED}✗ Failed to download binary.${NC}"
        echo -e "${YELLOW}Asset '${ASSET_NAME}' may not exist for this platform.${NC}"
        echo -e "${YELLOW}Check releases at: https://github.com/$REPO/releases${NC}"
        echo -e "${YELLOW}Or install Go from https://go.dev/dl/ and try again.${NC}"
        exit 1
    fi
    
    # Make executable
    chmod +x "$TMP_DIR/$BINARY_NAME$BINARY_EXT"
    
    # Install binary
    if [ -w "$INSTALL_DIR" ]; then
        mv "$TMP_DIR/$BINARY_NAME$BINARY_EXT" "$INSTALL_DIR/$BINARY_NAME"
    elif command -v sudo &> /dev/null; then
        echo -e "${YELLOW}⚠ Requesting sudo to install to $INSTALL_DIR${NC}"
        sudo mv "$TMP_DIR/$BINARY_NAME$BINARY_EXT" "$INSTALL_DIR/$BINARY_NAME"
        sudo chmod +x "$INSTALL_DIR/$BINARY_NAME"
    else
        # Fallback to user's local bin
        LOCAL_BIN="$HOME/.local/bin"
        mkdir -p "$LOCAL_BIN"
        mv "$TMP_DIR/$BINARY_NAME$BINARY_EXT" "$LOCAL_BIN/$BINARY_NAME"
        chmod +x "$LOCAL_BIN/$BINARY_NAME"
        
        if [[ ":$PATH:" != *":$LOCAL_BIN:"* ]]; then
            echo 'export PATH=$PATH:$HOME/.local/bin' >> ~/.bashrc
            echo 'export PATH=$PATH:$HOME/.local/bin' >> ~/.zshrc 2>/dev/null || true
            echo -e "${YELLOW}ℹ Added $LOCAL_BIN to PATH. Restart your terminal or run: source ~/.bashrc${NC}"
        fi
        
        INSTALL_DIR="$LOCAL_BIN"
    fi
    
    echo -e "${GREEN}✅ Neona ${LATEST_VERSION} installed to $INSTALL_DIR/$BINARY_NAME${NC}"
}

# Install via Go (preferred if Go is available)
install_from_go() {
    echo -e "${GREEN}✓ Go detected. Installing via 'go install'...${NC}"
    
    # Install directly from main repo
    go install github.com/$REPO/cmd/neona@latest
    
    # Try to make it globally available immediately via symlink
    if [ -d "/usr/local/bin" ]; then
        if sudo ln -sf "$GO_BIN_DIR/neona" /usr/local/bin/neona 2>/dev/null; then
             echo -e "${GREEN}✓ Linked to /usr/local/bin/neona (available instantly)${NC}"
        else
             # Fallback to PATH modification if sudo fails
             if [[ ":$PATH:" != *":$GO_BIN_DIR:"* ]]; then
                 echo -e "${BLUE}ℹ sudo failed/unavailable. Adding $GO_BIN_DIR to PATH in ~/.bashrc${NC}"
                 echo 'export PATH=$PATH:$HOME/go/bin' >> ~/.bashrc
                 echo 'export PATH=$PATH:$HOME/go/bin' >> ~/.zshrc 2>/dev/null || true
             fi
        fi
    fi

    echo -e "${GREEN}✅ Neona installed successfully!${NC}"
}

# Main installation logic
main() {
    # Check for Go installation first (preferred method)
    if command -v go &> /dev/null; then
        install_from_go
    else
        echo -e "${YELLOW}ℹ Go not found. Downloading pre-built binary...${NC}"
        install_from_release
    fi
    
    # Verify installation
    echo ""
    if command -v neona &> /dev/null; then
        INSTALLED_VERSION=$(neona version 2>/dev/null || echo "unknown")
        echo -e "${GREEN}🎉 Installation complete!${NC}"
        echo -e "   Version: ${BLUE}${INSTALLED_VERSION}${NC}"
        echo -e "   Run ${GREEN}neona${NC} to start."
    elif [ -f "/usr/local/bin/neona" ] || [ -f "$HOME/.local/bin/neona" ] || [ -f "$GO_BIN_DIR/neona" ]; then
        echo -e "${GREEN}🎉 Installation complete!${NC}"
        echo -e "${YELLOW}⚠ Restart your terminal or run: source ~/.bashrc${NC}"
    else
        echo -e "${RED}✗ Installation may have failed. Please check for errors above.${NC}"
        exit 1
    fi
}

main
