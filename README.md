# SRG SynVis

Synthetic Vision System (SVS) for Systems Research Group at WWU

## Running the Application

```bash
go run ./cmd/svs
```

### Command Arguments

```bash
go run ./cmd/svs
  -gps
     Enable GPS
  -hat
     Enable Hat
  -monitor int
     Monitor selection (0 = windowed, 1 = primary monitor, etc.)
  -pos string
     Starting Camera Position (lat,lon,height) where height is in meters (default "48.5,-123.5,2600")
  -dir string
     Starting Camera Direction (x,y,z) - normalized automatically (default "0,0,-1")
  -render-distance float
     Maximum render distance in world units (default 5000)
  -disable-warnings
     Disable terrain height warnings (red coloring)
```

### Download Tool

The Download api must have at least one "Download" selection (nasa/faa/osm)

```bash
go run ./cmd/download
  -nasa             Download NASA elevation data
  -faa              Download FAA airport/navigation data
  -osm              Download OpenStreetMap data
  -v                Verbose output
  -bbox   <string>  Bounding box coordinates (e.g., '49,-125,45,-118') in format 'n, e, s, w'
  -out    <string>  Output directory for downloaded files (default "assets/maps/")
  -preset <string>  Use preset region: [bellingham/washington/...]
  -region <string>  Comma-separated list of region codes (e.g., 'n48w123,n48w122')
```

## Installation

To install all necessary libraries and the latest `Go` version, run:

```bash
sudo ./install.sh
```

### Manual Installation Steps

1. Download the latest version of Go

    1. Through apt

        ```bash
        sudo apt -y install golang-go
        ```

    2. Or manually

        ```bash
        cd ~
        wget https://go.dev/dl/go1.24.2.linux-arm64.tar.gz
        sudo tar -xzf go1.24.2.linux-arm64.tar.gz
        rm go1.24.2.linux-arm64.tar.gz
        ```

2. Add the following to the bottom of your `~/.profile`

    ```bash
    # Set GOPATH to the default user workspace if it is not already set
    if [ -z "$GOPATH" ]; then
        export GOPATH=$HOME/go
    fi
    # Optionally, add GOPATH/bin to PATH for easier access to installed Go binaries
    if [ -d "$GOPATH/bin" ]; then
        export PATH=$PATH:$GOPATH/bin
    fi
    ```

3. Run `go version` to test successful install

4. Install Vulkan Libraries

    ```bash
    sudo apt -y install mesa-vulkan-drivers vulkan-tools libvulkan-dev vulkan-validationlayers-dev spirv-tools libglfw3-dev libglm-dev libxi-dev libxxf86vm-dev libxcursor-dev libxinerama-dev libxrandr-dev
    ```

    You may run into issues with the `vulkan-validationlayers-dev` package. If so, use the following package instead: `vulkan-utility-libraries-dev`.

5. Run `vkcube` and `go run cmd/svs/main.go` to test installation

## Guidelines

- Check in code to the repo, including experimental investigations
- Don't check in gigantic data sets. It's ok to check in small data sets for experimentation
- There will be further guidelines for testing and continuous integration when we get to that point

## Founding Members

Ada Kolze, Cooper Morgan, Kyle Peterson, See-Mong Tan

## License

MIT
