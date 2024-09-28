#!/bin/bash

# This installs the latest version of go for your architecture
# This also installs all necessary vulkan libraries and tests them

# Color codes
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Check if the script is run with sudo privileges
if [ "$EUID" -ne 0 ]; then
    echo -e "${RED}Please run this script with sudo privileges${NC}"
    exit 1
fi

# Get the actual user (not root)
ACTUAL_USER=$(logname)
USER_HOME=$(eval echo ~$ACTUAL_USER)

# Function to get the latest Go version
get_latest_go_version() {
    wget -qO- https://golang.org/VERSION?m=text | sed -n 's/^go//p'
}

# Function to get the current installed Go version
get_current_go_version() {
    if /usr/local/go/bin/go version &> /dev/null; then
        /usr/local/go/bin/go version | cut -d' ' -f3 | sed 's/go//'
    else
        echo "none"
    fi
}

# Function to detect system architecture
detect_architecture() {
    local arch=$(uname -m)
    case $arch in
        x86_64)
            echo "amd64"
            ;;
        aarch64|armv8*|arm64)
            echo "arm64"
            ;;
        *)
            echo -e "${RED}Unsupported architecture: $arch${NC}"
            exit 1
            ;;
    esac
}

# Get the latest Go version
LATEST_GO_VERSION=$(get_latest_go_version)

# Get the current installed Go version
CURRENT_GO_VERSION=$(get_current_go_version)

# Detect system architecture
ARCH=$(detect_architecture)
echo "Detected architecture: $ARCH"

# Check if Go needs to be installed or updated
if [ "$CURRENT_GO_VERSION" = "$LATEST_GO_VERSION" ]; then
    echo "Go $CURRENT_GO_VERSION is already installed and up to date."
else
    echo "Installing Go $LATEST_GO_VERSION..."

    # Remove existing Go installation if it exists
    if [ -d "/usr/local/go" ]; then
        echo "Removing existing Go installation..."
        rm -rf /usr/local/go
    fi

    # Download the appropriate Go package
    DOWNLOADEDFILE="go${LATEST_GO_VERSION}.linux-${ARCH}.tar.gz"
    DOWNLOAD_URL="https://golang.org/dl/${DOWNLOADEDFILE}"
    echo "Downloading from: $DOWNLOAD_URL"
    wget "$DOWNLOAD_URL" > /dev/null

    if [ $? -ne 0 ]; then
        echo -e "${RED}Failed to download Go. Please check the URL and try again.${NC}"
        exit 1
    fi

    # Extract the downloaded file
    tar -C /usr/local -xzf "$DOWNLOADEDFILE"

    /usr/local/go/bin/go version

    if [ $? -ne 0 ]; then
        echo -e "${RED}Failed to install Go.${NC}"
    else
        echo -e "${GREEN}Go $LATEST_GO_VERSION has been installed successfully!${NC}"
    fi

    # Remove the downloaded file
    rm "$DOWNLOADEDFILE"
fi

# Set up Go environment for the actual user
cat << EOF | sudo -u $ACTUAL_USER tee -a $USER_HOME/.bashrc > /dev/null

# Go settings
export GOPATH=\$HOME/go
export PATH=\$PATH:/usr/local/go/bin:\$GOPATH/bin
EOF

# Make sure the GOPATH directory exists
sudo -u $ACTUAL_USER mkdir -p $USER_HOME/go

# Install required packages
echo "Installing Vulkan tools and related packages..."
apt update
apt -y install mesa-vulkan-drivers vulkan-tools libvulkan-dev vulkan-validationlayers-dev spirv-tools libglfw3-dev libglm-dev libxi-dev libxxf86vm-dev

# Run vkcube and check its exit status
echo "Running vkcube to verify Vulkan installation..."
timeout 1s vkcube > /dev/null 2>&1
VKCUBE_EXIT_STATUS=$?

if [ $VKCUBE_EXIT_STATUS -eq 124 ]; then
    echo -e "${GREEN}vkcube ran successfully for 21 second and was terminated.${NC}"
    echo "Vulkan tools appear to be installed correctly."
elif [ $VKCUBE_EXIT_STATUS -eq 0 ]; then
    echo -e "${GREEN}vkcube ran successfully and exited normally.${NC}"
    echo "Vulkan tools appear to be installed correctly."
else
    echo -e "${RED}vkcube encountered an error (exit status: $VKCUBE_EXIT_STATUS).${NC}"
    echo "There might be an issue with the Vulkan installation."
fi

echo "Go has been installed to /usr/local/go"
echo "GOPATH has been set to $USER_HOME/go for user $ACTUAL_USER"
echo "Please restart your terminal or run 'source ~/.bashrc' as $ACTUAL_USER to apply the PATH changes."
