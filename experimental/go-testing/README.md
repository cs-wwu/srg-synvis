# go-testing

Experimental vulkan testing in `Go`.

## To-Do List

- [x] Pull the VKCube Repo
- [x] Factor into modular project
- [x] Move Camera
- [x] Render Based on UBO
- [x] Load a Heightmap
- [x] Render a Heightmap
  - [x] Put Normal, Color Vectors back into Vertex struct
  - [x] Use the new UBO, make it more modular/efficient
  - [x] Make Shader's work like `main.cpp` shaders
- [ ] Load based on chunks
- [ ] Optimizations (30fps on RPi?)
  - [ ] Frustum Culling
  - [ ] LODs
- [ ] Create a modular coordinate system
- [ ] Splice Heightmaps together
- [ ] (opt) Render Text on-screen
- [ ] Pull Data from RPi Sensors
- [ ] Render Live Location from RPi Sensors
- [ ] Render View Vectors based on RPi Sensors

## Requirements

Note: These instructions are based on the application being run on a
RaspberryPi (which is linux-arm64).

1. Install `Go`

Download Go from the official site. (Version might change)

```bash
cd ~
wget https://go.dev/dl/go1.22.5.linux-arm64.tar.gz
tar -xzf go1.22.5.linux-arm64.tar.gz # Might need sudo
rm go1.22.5.linux-arm64.tar.gz
```

Add the following to the bottom of your `~/.profile`

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

4. Run `vkcube` to test Vulkan Libraries

5. Run `go run cmd/syvis/main.go` to run the application from project root
