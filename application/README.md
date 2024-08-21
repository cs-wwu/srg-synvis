# Synthetic Vision System (SVS) in Go

```bash
go run cmd/svs/main.go
```

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
- [ ] Add LODs with ChunkManager
- [ ] Make Map and chunk rendering dynamic based on position
- [ ] Rework Main method to not use ticker-based system
- [ ] Change Camera to use Quaternions
- [ ] Re-implement UI (try to use original `asche`?)
- [ ] Find and fix GPU hanging
