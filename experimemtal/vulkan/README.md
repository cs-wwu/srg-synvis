# Vulkan
To run the file, use `make test`

For Linux pre-install

```
sudo apt -y install mesa-vulkan-drivers vulkan-tools libvulkan-dev vulkan-validationlayers-dev spirv-tools
```

Run `vkcube` to check if correct so far

```
sudo apt -y install libglfw3-dev libglm-dev libxi-dev
```

For the shader compiler
    download Google's binaries and copy glslc to `/usr/local`
    ```sudo git clone https://github.com/google/shaderc/tree/main/glslc```


Additionally, for the Linux Tarball Vulkan SDK on RPi OS
```sudo apt install xz-utils```

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
