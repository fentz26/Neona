#!/bin/bash
set -e

# Neona Installer
# Usage: curl -fsSL https://cli.neona.app/install.sh | bash

REPO="Neona-AI/Neona"
BINARY_NAME="neona"
INSTALL_DIR="/usr/local/bin"
GO_BIN_DIR="$HOME/go/bin"

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
CYAN='\033[0;96m'  # Light blue/cyan for banner
YELLOW='\033[0;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Logging functions with prefixes
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[FAILED]${NC} $1"
}

# ASCII Banner
show_banner() {
    echo ""
    echo -e "${CYAN}"
    cat << 'EOF'
 _  _  ____  _____  _  _    __   
( \( )( ___)(  _  )( \( )  /__\  
 )  (  )__)  )(_)(  )  (  /(__)\ 
(_)\_)(____)(_____)(_)\_)(__)(__)
EOF
    echo -e "${NC}"
    echo ""
}

show_banner
log_info "Starting Neona Control Plane installation..."

# Detect OS and Architecture
detect_platform() {
    OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
    ARCH="$(uname -m)"
    
    case "$OS" in
        linux)  OS="linux" ;;
        darwin) OS="darwin" ;;
        mingw*|msys*|cygwin*) OS="windows" ;;
        *)
            log_error "Unsupported operating system: $OS"
            exit 1
            ;;
    esac
    
    case "$ARCH" in
        x86_64|amd64)  ARCH="amd64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        armv7l)        ARCH="arm" ;;
        i386|i686)     ARCH="386" ;;
        *)
            log_error "Unsupported architecture: $ARCH"
            exit 1
            ;;
    esac
    
    # Set binary extension for Windows
    BINARY_EXT=""
    if [ "$OS" = "windows" ]; then
        BINARY_EXT=".exe"
    fi
    
    log_info "Detected platform: ${OS}-${ARCH}"
}

# Get latest release version from GitHub API
get_latest_version() {
    LATEST_VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" 2>/dev/null | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
    
    if [ -z "$LATEST_VERSION" ]; then
        log_warning "Could not fetch latest version. Trying to find any release..."
        # Fallback: get latest pre-release
        LATEST_VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases" 2>/dev/null | grep '"tag_name"' | head -1 | sed -E 's/.*"([^"]+)".*/\1/')
    fi
    
    if [ -z "$LATEST_VERSION" ]; then
        log_error "No releases found."
        return 1
    fi
    
    log_info "Latest version: ${LATEST_VERSION}"
}

# Download and install pre-built binary
install_from_release() {
    detect_platform
    
    if ! get_latest_version; then
        log_error "No releases available. Please install Go and try again."
        log_info "Install Go from: https://go.dev/dl/"
        exit 1
    fi
    
    # Asset naming: neona-{os}-{arch} (matches CI output)
    ASSET_NAME="neona-${OS}-${ARCH}${BINARY_EXT}"
    DOWNLOAD_URL="https://github.com/$REPO/releases/download/${LATEST_VERSION}/${ASSET_NAME}"
    
    log_info "Downloading: ${ASSET_NAME}"
    log_info "From: ${DOWNLOAD_URL}"
    
    # Create temporary directory
    TMP_DIR=$(mktemp -d)
    trap "rm -rf $TMP_DIR" EXIT
    
    # Download binary
    if ! curl -fsSL "$DOWNLOAD_URL" -o "$TMP_DIR/$BINARY_NAME$BINARY_EXT"; then
        log_error "Failed to download binary."
        log_info "Asset '${ASSET_NAME}' may not exist for this platform."
        log_info "Check releases at: https://github.com/$REPO/releases"
        log_info "Or install Go from https://go.dev/dl/ and try again."
        exit 1
    fi
    
    log_success "Download complete!"
    
    # Make executable
    chmod +x "$TMP_DIR/$BINARY_NAME$BINARY_EXT"
    
    # Install binary
    log_info "Installing to ${INSTALL_DIR}..."
    if [ -w "$INSTALL_DIR" ]; then
        mv "$TMP_DIR/$BINARY_NAME$BINARY_EXT" "$INSTALL_DIR/$BINARY_NAME"
    elif command -v sudo &> /dev/null; then
        log_info "Requesting sudo to install to $INSTALL_DIR"
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
            log_info "Added $LOCAL_BIN to PATH. Restart your terminal or run: source ~/.bashrc"
        fi
        
        INSTALL_DIR="$LOCAL_BIN"
    fi
    
    log_success "Neona ${LATEST_VERSION} installed to $INSTALL_DIR/$BINARY_NAME"
}

# Install via Go (preferred if Go is available)
install_from_go() {
    log_success "Go detected. Installing via 'go install'..."
    
    # Install directly from main repo
    go install github.com/$REPO/cmd/neona@latest
    
    # Try to make it globally available immediately via symlink
    if [ -d "/usr/local/bin" ]; then
        if sudo ln -sf "$GO_BIN_DIR/neona" /usr/local/bin/neona 2>/dev/null; then
             log_success "Linked to /usr/local/bin/neona (available instantly)"
        else
             # Fallback to PATH modification if sudo fails
             if [[ ":$PATH:" != *":$GO_BIN_DIR:"* ]]; then
                 log_info "sudo failed/unavailable. Adding $GO_BIN_DIR to PATH in ~/.bashrc"
                 echo 'export PATH=$PATH:$HOME/go/bin' >> ~/.bashrc
                 echo 'export PATH=$PATH:$HOME/go/bin' >> ~/.zshrc 2>/dev/null || true
             fi
        fi
    fi

    log_success "Neona installed successfully!"
}

# Main installation logic
main() {
    # Check for Go installation first (preferred method)
    if command -v go &> /dev/null; then
        install_from_go
    else
        log_info "Go not found. Downloading pre-built binary..."
        install_from_release
    fi
    
    # Verify installation
    echo ""
    if command -v neona &> /dev/null; then
        INSTALLED_VERSION=$(neona version 2>/dev/null || echo "unknown")
        log_success "Installation complete!"
        echo ""
        log_info "Version: ${INSTALLED_VERSION}"
        log_info "Run 'neona' to start."
    elif [ -f "/usr/local/bin/neona" ] || [ -f "$HOME/.local/bin/neona" ] || [ -f "$GO_BIN_DIR/neona" ]; then
        log_success "Installation complete!"
        log_warning "Restart your terminal or run: source ~/.bashrc"
    else
        log_error "Installation may have failed. Please check for errors above."
        exit 1
    fi
    
    echo ""
    log_info "Next steps:"
    echo "  1. Run 'neona daemon' to start the control plane"
    echo "  2. Run 'neona --help' for available commands"
}

main
