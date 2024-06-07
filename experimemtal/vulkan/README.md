# Vulkan
To run the file, use `make test`

For Linux pre-install

```
sudo apt install mesa-vulkan-drivers vulkan-tools
sudo apt install libvulkan-dev
sudo apt install vulkan-validationlayers-dev spirv-tools
```

Run vkcube to check if correct so far

```
sudo apt install libglfw3-dev
sudo apt install libglm-dev
sudo apt install libxxf86vm-dev libxi-dev
```

For the shader compiler
    download Google's binaries and copy glslc to `/usr/local`
    `sudo git clone https://github.com/google/shaderc/tree/main/glslc`


Additionally, for the Linux Tarball Vulkan SDK on RPi OS
`sudo apt install xz-utils`

```
sudo apt install libglm-dev cmake libxcb-dri3-0 libxcb-present0 libpciaccess0 \
libpng-dev libxcb-keysyms1-dev libxcb-dri3-dev libx11-dev g++ gcc \
libmirclient-dev libwayland-dev libxrandr-dev libxcb-randr0-dev libxcb-ewmh-dev \
git python python3 bison libx11-xcb-dev liblz4-dev libzstd-dev python3-distutils \
ocaml-core ninja-build pkg-config libxml2-dev wayland-protocols qtcreator \
qtbase5-dev qt5-qmake qtbase5-dev-tools
```

Run the `vulkansdk` script with `./vulkansdk -j 1` to restrict building to one CPU. 


## To Do List 
- [x] Render something
- [x] Texture mapping
- [x] Get some lighting
- [x] Make a Camera struct/object
- [ ] Make lighting more consistent
- [ ] Create a modular coordinate system
- [ ] Splice heightmaps together
- [ ] Create LODs based on Camera position

- [ ] Write it all in Go!
