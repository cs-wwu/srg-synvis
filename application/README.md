# Synthetic Vision System (SVS) in Go

```bash
go run cmd/svs/main.go
```

## TODO

- [x] Messily Move old stuff into Main System
- [x] Refactor Vulkan initializing data types into vulkan_consts.go
- [ ] Rework Chunk Management
  - [ ] Create a Chunk Manager
    - [ ] Frustum Culling
    - [ ] LODs
  - [ ] Maps
    - [ ] Heightmaps as App Images (HGT)
    - [ ] Watermaps as App Images (SWB)
  - [ ] Readd Processing of watermaps to texcoord[2]
  - [ ] Undo Chunk Skirts
  - [ ] Fix Last Triangle Not Rendering
  - [ ] Redo Indices Management
- [ ] Rework Main method to not use ticker-based system
- [x] Refactor Heightmap loading
  - [x] Refactor Downloading into cmd/download/main.go
  - [x] Refactor API Usage into api/nasa.go
- [ ] Change heightmap loading and position to be modular based on arcminutes
  - [ ] Attempt Rendering of more than one heightmap
- [ ] Change Camera to use Quaternions
- [ ] Begin Implementation Unit and Integration Testing
- [ ] Re-implement UI (try to use original `asche`?)
