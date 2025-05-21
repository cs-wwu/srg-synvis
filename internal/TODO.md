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
- [ ] Dynamic texture loading, can swap in and out of buffers
- [ ] Better Dynamic Chunk Loading
  - [ ] Remove loading based on map
  - [ ] Load each image into memory
  - [ ] Only load chunks in a ring around the player
- [ ] Fix chunk boundaries (with stitching/skirts)
- [ ] Re-implement frustum culling based upon performance

### Longterm

- [ ] Refactor project
  - [ ] Separate inputs into separate files
  - [ ] More distinctly define "Debug" mode vs "Prod" mode

## Terrain System Implementation Plan

### Overview

This plan details how to implement a terrain memory management and rendering system using a Grid-based spatial system with a Bucket-based memory allocator. The architecture is designed to efficiently manage terrain chunks, support LOD transitions, enable chunk stitching, and optimize ocean rendering.

### 1. Core Memory System

#### Memory Allocator

**Internal Workings:**

- Base interface that defines allocation operations
- Provides mechanisms to allocate, deallocate, and access GPU memory
- Tracks memory usage statistics and allocation states
- Implements safety checks and validation

**Usage:**

- Initialize with Vulkan device and memory properties
- Define memory requirements (type, alignment, etc.)
- Request memory allocations
- Release memory when no longer needed

#### Bucket Allocator

**Internal Workings:**

- Divides available GPU memory into percentage-based "buckets" for different LOD levels
- Each bucket is a contiguous memory region optimized for uniform chunk sizes
- Uses a simple append-based allocation strategy within each bucket
- Maintains metadata about allocations in each bucket
- Implements fast allocation through fixed stride lengths within buckets

**Usage:**

- Initialize with total memory budget and percentage allocation for each LOD level
- Request allocations for specific LOD levels
- Allocation returns offset into the appropriate bucket's memory
- Track allocations by chunk ID for later reference

#### Memory Pools

**Internal Workings:**

- Manages the actual Vulkan buffers and device memory for each bucket
- Handles memory type selection based on requirements
- Maps/unmaps memory for updates as needed
- Ensures proper alignment and padding

**Usage:**

- Created by the Bucket Allocator during initialization
- Provides raw memory access for uploading chunk data
- Exposes buffer handles for rendering commands

#### Memory Manager

**Internal Workings:**

- Coordinates between spatial system and memory allocation
- Implements policies for when to load/unload chunks
- Manages RAM cache of processed chunks
- Tracks chunk states (unloaded, processing, loaded)
- Schedules asynchronous chunk generation

**Usage:**

- Query if a chunk is loaded
- Request chunk loading at specific LOD
- Manage chunk transitions between LOD levels
- Schedule evictions when memory is needed

#### Eviction Strategy

**Internal Workings:**

- Implements policies for which chunks to evict when memory is full
- Maintains priority ordering of loaded chunks
- Can use least-recently-used, distance-based, or hybrid approach
- Handles graceful degradation under memory pressure

**Usage:**

- Called by Memory Manager when memory is needed
- Returns chunks that can be safely evicted
- Updates chunk state when eviction occurs

### 2. Spatial Management System

#### Grid System

**Internal Workings:**

- Divides world into fixed-size grid cells
- Maintains mapping between world coordinates and grid cells
- Stores chunks in cell-based collections for fast spatial lookup
- Provides efficient queries for chunks around a position

**Usage:**

- Add/remove chunks to/from the grid
- Query chunks in a specific cell
- Get chunks within a radius of a position
- Iterate over all tracked chunks

#### Cell Lookup

**Internal Workings:**

- Converts world coordinates to grid cell coordinates
- Implements hash-based lookup for efficient cell access
- Manages memory for cells (creating/removing as needed)
- Optimizes for common access patterns

**Usage:**

- Get cell key for a world position
- Retrieve cell contents by key
- Iterate over cells in a region

#### Neighbor Search

**Internal Workings:**

- Finds adjacent chunks for a given chunk
- Accounts for different LOD levels when determining neighbors
- Supports different neighbor patterns (4-way, 8-way, etc.)
- Handles edge cases where neighbors might not exist

**Usage:**

- Get all neighbors for a chunk
- Get specific neighbor in a direction
- Check if chunks are adjacent

#### Chunk Tracker

**Internal Workings:**

- Maintains collection of all active chunks
- Tracks chunk states (unprocessed, processing, ready, loaded)
- Updates chunk visibility based on camera position
- Manages chunk lifecycle from creation to deletion

**Usage:**

- Register new chunks
- Update chunk states
- Query chunks by various criteria
- Check if specific chunks exist

#### Priority Calculator

**Internal Workings:**

- Computes importance values for chunks based on multiple factors
- Considers distance from camera, visibility, and other heuristics
- Provides sorting function for priority-based processing
- Adapts to changing camera position and view direction

**Usage:**

- Calculate priority value for a chunk
- Sort chunks by priority
- Get top N priority chunks

### 3. Rendering System

#### Terrain Manager

**Internal Workings:**

- Central coordinator that ties together spatial and memory systems
- Implements the main update loop for chunk management
- Processes priority queue to determine which chunks to load/unload
- Dispatches rendering commands
- Handles multi-threaded chunk processing

**Usage:**

- Initialize with configuration parameters
- Call Update() each frame with camera position
- Call Render() to draw the terrain
- Access statistics and debug information

#### Chunk Renderer

**Internal Workings:**

- Manages Vulkan render pipelines for terrain chunks
- Creates descriptor sets for terrain rendering
- Batches draw calls for efficient rendering
- Handles different LOD levels with appropriate shaders
- Implements frustum culling

**Usage:**

- Register new chunk meshes for rendering
- Update chunk transforms
- Generate draw commands for visible chunks
- Apply rendering effects

#### Ocean Renderer

**Internal Workings:**

- Implements instanced rendering for ocean chunks
- Maintains a single template mesh for ocean surface
- Positions instances based on grid system
- Optimizes draw calls through instancing

**Usage:**

- Register ocean chunks in areas without terrain
- Update ocean parameters (waves, color, etc.)
- Generate draw commands for visible ocean areas

#### Stitching Helper

**Internal Workings:**

- Detects LOD transitions between adjacent chunks
- Implements stitching techniques (skirts or adaptive edges)
- Modifies chunk meshes or generates additional geometry
- Ensures seamless transitions between different detail levels

**Usage:**

- Check if stitching is needed between chunks
- Apply stitching technique to chunk edges
- Update stitching when LOD levels change

### Implementation Sequence

#### Phase 1: Core Memory System

1. Implement Memory Allocator interface
2. Build Bucket Allocator with percentage-based allocation
3. Create Memory Pools for each LOD level
4. Implement basic Memory Manager
5. Add simple Eviction Strategy

#### Phase 2: Spatial Management System

1. Implement Grid System with cell-based storage
2. Build Cell Lookup for efficient spatial queries
3. Add Neighbor Search functionality
4. Create Chunk Tracker to manage chunk states
5. Implement Priority Calculator for chunk importance

#### Phase 3: Basic Rendering Integration

1. Create simplified Terrain Manager
2. Implement basic Chunk Renderer
3. Integrate Memory System with Rendering
4. Add minimal ocean rendering
5. Test basic functionality

#### Phase 4: Advanced Features

1. Implement Stitching Helper
2. Enhance Ocean Renderer with instancing
3. Add multi-threaded chunk processing
4. Implement advanced eviction strategies
5. Optimize for performance
