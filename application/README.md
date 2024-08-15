# Synthetic Vision System (SVS) in Go

```bash
go run cmd/svs/main.go
```

## TODO

- [x] Messily Move old stuff into Main System
- [ ] Refactor Vulkan initializing data types into vulkan-consts.go
- [ ] Rework Chunk Management
  - [ ] Create a Chunk Manager
    - [ ] Frustum Culling
    - [ ] LODs
  - [ ] Maps
    - [ ] Heightmaps as App Images (HGT)
    - [ ] Watermaps as App Images (SWB)
  - [ ] Undo Chunk Skirts
  - [ ] Fix Last Triangle Not Rendering
  - [ ] Redo Indices Management
- [ ] Rework Main method to not use ticker-based system
- [ ] Refactor Heightmap loading
  - [ ] Refactor Downloading into cmd/svs/download.go
  - [ ] Readd Processing of watermaps to texcoord[2]
  - [ ] Refactor API Usage into api/nasa.go
- [ ] Change heightmap loading and position to be modular based on arcminutes
  - [ ] Attempt Rendering of more than one heightmap
- [ ] Change Camera to use Quaternions
- [ ] Begin Implementation Unit and Integration Testing
- [ ] Re-implement UI (try to use original `asche`?)
