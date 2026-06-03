# SRG SynVis

Synthetic Vision System (SVS) for Systems Research Group at WWU

Our Synthetic Vision System (SVS) was created with the goal to develop an open-source SVS that runs on low-cost hardware, prioritizing memory safety, modern tools, and overall performance. Currently, all SVS’s are proprietary or subscription-based, with costs ranging in the thousands. In contrast, a complete setup running our SVS on a Raspberry Pi 5 can be built for under $300, a more realistic option for private pilots. Synthetic vision dramatically enhances pilot safety, both in terms of general awareness and terrain-related crash prevention. By offering a secure, customizable, and affordable open-source alternative, our system aims to make this valuable technology accessible to more pilots. To our knowledge, this is the first fully open-source SVS designed to run efficiently on accessible consumer hardware.

We decided to use Go for its safety guarantees, paired with the Vulkan graphics API on a Raspberry Pi 5. Vulkan was selected for its low-level control over rendering, despite increased verbosity. This choice enabled us to build a custom graphics engine and rendering pipeline with integrated shading. To support accurate mesh rendering, we implemented a coordinate system and a virtual camera.

Heightmaps and water data were sourced from publicly available NASA datasets, with each coordinate square individually downloadable. For real-time location and orientation, we used the Adafruit Ultimate GPS and the Raspberry 5 Sense Hat. The GPS provides latitude, longitude, and altitude, while the Sense Hat’s accelerometer and gyroscope supply orientation data. We manually calibrated this input using the World Magnetic Model. A debug mode allows users to freely navigate the virtual terrain via keyboard input, while the full system uses real-time sensor fusion for accurate, meter-level positioning anywhere on Earth.

Roadways, obstructions, and airspaces were also implemented. Roads are downloaded from the open-source OpenStreetMap (OSM) project, while obstructions and airspaces are sourced from the FAA. To minimize storage overhead, roads and obstructions compacted into coordinate squares and stored in protobufs to be unmarshaled in the rendering pipeline. Airspaces are in GeoJSON format, and similarly grouped into coordinate squares.

UI features including a compass, altimeter, speedometer, and attitude indicator were implemented using a custom Go wrapper to interact with Vulkan. 

To improve performance, we implemented level-of-detail (LOD) rendering and a quad-tree chunk management system. These strategies allow the engine to dynamically update terrain as the user moves, minimizing lag. LOD techniques significantly reduce memory usage, making the project feasible on the Raspberry Pi’s limited VRAM.

Through extensive optimization, our system achieves a stable 60 frames per second on the Raspberry Pi 5. Users can choose between keyboard navigation or real-time control via live sensor fusion. Terrain is procedurally generated around the user’s position, enabling smooth continuous exploration.

In the future, we plan to continue working on the current build for further optimizations and features. We also plan to port to mobile, increasing SVS accessibility further.

## Running the Application

```bash
go run ./cmd/svs
```

### Command Arguments

```bash
go run ./cmd/svs
  -gps
     Enable GPS
  -hat
     Enable Hat
  -monitor int
     Monitor selection (0 = windowed, 1 = primary monitor, etc.)
  -pos string
     Starting Camera Position (lat,lon,height) where height is in meters (default "48.5,-123.5,2600")
  -dir string
     Starting Camera Direction (x,y,z) - normalized automatically (default "0,0,-1")
   -fov float
      Field of view (degrees) (default 60)
  -render-distance float
     Maximum render distance in world units (default 5000)
  -disable-warnings
     Disable terrain height warnings (red coloring)
```

### Download Tool

The Download api must have at least one "Download" selection (nasa/faa/osm)

```bash
go run ./cmd/download
  -nasa             Download NASA elevation data
  -faa              Download FAA airport/navigation data
  -osm              Download OpenStreetMap data
  -v                Verbose output
  -bbox   <string>  Bounding box coordinates (e.g., '49,-125,45,-118') in format 'n, e, s, w'
  -out    <string>  Output directory for downloaded files (default "assets/maps/")
  -preset <string>  Use preset region: [bellingham/washington/...]
  -region <string>  Comma-separated list of region codes (e.g., 'n48w123,n48w122')
```

## Installation

To install all necessary libraries and the latest `Go` version, run:

```bash
sudo ./install.sh
```

### Manual Installation Steps

1. Download the latest version of Go

    1. Through apt

        ```bash
        sudo apt -y install golang-go
        ```

    2. Or manually

        ```bash
        cd ~
        wget https://go.dev/dl/go1.24.2.linux-arm64.tar.gz
        sudo tar -xzf go1.24.2.linux-arm64.tar.gz
        rm go1.24.2.linux-arm64.tar.gz
        ```

2. Add the following to the bottom of your `~/.profile`

    ```bash
    # Set GOPATH to the default user workspace if it is not already set
    if [ -z "$GOPATH" ]; then
        export GOPATH=$HOME/go
    fi
    # Optionally, add GOPATH/bin to PATH for easier access to installed Go binaries
    if [ -d "$GOPATH/bin" ]; then
        export PATH=$PATH:$GOPATH/bin
    fi
    ```

3. Run `go version` to test successful install

4. Install Vulkan Libraries

    ```bash
    sudo apt -y install mesa-vulkan-drivers vulkan-tools libvulkan-dev vulkan-validationlayers-dev spirv-tools libglfw3-dev libglm-dev libxi-dev libxxf86vm-dev libxcursor-dev libxinerama-dev libxrandr-dev
    ```

    You may run into issues with the `vulkan-validationlayers-dev` package. If so, use the following package instead: `vulkan-utility-libraries-dev`.

5. Run `vkcube` and `go run cmd/svs/main.go` to test installation

## Downloading data

If you need to download more data than what's checked in, the `download`
command should be used. We currently source data from two places:

- NASA: This is the heightmap data used to render the terrain.
- OSM: This provides the street and obstruction data.

The downloader script (at `./cmd/download`) has a few common flags:

- `-bbox`: Set a bounding box in lat/lon coordinates to download.
- `-region`: A comma-separated list of 1-degree coordinate squares to download.
- `-preset`: Set a preset region (currently `washington` or `bellingham`)

### OSM Data

OSM data does not require authentication, so you can simply run the downloader
with a region (note that OSM has strict rate limits, so this will take a while):
```
go run ./cmd/download -osm -preset washington
```

### NASA Data

To download NASA data, create an EarthData account at `https://urs.earthdata.nasa.gov`.
Once you have an account, create a token in the `Generate Token` page. Store
that token in the `EARTHDATA_TOKEN` environment variable, and then run the
downloader:
```
go run ./cmd/download -nasa -preset washington
```

#### Airspace Data
Downloads controlled airspace boundaries (Class B, C, D, etc.) from the FAA's
Airspace GeoJSON API. You can filter by state and airspace class:
```
go run ./cmd/download -faa -air -faaStates "WA,OR,CA" -faaClasses "B,C,D"
```
If no filters are specified, all Class B, C, and D airspace nationwide will be downloaded.

#### Obstruction Data
Downloads the FAA's Digital Obstacle File (DOF), which contains towers, buildings,
and other obstructions to aviation:
```
go run ./cmd/download -faa -obs
```
The DOF data is updated every 56 days and covers the entire United States.

## Guidelines

- Check in code to the repo, including experimental investigations
- Don't check in gigantic data sets. It's ok to check in small data sets for experimentation
- There will be further guidelines for testing and continuous integration when we get to that point

## Founding Members

Ada Kolze, Cooper Morgan, Kyle Peterson, See-Mong Tan

## License

MIT
