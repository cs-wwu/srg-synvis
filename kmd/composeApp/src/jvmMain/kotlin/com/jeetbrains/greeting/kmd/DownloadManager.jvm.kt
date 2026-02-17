package com.jeetbrains.greeting.kmd

import api.downloadHeightmaps
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import processor.generateAirData
import processor.generateObstructionData

fun BoundingBox.toHeightmapTiles(): List<String> {
    val tiles = mutableListOf<String>()

    val latRange = floor(South).toInt()..floor(North).toInt()
    val lonRange = floor(West).toInt()..floor(East).toInt()

    for (lat in latRange) {
        for (lon in lonRange) {
            val latStr = if (lat >= 0) "n%02d".format(lat) else "s%02d".format(-lat)
            val lonStr = if (lon >= 0) "e%03d".format(lon) else "w%03d".format(-lon)
            tiles.add("$latStr$lonStr")
        }
    }

    return tiles
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
                    val heightmaps = regions
                        .flatMap { it.toHeightmapTiles() }
                        .distinct()

                    downloadHeightmaps(heightmaps, outputDir)
                    output.appendLine("NASA: downloaded ${heightmaps.size} heightmap tile(s)")
                }

                if (useFAA) {
                    generateObstructionData()
                    output.appendLine("FAA: obstruction data generated")
                }

                if (useOSM) {
                    val coordinateSquares = regions
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

    companion object {
        private const val OSM_API_URL = "https://overpass-api.de/api/interpreter"
    }
}