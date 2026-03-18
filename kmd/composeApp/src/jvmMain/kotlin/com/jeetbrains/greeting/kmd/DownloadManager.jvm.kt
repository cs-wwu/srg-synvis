package com.jeetbrains.greeting.kmd

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

// Extension function for BoundingBox -> heightmap tiles
fun BoundingBox.toHeightmapTiles(): List<String> {
    val tiles = mutableListOf<String>()

    val latRange = South..North
    val lonRange = West..East

    for (lat in latRange) {
        for (lon in lonRange) {
            val latStr = if (lat >= 0) "n%02d".format(lat) else "s%02d".format(-lat)
            val lonStr = if (lon >= 0) "e%03d".format(lon) else "w%03d".format(-lon)
            tiles.add("$latStr$lonStr")
        }
    }

    return tiles
}

// Extension function for BoundingBox -> coordinate squares
fun BoundingBox.toCoordinateSquares(): List<List<Int>> {
    val squares = mutableListOf<List<Int>>()

    val latRange = South..North
    val lonRange = West..East

    for (lat in latRange) {
        for (lon in lonRange) {
            squares.add(listOf(lat, lon))
        }
    }

    return squares
}

actual class DownloadManager {
    actual suspend fun downloadData(
        regions: Set<BoundingBox>,
        useNASA: Boolean,
        useOSM: Boolean,
        useFAA: Boolean,
        outputDir: String,
    ): Result<String> =
        withContext(Dispatchers.IO) {
            try {
                val output = StringBuilder()

                if (useNASA) {
                    println("trying nasa")
                    val heightmaps = regions.flatMap{ it.toHeightmapTiles() }.distinct()

                    downloadHeightmaps(heightmaps, outputDir)
                    processHeightmaps(heightmaps, outputDir)
                    output.appendLine("NASA: downloaded ${heightmaps.size} heightmap tile(s)")
                    
                    generateRoadProtobufs(heightmaps)
                }

                if (useFAA) {
                    println("trying faa")
                    generateObstructionData()
                    output.appendLine("FAA: obstruction data generated")
                }

                if (useOSM) {
                    println("trying osm")
                    val coordinateSquares =
                        regions
                            .flatMap { it.toCoordinateSquares() }
                            .distinct()

                    generateAirData(coordinateSquares, OSM_API_URL)
                    output.appendLine("OSM: aeroway data generated for ${coordinateSquares.size} square(s)")
                }

                Result.success(output.toString())
            } catch (e: Exception) {
                Result.failure(e)
            }
        }

    // Remove installScript() if the expect declaration still exists in commonMain
    actual suspend fun installScript(): Result<String> {
        return Result.failure(Exception("installScript is deprecated - not needed for Kotlin implementation"))
    }

    companion object {
        private const val OSM_API_URL = "https://overpass-api.de/api/interpreter"
    }
}
