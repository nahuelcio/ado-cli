#!/bin/bash
set -e

# Azure DevOps CLI Installer
# Usage: curl -fsSL https://raw.githubusercontent.com/your-org/azure-devops-cli/main/install.sh | bash

REPO="nahuelcio/ado-cli"
BINARY_NAME="ado"
INSTALL_DIR="/usr/local/bin"
VERSION="${VERSION:-latest}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Detect OS and Architecture
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)
    
    case $OS in
        linux)
            PLATFORM="linux"
            ;;
        darwin)
            PLATFORM="darwin"
            ;;
        *)
            echo -e "${RED}Error: Unsupported OS: $OS${NC}"
            exit 1
            ;;
    esac
    
    case $ARCH in
        x86_64|amd64)
            ARCH="amd64"
            ;;
        aarch64|arm64)
            ARCH="arm64"
            ;;
        *)
            echo -e "${RED}Error: Unsupported architecture: $ARCH${NC}"
            exit 1
            ;;
    esac
    
    TARGET="${PLATFORM}-${ARCH}"
}

# Get latest release version
get_latest_version() {
    if [ "$VERSION" = "latest" ]; then
        echo "Fetching latest version..."
        VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
        if [ -z "$VERSION" ]; then
            echo -e "${RED}Error: Could not fetch latest version${NC}"
            exit 1
        fi
    fi
    echo -e "${GREEN}Installing $BINARY_NAME $VERSION...${NC}"
}

# Download and install
download_and_install() {
    DOWNLOAD_URL="https://github.com/$REPO/releases/download/$VERSION/ado-${TARGET}.tar.gz"
    TEMP_DIR=$(mktemp -d)
    
    echo "Downloading from $DOWNLOAD_URL..."
    if ! curl -fsSL "$DOWNLOAD_URL" -o "$TEMP_DIR/$BINARY_NAME.tar.gz"; then
        echo -e "${RED}Error: Failed to download${NC}"
        rm -rf "$TEMP_DIR"
        exit 1
    fi
    
    echo "Extracting..."
    tar -xzf "$TEMP_DIR/$BINARY_NAME.tar.gz" -C "$TEMP_DIR"
    
    echo "Installing to $INSTALL_DIR..."
    if [ -w "$INSTALL_DIR" ]; then
        mv "$TEMP_DIR/$BINARY_NAME" "$INSTALL_DIR/"
    else
        echo -e "${YELLOW}Requesting sudo access to install to $INSTALL_DIR...${NC}"
        sudo mv "$TEMP_DIR/$BINARY_NAME" "$INSTALL_DIR/"
    fi
    
    chmod +x "$INSTALL_DIR/$BINARY_NAME"
    rm -rf "$TEMP_DIR"
}

# Verify installation
verify_installation() {
    if command -v "$BINARY_NAME" &> /dev/null; then
        INSTALLED_VERSION=$($BINARY_NAME --version 2>/dev/null || echo "unknown")
        echo -e "${GREEN}✓ Successfully installed $BINARY_NAME${NC}"
        echo -e "  Version: $INSTALLED_VERSION"
        echo -e "  Location: $(command -v $BINARY_NAME)"
        echo ""
        echo "Quick start:"
        echo "  $BINARY_NAME --help"
        echo "  $BINARY_NAME profile add --name myorg --org https://dev.azure.com/myorg --project myproject"
        echo "  $BINARY_NAME auth login --profile myorg"
    else
        echo -e "${RED}Error: Installation failed${NC}"
        exit 1
    fi
}

# Main
main() {
    echo "Azure DevOps CLI Installer"
    echo "=========================="
    echo ""
    
    detect_platform
    get_latest_version
    download_and_install
    verify_installation
}

# Handle arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --version)
            VERSION="$2"
            shift 2
            ;;
        --install-dir)
            INSTALL_DIR="$2"
            shift 2
            ;;
        --help)
            echo "Usage: install.sh [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --version <version>     Install specific version (default: latest)"
            echo "  --install-dir <dir>     Installation directory (default: /usr/local/bin)"
            echo "  --help                  Show this help message"
            echo ""
            echo "Examples:"
            echo "  curl -fsSL ... | bash"
            echo "  curl -fsSL ... | bash -s -- --version v1.0.0"
            echo "  curl -fsSL ... | bash -s -- --install-dir ~/.local/bin"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

main
