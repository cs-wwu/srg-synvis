# Synthetic Vision in Go

This is the synthetic vision renderer/application written in Go

NOTE: When adding new modules and requirements, run `go mod tidy` in the
`srg-synvis/go` directory to update the required modules.

## Prerequisites

 - Go 1.20 or later
 - Vulkan SDK
 - GLFW

For Linux pre-install

```bash
sudo apt -y install mesa-vulkan-drivers vulkan-tools libvulkan-dev vulkan-validationlayers-dev spirv-tools
```

Run `vkcube` to check if correct so far

```bash
sudo apt -y install libglfw3-dev libglm-dev libxi-dev libxxf86vm-dev libxcursor-dev libxinerama-dev
```

## Installation

1. Clone the Repository
2. Use `go mod tidy` to install dependencies

## Building the application

To build the application, run `go build -o syvis ./cmd/syvis`

## Running the application

After building, you can run the application with `./syvis`

## Project Structures

```
go
├── assets              // Object assets
│   ├── heightmaps
│   ├── shaders
│   └── textures
├── cmd                 // Entry Point
│   └── syvis
│       └── main.go
├── internal            // Renderer and functions
│   ├── heightmap
│   │   └── heightmap.go
│   ├── mesh
│   │   ├── loader.go
│   │   └── unloader.go
│   ├── renderer
│   │   ├── camera.go
│   │   ├── renderer.go
│   │   └── vulkan.go
│   ├── shader
│   │   └── shader.go
│   ├── texture
│   │   └── texture.go
│   └── types
│       └── common.go
├── pkg                 // Vulkan Utilities
│   └── vulkanutil
│       └── vulkanutil.go
├── go.mod
└── README.md
```

