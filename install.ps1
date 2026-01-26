# PowerShell installation script for Windows
# Installs the version of Go required for the SVS

# Requires administrator privileges
#Requires -RunAsAdministrator

# Function to print colored messages
function Write-ColorOutput($ForegroundColor, $Message) {
    Write-Host $Message -ForegroundColor $ForegroundColor
}

# Function to print error and exit
function Exit-WithError($Message) {
    Write-ColorOutput Red "Error: $Message"
    exit 1
}

# Check if go.mod exists
if (-not (Test-Path "go.mod")) {
    Exit-WithError "go.mod file not found in current directory"
}

# Get Go version from go.mod
$goModContent = Get-Content "go.mod"
$goVersionLine = $goModContent | Where-Object { $_ -match "^go\s+" } | Select-Object -First 1
if (-not $goVersionLine) {
    Exit-WithError "Could not determine Go version from go.mod"
}

$GO_VERSION = ($goVersionLine -split "\s+")[1]
if (-not $GO_VERSION) {
    Exit-WithError "Could not parse Go version from go.mod"
}

Write-ColorOutput Green "Required Go version: $GO_VERSION"

# Check if Go is already installed and version is sufficient
$currentGoVersion = $null
if (Get-Command go -ErrorAction SilentlyContinue) {
    $goVersionOutput = go version
    if ($goVersionOutput -match "go(\d+\.\d+\.?\d*)") {
        $currentGoVersion = $matches[1]
        Write-ColorOutput Green "Current Go version: $currentGoVersion"
        
        # Compare versions
        $requiredVersion = [version]$GO_VERSION
        $installedVersion = [version]$currentGoVersion
        
        if ($installedVersion -ge $requiredVersion) {
            Write-ColorOutput Green "Go $currentGoVersion is already installed and meets the requirement ($GO_VERSION)"
            exit 0
        }
    }
}

# Determine architecture
$ARCH = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { "386" }
Write-ColorOutput Green "Detected architecture: $ARCH"

# Install Go
Write-ColorOutput Green "Installing Go $GO_VERSION..."

# Set installation directory
$GO_INSTALL_DIR = "$env:USERPROFILE\.go"
if (-not (Test-Path $GO_INSTALL_DIR)) {
    New-Item -ItemType Directory -Path $GO_INSTALL_DIR -Force | Out-Null
}

# Download Go
$GO_FILENAME = "go${GO_VERSION}.windows-${ARCH}.zip"
$GO_URL = "https://golang.org/dl/${GO_FILENAME}"
$TEMP_FILE = "$env:TEMP\${GO_FILENAME}"

Write-ColorOutput Green "Downloading from $GO_URL..."
try {
    Invoke-WebRequest -Uri $GO_URL -OutFile $TEMP_FILE -UseBasicParsing
} catch {
    Exit-WithError "Failed to download Go: $_"
}

# Extract Go
Write-ColorOutput Green "Extracting Go..."
try {
    # Remove old installation if it exists
    if (Test-Path "$GO_INSTALL_DIR\go") {
        Remove-Item -Path "$GO_INSTALL_DIR\go" -Recurse -Force
    }
    
    Expand-Archive -Path $TEMP_FILE -DestinationPath $GO_INSTALL_DIR -Force
    Remove-Item $TEMP_FILE
} catch {
    Exit-WithError "Failed to extract Go: $_"
}

# Update PATH environment variable
$GO_BIN_PATH = "$GO_INSTALL_DIR\go\bin"
$GOROOT = "$GO_INSTALL_DIR\go"
$GOPATH = "$env:USERPROFILE\go"

Write-ColorOutput Green "Updating environment variables..."

# Set GOROOT
[Environment]::SetEnvironmentVariable("GOROOT", $GOROOT, "User")

# Set GOPATH
[Environment]::SetEnvironmentVariable("GOPATH", $GOPATH, "User")

# Update PATH
$currentPath = [Environment]::GetEnvironmentVariable("PATH", "User")

# Remove any existing Go paths
$pathArray = $currentPath -split ";" | Where-Object { 
    $_ -notmatch "\\\.go\\go\\bin" -and 
    $_ -notmatch "\\go\\bin" -and
    $_ -ne ""
}

# Add new Go paths
$newPath = (@($GO_BIN_PATH, "$GOPATH\bin") + $pathArray) -join ";"
[Environment]::SetEnvironmentVariable("PATH", $newPath, "User")

# Also update current session PATH
$env:PATH = "$GO_BIN_PATH;$GOPATH\bin;$env:PATH"
$env:GOROOT = $GOROOT
$env:GOPATH = $GOPATH

Write-ColorOutput Green "Installation complete!"
Write-ColorOutput Green "Go $GO_VERSION has been installed to: $GOROOT"
Write-ColorOutput Green "GOPATH set to: $GOPATH"
Write-ColorOutput Green ""
Write-ColorOutput Green "Note: You may need to restart your terminal or application for changes to take effect."

# Verify installation
Write-ColorOutput Green "Verifying installation..."
$verifyOutput = & "$GO_BIN_PATH\go.exe" version 2>&1
if ($LASTEXITCODE -eq 0) {
    Write-ColorOutput Green "Verification successful: $verifyOutput"
} else {
    Write-ColorOutput Yellow "Warning: Could not verify Go installation. Please restart your terminal and run 'go version'."
}
