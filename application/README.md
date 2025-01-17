# Synthetic Vision System (SVS) in Go

```bash
go run cmd/svs/main.go
```

## Possible Command Arguments

```bash
go run cmd/svs/main.go
  -gps Enable GPS
  -hat Enable Hat
  -pos <"lat,lon,hgt"> Starting Camera Position (lat,lon,height) where height is in meters (default "48.0,-123.0,1300") 
  -monitor <int> Monitor selection (0 = windowed, 1 = primary monitor, etc.)
```

## Installation

1. Download the latest version of Go

    a. Download and install Go

    ```bash
    cd ~
    wget https://go.dev/dl/go1.23.1.linux-arm64.tar.gz
    sudo tar -xzf go1.23.1.linux-arm64.tar.gz
    rm go1.23.1.linux-arm64.tar.gz
    ```

    b. Add the following to the bottom of your `~/.profile`

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

2. Run `go version` to test sucessfull install

3. Install `Vulkan` Libraries

```bash
sudo apt -y install mesa-vulkan-drivers vulkan-tools libvulkan-dev vulkan-validationlayers-dev spirv-tools libglfw3-dev libglm-dev libxi-dev libxxf86vm-dev
```

4. Run `vkcube` and `go run cmd/svs/main.go` to test installation.

## TODO

- [ ] Simplify chunk logic
- [ ] Add capacity to render 4 maps at once
  - [ ] Find max memory based on LODs and distance
  - [ ] Increase memory capacity
  - [ ] If map do not exist, load water square (stitch to corners)
  - [ ] Add LOD stitching
  - [ ] Add dynamic map loading
- [ ] Add dynamic texture loading per map (kyle/roads)
  - [ ] Fix texture striping
- [ ] Simplify frustum logic
  - [ ] Move frustum culling from app to TerrainManager
  - [ ] Test performance difference
- [ ] Change Camera to use Quaternions (sensors?)

- [ ] Eventually remove local `asche` copy (public repo)
