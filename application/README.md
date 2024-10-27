# Synthetic Vision System (SVS) in Go

```bash
go run cmd/svs/main.go
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

- [x] Messily Move old stuff into Main System
- [x] Refactor Vulkan initializing data types into vulkan_consts.go
- [x] Refactor Heightmap loading
  - [x] Refactor Downloading into cmd/download/main.go
  - [x] Refactor API Usage into api/nasa.go
- [x] Rework Chunk Management
  - [x] Create a Chunk Manager
  - [x] Maps
    - [x] Heightmaps as ChunkManager Images (HGT)
    - [x] Watermaps as ChunkManager Images (SWB)
    - [x] Readd Processing of watermaps to texcoord[2]
    - [x] Undo Chunk Skirts
    - [x] Fix Last Triangle Not Rendering
    - [x] Redo Indices Management
    - [x] Re-implement ChunkManager as new rendering system
- [x] Fix `math.go` and/or iteration to load only necessary heightmap
- [x] Fix height rendering via Y coordinate (`heightmap.Gray16At(x, z)`)
- [x] Change heightmap loading and position to be modular based on arcseconds
  - [x] Attempt Rendering of more than one heightmap
- [x] Begin Implementation Unit and Integration Testing
- [x] Re-Work Frustum Culling with ChunkManager
- [x] Look at feasiblity of https://github.com/zeux/meshoptimizer
  - [x] If it looks good, create CGo bindings
  - [ ] Redo ChunkManager with meshoptimizer
  - [ ] Work in ChunkManager LODs with meshoptimizer
- [x] Fork `asche`:
  - [x] Allow for Physical Device Feature Settings (`vk.MultiDrawIndirect`)
  - [x] Allow for Command Pool Flag Settings (`vk.CommandPoolCreateResetCommandBufferBit`)
  - [ ] Use the forked `asche` to allow for UI implementation?
- [x] Find and fix all validation layer issues
  - [x] Fix Main validation layer issues
  - [ ] Fix on-close validation layer issues
- [x] Find and fix GPU hanging
- [ ] Make Map and chunk rendering dynamic based on position
- [ ] Rework Main method to not use ticker-based system
- [ ] Change Camera to use Quaternions (sensors?)

- [ ] Eventually remove local `asche` copy (public repo)
- [ ] Eventually remove local `meshopt` copy (public repo)
