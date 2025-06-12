#!/bin/bash

# claude-squad installation script
# This script downloads and installs the latest claude-squad binary

set -e

# Configuration
GITHUB_REPO="smtg-ai/claude-squad"
BINARY_NAME="cs"
INSTALL_DIR="${HOME}/bin"
LATEST_VERSION=""  # Will be updated by GitHub Actions

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Helper functions
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
    echo -e "${RED}[ERROR]${NC} $1"
}

# Detect OS and architecture
detect_platform() {
    local os arch
    
    # Detect OS
    case "$(uname -s)" in
        Linux*)     os="linux" ;;
        Darwin*)    os="darwin" ;;
        CYGWIN*|MINGW*|MSYS*) os="windows" ;;
        *)          log_error "Unsupported operating system: $(uname -s)" && exit 1 ;;
    esac
    
    # Detect architecture
    case "$(uname -m)" in
        x86_64|amd64)   arch="amd64" ;;
        arm64|aarch64)  arch="arm64" ;;
        *)              log_error "Unsupported architecture: $(uname -m)" && exit 1 ;;
    esac
    
    echo "${os}-${arch}"
}

# Get latest version from GitHub API
get_latest_version() {
    if [ -n "$LATEST_VERSION" ]; then
        echo "$LATEST_VERSION"
        return
    fi
    
    log_info "Fetching latest version from GitHub..."
    local latest_version
    if command -v curl >/dev/null 2>&1; then
        latest_version=$(curl -s "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    elif command -v wget >/dev/null 2>&1; then
        latest_version=$(wget -qO- "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    else
        log_error "Neither curl nor wget is available. Please install one of them."
        exit 1
    fi
    
    if [ -z "$latest_version" ]; then
        log_error "Failed to fetch latest version"
        exit 1
    fi
    
    echo "$latest_version"
}

# Download binary
download_binary() {
    local version="$1"
    local platform="$2"
    local extension=""
    
    if [[ "$platform" == *"windows"* ]]; then
        extension=".exe"
    fi
    
    local binary_name="${BINARY_NAME}-${platform}${extension}"
    local download_url="https://github.com/${GITHUB_REPO}/releases/download/${version}/${binary_name}"
    local temp_file="/tmp/${binary_name}"
    
    log_info "Downloading ${binary_name}..."
    log_info "URL: ${download_url}"
    
    if command -v curl >/dev/null 2>&1; then
        if ! curl -L -o "$temp_file" "$download_url"; then
            log_error "Failed to download binary"
            exit 1
        fi
    elif command -v wget >/dev/null 2>&1; then
        if ! wget -O "$temp_file" "$download_url"; then
            log_error "Failed to download binary"
            exit 1
        fi
    else
        log_error "Neither curl nor wget is available"
        exit 1
    fi
    
    echo "$temp_file"
}

# Install binary
install_binary() {
    local temp_file="$1"
    local install_path="${INSTALL_DIR}/${BINARY_NAME}"
    
    # Create install directory if it doesn't exist
    if [ ! -d "$INSTALL_DIR" ]; then
        log_info "Creating install directory: $INSTALL_DIR"
        mkdir -p "$INSTALL_DIR"
    fi
    
    # Make binary executable and move to install directory
    chmod +x "$temp_file"
    mv "$temp_file" "$install_path"
    
    log_success "Installed $BINARY_NAME to $install_path"
}

# Check if binary is in PATH
check_path() {
    if ! echo "$PATH" | grep -q "$INSTALL_DIR"; then
        log_warning "$INSTALL_DIR is not in your PATH"
        log_info "Add the following line to your shell profile (~/.bashrc, ~/.zshrc, etc.):"
        echo "    export PATH=\"\$PATH:$INSTALL_DIR\""
        log_info "Then reload your shell or run: source ~/.bashrc"
    else
        log_success "$INSTALL_DIR is in your PATH"
    fi
}

# Verify installation
verify_installation() {
    local install_path="${INSTALL_DIR}/${BINARY_NAME}"
    
    if [ -x "$install_path" ]; then
        log_success "Installation verified!"
        log_info "Run '$BINARY_NAME --help' to get started"
        
        # Show version if binary is working
        if "$install_path" version >/dev/null 2>&1; then
            log_info "Installed version:"
            "$install_path" version
        fi
    else
        log_error "Installation verification failed"
        exit 1
    fi
}

# Cleanup function
cleanup() {
    local temp_file="$1"
    if [ -f "$temp_file" ]; then
        rm -f "$temp_file"
    fi
}

# Main installation function
main() {
    log_info "claude-squad installation script"
    log_info "Repository: https://github.com/${GITHUB_REPO}"
    echo
    
    # Check for required tools
    if ! command -v curl >/dev/null 2>&1 && ! command -v wget >/dev/null 2>&1; then
        log_error "This script requires either curl or wget to be installed"
        exit 1
    fi
    
    # Detect platform
    local platform
    platform=$(detect_platform)
    log_info "Detected platform: $platform"
    
    # Get latest version
    local version
    version=$(get_latest_version)
    log_info "Latest version: $version"
    
    # Download binary
    local temp_file
    temp_file=$(download_binary "$version" "$platform")
    
    # Install binary
    install_binary "$temp_file"
    
    # Check PATH
    check_path
    
    # Verify installation
    verify_installation
    
    # Cleanup
    cleanup "$temp_file"
    
    echo
    log_success "claude-squad installation completed!"
    log_info "You can now run: $BINARY_NAME"
}

# Handle script arguments
case "${1:-}" in
    --help|-h)
        echo "claude-squad installation script"
        echo
        echo "Usage: $0 [options]"
        echo
        echo "Options:"
        echo "  --help, -h     Show this help message"
        echo "  --version, -v  Show version and exit"
        echo
        echo "Environment variables:"
        echo "  INSTALL_DIR    Installation directory (default: \$HOME/bin)"
        echo
        echo "Examples:"
        echo "  # Install to default location"
        echo "  curl -fsSL https://raw.githubusercontent.com/${GITHUB_REPO}/main/install.sh | sh"
        echo
        echo "  # Install to custom directory"
        echo "  INSTALL_DIR=/usr/local/bin curl -fsSL https://raw.githubusercontent.com/${GITHUB_REPO}/main/install.sh | sh"
        exit 0
        ;;
    --version|-v)
        echo "claude-squad installation script v1.0.0"
        exit 0
        ;;
esac

# Run main function
main "$@"