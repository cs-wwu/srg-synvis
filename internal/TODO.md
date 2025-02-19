# Interal TODOs

These are the internal system TODOs.

## TODO

### Cooper

Refactor:

```shell
srg-synvis/
├── assets/                # Static assets like shaders, textures, and maps
├── cmd/                   # Application entry points
│   ├── download/          # Data download and processing tool
│   └── svs/               # Main SVS application
├── internal/              # Internal packages
│   ├── platform/          # Platform-specific code
│   │   ├── types/         # Fundamental platform types
│   │   ├── gpu/           # GPU interface definitions
│   │   └── vulkan/        # Vulkan implementation
│   ├── render/            # Rendering system
│   │   ├── pipeline/      # Render pipeline management
│   │   ├── mesh/          # Mesh generation and management
│   │   └── scene/         # Scene management
│   ├── terrain/           # Terrain system
│   │   ├── generator/     # Terrain generation
│   │   ├── chunk/         # Chunk management
│   │   └── cache/         # Terrain data caching
│   ├── engine/            # Core engine functionality
│   │   ├── camera/        # Camera management
│   │   ├── input/         # Input handling
│   │   └── config/        # Engine configuration
│   └── data/              # Data processing
│       ├── heightmap/     # Height data processing
│       ├── watermap/      # Water data processing
│       └── roadmap/       # Road data processing
├── pkg/                   # Public packages
│   ├── api/               # External API interfaces
│   └── wmm/               # World Magnetic Model
└── test/                  # Testing directory
    ├── mocks/             # Mock implementations
    ├── unit/              # Unit tests
    └── integration/       # Integration tests
```

- [ ] Simplify chunk logic
- [ ] Add capacity to render 4 maps at once
  - [ ] Find max memory based on LODs and distance
  - [ ] Increase memory capacity
  - [ ] If map do not exist, load water square (stitch to corners)
  - [ ] Add LOD stitching
  - [ ] Add dynamic map loading
- [ ] Add dynamic texture loading per map (kyle/roads)
  - [ ] Fix texture striping
- [ ] Simplify frustum logic
  - [ ] Move frustum culling from app to TerrainManager
  - [ ] Test performance difference
- [ ] Change Camera to use Quaternions (sensors?)

### Anyone

- [ ] Move `asche` changes to a custom pullable public branch
  - [ ] UI Application in `asche`
