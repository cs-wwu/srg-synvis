#!/bin/bash

# Check if the script is run with sudo privileges
if [ "$EUID" -ne 0 ]; then
    echo "Please run this script with sudo privileges"
    exit 1
fi

# Function to get the latest Go version
get_latest_go_version() {
    wget -qO- https://golang.org/VERSION?m=text | cut -d' ' -f2
}

# Get the latest Go version
LATEST_GO_VERSION=$(get_latest_go_version)

# Check if the architecture flag is provided
if [ "$#" -ne 1 ]; then
    echo "Usage: $0 <arch>"
    echo "Where <arch> is either 'arm64' or 'amd64'"
    exit 1
fi

# Set the architecture based on the flag
ARCH=$1
if [ "$ARCH" != "arm64" ] && [ "$ARCH" != "amd64" ]; then
    echo "Invalid architecture. Please use 'arm64' or 'amd64'"
    exit 1
fi

# Download the appropriate Go package
DOWNLOADEDFILE="go${LATEST_GO_VERSION}.linux-${ARCH}.tar.gz"
wget "https://golang.org/dl/${DOWNLOADEDFILE}"

# Extract the downloaded file
tar -C /usr/local -xzf "$DOWNLOADEDFILE"

# Remove the downloaded file
rm "$DOWNLOADEDFILE"

# Set GOPATH and update PATH if not already set
GOPATH="$HOME/go"
if [[ ":$PATH:" != *":$GOPATH/bin:"* ]]; then
    echo "export GOPATH=$GOPATH" >> "$HOME/.bashrc"
    echo "export PATH=\$PATH:\$GOPATH/bin" >> "$HOME/.bashrc"
    source "$HOME/.bashrc"
fi

# Install required packages
apt update
apt -y install mesa-vulkan-drivers vulkan-tools libvulkan-dev vulkan-validationlayers-dev spirv-tools libglfw3-dev libglm-dev libxi-dev libxxf86vm-dev

echo "Go $LATEST_GO_VERSION has been installed successfully!"
echo "Please restart your terminal or run 'source ~/.bashrc' to apply the PATH changes."

# Run vkcube and check its exit status
echo "Running vkcube to verify Vulkan installation..."
timeout 5s vkcube > /dev/null 2>&1
VKCUBE_EXIT_STATUS=$?

if [ $VKCUBE_EXIT_STATUS -eq 124 ]; then
    echo "vkcube ran successfully for 5 seconds and was terminated."
    echo "Vulkan tools appear to be installed correctly."
elif [ $VKCUBE_EXIT_STATUS -eq 0 ]; then
    echo "vkcube ran successfully and exited normally."
    echo "Vulkan tools appear to be installed correctly."
else
    echo "vkcube encountered an error (exit status: $VKCUBE_EXIT_STATUS)."
    echo "There might be an issue with the Vulkan installation."
fi
