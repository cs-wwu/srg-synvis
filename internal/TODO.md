# Interal TODOs

These are the internal system TODOs.

## TODO

### Cooper

Some of these changes will likely require a large amount of low-level changes
to the core engine. E.g., More than one map being loaded, not swapping entire
buffers when loading new chunks, etc. This may require new buffer management
strategies, or a better solution than the current pending/active buffer swap
solution (specifically for Chunk loading). I'm still not sure how to
implement the dynamic texture swapping.

- [x] Fix background colors
  - [x] Horizon fog
  - [x] Blue sky
  - [x] Better vertex colors? E.g.,
    - [x] Better randomized green colors
- [x] Fix texture striping on some devices
  - [ ] Better road colorings? (Kyle/Katria)
  
### Longterm

- [x] Refactor project
  - [x] Separate inputs into separate files
  - [x] More distinctly define "Debug" mode vs "Prod" mode

### Core System Overhaul

# Terrain System Implementation TODO List

## Core Memory System

- [x] Implement Memory Allocator interface
- [x] Build Bucket Allocator with percentage-based allocation
- [x] Create Memory Pools for each LOD level
- [x] Implement basic Memory Manager
- [x] Add simple Eviction Strategy

## Spatial Management System

- [x] Implement Grid System with cell-based storage
- [x] Build Cell Lookup for efficient spatial queries
- [x] Add Neighbor Search functionality
- [x] Create Chunk Tracker to manage chunk states
- [x] Implement Priority Calculator for chunk importance

## Basic Rendering Integration

- [x] Create simplified Terrain Manager
- [x] Integrate Memory System with Rendering
- [x] Implement Texture Manager
  - [x] Create texture loading/unloading functionality
  - [x] Implement LRU-based texture cache (max ~10 textures)
  - [x] Integrate with asche for Vulkan operations
- [x] Test basic functionality

## Stream Manager

- [x] Decides WHICH assets to load (coordinates with Asset Manager)
- [x] Manages chunk lifecycle (unloaded → loading → ready → evicting)
- [x] Coordinates between TerrainGrid, BucketAllocator, AssetManager
- [x] Handles loading strategy and eviction policy
- [x] Multithreading and async coordination

## Component Interfaces

- [x] Define StreamManager → TerrainGrid interface
- [x] Define StreamManager → BucketAllocator interface  
- [x] Define StreamManager → AssetLoader interface
- [x] Define StreamManager → AssetManager interface
- [x] Define TerrainRenderer → StreamManager interface
- [x] Define AssetManager interface (what raw data it provides)

## Asset Manager

- [x] Load/decode heightmap and watermap files on demand
- [x] Cache raw image data (LRU eviction by file, not chunk)
- [x] Handle file I/O errors gracefully
- [x] No knowledge of chunks or GPU - just provides raw data

## Next Steps

- [ ] Render only frustum-visible chunks
- [ ] Implement dynamic loading of chunk LODs
  - [ ] Chunk LODs should change as the camera moves

## Texture Management

- [ ] Implement texture streaming
  - [ ] Load textures on demand
  - [ ] Unload textures when not needed
  - [ ] Use LRU cache for texture management
- [ ] Implement texture atlas for terrain textures

## Memory Enhancement

- [ ] Dynamically decide on size and bucket sizes based on GPU
- [ ] Use a single buffer that is locally managed/split for faster allocation
- [ ] Change to `allocator`
  - [ ] Add capacity for a "models" folder for instanced rendering
  - [ ] Render Ocean chunk as instanced model
- [ ] Do we still need to store vertices/indices after GPU upload?
  - [ ] If not, we can shortcut when lowering LODs (use existing vertices/indices)

## Redo Chunk Calculations

- [x] Implement Stitching Helper or Redo Chunk calculations
- [ ] Fix Map edge calculations (wrapping around ambiguously because of `WorldToPixel`)

## New file structure

```sh
srg-synvis/
├── cmd/...
├── core/                    # Core systems (no dependencies on each other)
│   ├── memory/
│   ├── spatial/
│   └── textures/
├── internal/
│   ├── assets/              # NEW - Pure file I/O
│   │   ├── loader.go        # Raw heightmap/watermap loading
│   │   └── cache.go         # LRU cache for raw image data
│   ├── streaming/           # NEW - Coordination and strategy
│   │   ├── manager.go       # Main StreamManager
│   │   ├── strategy.go      # Loading/eviction strategies
│   │   └── state.go         # Chunk state machine
│   ├── Camera/              # Camera refactor
│   │   ├── camstuff.go      # Loading/eviction strategies
│   │   └── camera.go
│   ├── renderer/            # SIMPLIFIED - Pure rendering
│   │   ├── terrain.go       # New clean TerrainRenderer
│   │   ├── chunk.go         # Chunk mesh generation
│   │   ├── vulkan_*.go      # Vulkan setup/management
│   │   └── types.go
│   ├── models/
│   ├── processor/           # Maybe move to internal/assets?
│   └── sensors/
├── pkg/...
```

## Side Goals

- [x] Separate shader pipeline for Boris
- [x] Add camera turn param for Boris
- [x] Add render distance while I'm at it.
- [x] Add FOV as flag
- [x] Take the camera out of the renderer
- [x] Refactor the camera
  - [x] Input Interface
  - [x] Sensors Interface
  - [x] Motion Interface
  - [x] Enhance testing suite
