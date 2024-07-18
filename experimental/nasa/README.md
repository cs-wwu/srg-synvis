# NASA Lidar Data References
[NASA User Guide](https://lpdaac.usgs.gov/documents/592/NASADEM_User_Guide_V1.pdf?_ga=2.25675259.792210743.1712863665-544950785.1712863664)

[LASA Lidar Data](https://search.earthdata.nasa.gov/search?q=NASA%20Shuttle%20Radar%20Topography%20Mission%20Global%201%20arc%20second%20V003)

[Notes](https://docs.google.com/document/d/1eqsyck9LMbmbHWKJkRScVJnAsD-FjPVy4paW8LULAXU/edit?usp=sharing)

## Conversions
`1 Nautical Mile = 1 Arcminute`
`1 Arc Minute = 1852 meters` at the equator.
| Conversion Table   | **Nautical Mile (NM)** | **Degree (°)** | **Minute (‘)** | **Second (“)** | **Meter (m)** |
|--------------------|------------------------|----------------|----------------|----------------|---------------|
| **1 Degree (°)**   | 1/60                   | 1              | 60             | 360            | 1852\*60      |
| **1 Minute (')**   | 1/360                  | 1/60           | 1              | 60             | 1852          |
| **1 Second (")**   | 1/(60\*60\*60)         | 1/(60\*60)     | 1/60           | 1              | 1852/60       |
| **1 Meter (m)**    | 1/1852                 | 1/(1852\*60)   | 1/1852         | 1/(1852/60)    | 1             |

| Exact Conversions   | **Nautical Mile (NM)** | **Degree (°)** | **Minute (‘)** | **Second (“)** | **Meter (m)** |
|---------------------|------------------------|----------------|----------------|----------------|---------------|
| **1 Degree (°)**    | 0.01666...             | 1              | 60             | 360            | 111120        |
| **1 Minute (')**    | 0.0002777...           | 0.01666...     | 1              | 60             | 1852          |
| **1 Second (")**    | 0.00000462962          | 0.0002777...   | 0.01666...     | 1              | 30.8666...    |
| **1 Meter (m)**     | 0.0005399568           | 0.00000899928  | 0.0005399568   | 0.0323974082   | 1             |

## DEM File Parsing
NASA's files are stored in a zip file with a label like the following:
`NASADEM_HGT_n01w001.zip`

The layout of each of these zipfiles is as follows:
```
NASADEM_HGT_n01w001.zip
├── n01w001.hgt
├── n01w001.num
└── n01w001.swb
```

Each of the following files has 3601 rows by 3601 columns in big-endian format,
representing about one arcsecond by one arcsecond per tile.
- The `hgt` file shows meters relative to the EGM96 geoid. The data is 2-byte
signed integers.
- The `num` file has data about where each tiles data was collected. The data
is 1 byte unisigned integers. This is relatively useless to us.
- The `swb` file shows land/water per tile. The data is 1 byte unsigned integers.
0 for land, 255 for water.

Coordinates for the file (e.g. `n01e001.hgt`) represent a region from 
1°N to just below 2°N latitude, and 1°E to just below 2°E longitude. Note that
since West and South are techincally negative coordinates, this "seems" like
it's inverted when in the western or southern latitude and/or longitude.

We'll want to standardize our world coordinates as `(lat, lon, hgt)`.
Note that the data from NASA's tiles are about one arcsecond by one arcsecond.
Or 30.8667m by 30.8667m. The height however is in meters, so to convert meters
to arcseconds, we can multiply our height by `1/(1852/60)` to get arcseconds.
We can then use modulos operations to find the DMS (Degree Minute Second)
representation of our height.

NASA's data also doesn't account for latitude distortion. So we can think of the
data as being a rectangle from -180° to 180° longitude, and -56° to 60° latitude.


