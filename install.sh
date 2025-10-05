#!/bin/bash

# This script installs vulkan dependencies for the current user
# and installs the version of Go required for the SVS

# Color codes
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color
TEMP_IMGUI_DIR="/tmp/imgui-download"
IMGUI_VERSION="1.92.3"
IMGUI_INSTALL_DIR="pkg/imgui"

# Function to print error and exit
error_exit() {
    echo -e "${RED}Error: $1${NC}" >&2
    exit 1
}

# Check if we're on a supported system
if ! command -v apt >/dev/null 2>&1; then
    error_exit "This script requires apt package manager"
fi

# Get architecture
ARCH=$(dpkg --print-architecture)
if [[ "$ARCH" != "amd64" && "$ARCH" != "arm64" ]]; then
    error_exit "Unsupported architecture: $ARCH. Only amd64 and arm64 are supported."
fi

# Install Vulkan dependencies if running as root
if [ "$EUID" -eq 0 ]; then
    echo -e "${GREEN}Installing Vulkan dependencies...${NC}"
    apt update || error_exit "Failed to update apt"
    apt -y install mesa-vulkan-drivers vulkan-tools libvulkan-dev vulkan-validationlayers-dev spirv-tools libglfw3-dev \
        libglm-dev libxi-dev libxxf86vm-dev libxcursor-dev libxinerama-dev libxrandr-dev ||
        error_exit "Failed to install Vulkan dependencies"

    # Get the original user who ran the script with sudo
    REAL_USER=${SUDO_USER:-$USER}
    if [ "$REAL_USER" = "root" ]; then
        error_exit "Please run this script as a normal user with sudo"
    fi

    # Save original working directory and re-run script as the original user
    ORIGINAL_PWD="$PWD"
    echo -e "${GREEN}Running Go installation as user $REAL_USER...${NC}"
    su - "$REAL_USER" -c "cd \"$ORIGINAL_PWD\" && $(readlink -f "$0")"
    exit 0
fi

# Get Go version from go.mod
if [ ! -f "go.mod" ]; then
    error_exit "go.mod file not found in current directory"
fi

GO_VERSION=$(grep -m 1 "^go " go.mod | awk '{print $2}')
if [ -z "$GO_VERSION" ]; then
    error_exit "Could not determine Go version from go.mod"
fi

# Install ImGui
echo -e "${GREEN}Installing ImGui v$IMGUI_VERSION...${NC}"
rm -r "$IMGUI_INSTALL_DIR"
rm -r "imgui-build"
mkdir "imgui-build"
mkdir -p "$TEMP_IMGUI_DIR"
mkdir -p "$IMGUI_INSTALL_DIR"
mkdir -p "$IMGUI_INSTALL_DIR/backends"
IMGUI_URL="https://github.com/ocornut/imgui/archive/refs/tags/v$IMGUI_VERSION.tar.gz"
wget -O "$TEMP_IMGUI_DIR/source.tar.gz" "$IMGUI_URL"
tar -xzf "$TEMP_IMGUI_DIR/source.tar.gz" -C "$TEMP_IMGUI_DIR"
EXTRACTED_IMGUI="$TEMP_IMGUI_DIR/imgui-$IMGUI_VERSION"
find "$EXTRACTED_IMGUI" -maxdepth 1 -type f -not -name ".*" -exec cp {} "$IMGUI_INSTALL_DIR/" \;
cp "$EXTRACTED_IMGUI/backends/imgui_impl_glfw.cpp" "$IMGUI_INSTALL_DIR/backends"
cp "$EXTRACTED_IMGUI/backends/imgui_impl_glfw.h" "$IMGUI_INSTALL_DIR/backends"
cp "$EXTRACTED_IMGUI/backends/imgui_impl_vulkan.cpp" "$IMGUI_INSTALL_DIR/backends"
cp "$EXTRACTED_IMGUI/backends/imgui_impl_vulkan.h" "$IMGUI_INSTALL_DIR/backends"
rm -r "$TEMP_IMGUI_DIR"

cd imgui-build
cmake ..
make
cd ..

# Check if Go is already installed and version is sufficient
if command -v go >/dev/null 2>&1; then
    CURRENT_GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    if [ "$(printf '%s\n' "$GO_VERSION" "$CURRENT_GO_VERSION" | sort -V | head -n1)" = "$GO_VERSION" ]; then
        echo -e "${GREEN}Go $CURRENT_GO_VERSION is already installed and meets the requirement ($GO_VERSION)${NC}"
        exit 0
    fi
fi

# Install Go
echo -e "${GREEN}Installing Go $GO_VERSION...${NC}"
GO_INSTALL_DIR="$HOME/.go"
mkdir -p "$GO_INSTALL_DIR"

# Download and install Go
GO_FILENAME="go${GO_VERSION}.linux-${ARCH}.tar.gz"
GO_URL="https://golang.org/dl/${GO_FILENAME}"
wget -q "$GO_URL" -O "/tmp/${GO_FILENAME}" || error_exit "Failed to download Go"
tar -C "$GO_INSTALL_DIR" -xzf "/tmp/${GO_FILENAME}" || error_exit "Failed to extract Go"
rm "/tmp/${GO_FILENAME}"


# Detect shell and update PATH
SHELL_TYPE=$(basename "$SHELL")
SHELL_RC=""
GO_PATH_ENTRY="export PATH=$GO_INSTALL_DIR/bin:$GO_INSTALL_DIR/go/bin:\$PATH"
GO_ROOT_ENTRY="export GOROOT=$GO_INSTALL_DIR/go"
GO_PATH_ENTRY2="export GOPATH=\$HOME/go"

case "$SHELL_TYPE" in
"bash")
    SHELL_RC="$HOME/.bashrc"
    ;;
"zsh")
    SHELL_RC="$HOME/.zshrc"
    ;;
"fish")
    SHELL_RC="$HOME/.config/fish/config.fish"
    GO_PATH_ENTRY="set -x PATH $GO_INSTALL_DIR/go/bin \$PATH"
    GO_ROOT_ENTRY="set -x GOROOT $GO_INSTALL_DIR/go"
    GO_PATH_ENTRY2="set -x GOPATH \$HOME/.go"
    mkdir -p "$(dirname "$SHELL_RC")"
    ;;
*)
    error_exit "Unsupported shell: $SHELL_TYPE"
    ;;
esac

# Update shell configuration
if [ ! -f "$SHELL_RC" ]; then
    touch "$SHELL_RC"
fi

# Remove any existing Go PATH entries
sed -i '/export.*GOROOT/d' "$SHELL_RC" 2>/dev/null
sed -i '/export.*GOPATH/d' "$SHELL_RC" 2>/dev/null
sed -i '/set -x.*GOROOT/d' "$SHELL_RC" 2>/dev/null
sed -i '/set -x.*GOPATH/d' "$SHELL_RC" 2>/dev/null
sed -i '/export.*\/go\/bin/d' "$SHELL_RC" 2>/dev/null
sed -i '/set -x PATH.*\/go\/bin/d' "$SHELL_RC" 2>/dev/null

# Add new Go PATH entries
echo "$GO_ROOT_ENTRY" >>"$SHELL_RC"
echo "$GO_PATH_ENTRY2" >>"$SHELL_RC"
echo "$GO_PATH_ENTRY" >>"$SHELL_RC"

echo -e "${GREEN}Installation complete!${NC}"
echo -e "${GREEN}Please restart your shell or run: source $SHELL_RC${NC}"
