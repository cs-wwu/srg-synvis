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

- [ ] Refactor project
  - [ ] Separate inputs into separate files
  - [ ] More distinctly define "Debug" mode vs "Prod" mode

### Core System Overhaul

# Terrain System Implementation TODO List

## Phase 1: Core Memory System

- [x] Implement Memory Allocator interface
- [x] Build Bucket Allocator with percentage-based allocation
- [x] Create Memory Pools for each LOD level
- [x] Implement basic Memory Manager
- [x] Add simple Eviction Strategy

## Phase 2: Spatial Management System

- [x] Implement Grid System with cell-based storage
- [x] Build Cell Lookup for efficient spatial queries
- [x] Add Neighbor Search functionality
- [x] Create Chunk Tracker to manage chunk states
- [x] Implement Priority Calculator for chunk importance

## Phase 3: Basic Rendering Integration

- [ ] Create simplified Terrain Manager
- [ ] Implement basic Chunk Renderer
- [ ] Integrate Memory System with Rendering
- [x] Implement Texture Manager
  - [x] Create texture loading/unloading functionality
  - [x] Implement LRU-based texture cache (max ~10 textures)
  - [ ] Add texture-to-chunk mapping system
  - [ ] Integrate with asche for Vulkan operations
- [ ] Add minimal ocean rendering
- [ ] Test basic functionality

## Phase 4: Advanced Features

- [ ] Implement Stitching Helper
- [ ] Enhance Ocean Renderer with instancing
- [ ] Add multi-threaded chunk processing
- [ ] Implement advanced eviction strategies
- [ ] Optimize for performance

## Phase 5: Refinement and Optimization

- [ ] Implement texture blending for terrain transitions
- [ ] Add detail textures for close-up rendering
- [ ] Implement texture LOD system (mipmaps)
- [ ] Optimize texture memory usage
- [ ] Add debugging and visualization tools for texture usage
