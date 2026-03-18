package com.jeetbrains.greeting.kmd

import RoadOuterClass
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import java.io.File
import java.io.FileOutputStream
import java.net.HttpURLConnection
import java.net.URL
import java.net.URLEncoder
import kotlin.math.pow

private const val IMG_LENGTH = 3600
private const val SCALE = 1.0
private const val OSM_ROAD_URL = "http://overpass-api.de/api/interpreter"

// Definition of each road type:
// https://wiki.openstreetmap.org/wiki/International_highway_classification_equivalence
val roadWidths = mapOf(
    // Highway types
    "motorway" to 8.0,
    "trunk" to 6.0,
    "primary" to 4.0,
    "secondary" to 2.0,
    "tertiary" to 1.0,
    // Railway types
    "rail" to 3.0,
    "light_rail" to 2.0,
    "tram" to 1.0,
    // Aeroway types
    "runway" to 6.0,
    "airstrip" to 5.0,
    "taxiway" to 2.0
)

class RateLimitedException : Exception("Rate limited by OSM")

@Serializable
data class RoadElement(
    val id: Long = 0,
    val type: String = "",
    val tags: Map<String, String> = emptyMap(),
    val lat: Double = 0.0,
    val lon: Double = 0.0,
    val nodes: List<Long> = emptyList()
)

@Serializable
data class RoadOverpassResponse(
    val elements: List<RoadElement> = emptyList()
)

private val json = Json { ignoreUnknownKeys = true }

/**
 * Makes an API request to Overpass API for road/railway/aeroway data.
 */
fun makeRoadApiRequest(lat: Int, lon: Int): RoadOverpassResponse {
    val bounds = "($lat,$lon,${lat + 1},${lon + 1});"

    val query = """
        [out:json];
        (
            way["highway"="motorway"]$bounds
            way["highway"="trunk"]$bounds
            way["highway"="primary"]$bounds
            way["highway"="secondary"]$bounds
            way["highway"="tertiary"]$bounds
            way["railway"="rail"]$bounds
            way["railway"="light_rail"]$bounds
            way["railway"="tram"]$bounds
            way["aeroway"="runway"]$bounds
            way["aeroway"="airstrip"]$bounds
            way["aeroway"="taxiway"]$bounds
        );
        (._;>;);
        out body;
    """.trimIndent()

    val encodedData = "data=" + URLEncoder.encode(query, "UTF-8")

    val connection = URL(OSM_ROAD_URL).openConnection() as HttpURLConnection
    connection.requestMethod = "POST"
    connection.doOutput = true
    connection.setRequestProperty("Content-Type", "application/x-www-form-urlencoded")

    connection.outputStream.use { it.write(encodedData.toByteArray()) }

    when (connection.responseCode) {
        429 -> throw RateLimitedException()
        HttpURLConnection.HTTP_OK -> {
            val body = connection.inputStream.bufferedReader().use { it.readText() }
            connection.disconnect()
            return json.decodeFromString<RoadOverpassResponse>(body)
        }
        else -> {
            connection.disconnect()
            throw Exception("Received non-200 response: ${connection.responseCode}")
        }
    }
}

/**
 * Converts heightmap string (e.g., "n48w123") to coordinate pair [lat, lon].
 */
fun parseHeightmapToCoords(heightmap: String): List<Int> {
    require(heightmap.length == 7) { "Invalid heightmap format: $heightmap" }

    val latDir = heightmap[0]
    val lonDir = heightmap[3]

    var lat = heightmap.substring(1, 3).toInt()
    var lon = heightmap.substring(4, 7).toInt()

    if (latDir == 's') lat = -lat
    if (lonDir == 'w') lon = -lon

    return listOf(lat, lon)
}

/**
 * Generates road protobuf files for the given heightmap tiles.
 */
fun generateRoadProtobufs(heightmaps: List<String>, outputName: String) {
    val outputDir = File(outputName)
    outputDir.mkdirs()

    // OSM takes a couple seconds per request, process sequentially to avoid rate limits
    // See https://dev.overpass-api.de/overpass-doc/en/preface/commons.html
    for (heightmap in heightmaps) {

        // Check if file exists
        val roadName = "$outputName$heightmap.pb"
        val roadfile = File(roadName)
        if (roadfile.exists()) {
            continue
        }

        try {
            val coords = parseHeightmapToCoords(heightmap)
            generateRoadSquare(outputDir, coords[0], coords[1])
        } catch (e: Exception) {
            println("Warning: skipping heightmap $heightmap: ${e.message}")
            continue
        }
    }
}

/**
 * Generates a single road square protobuf for the given coordinates.
 */
private fun generateRoadSquare(outputDir: File, lat: Int, lon: Int) {
    var attempts = 0
    var delay = 1.0
    var osmData: RoadOverpassResponse? = null

    while (attempts < 10) {
        try {
            println("Posting request to OSM at $lat,$lon...")
            osmData = makeRoadApiRequest(lat, lon)
            break
        } catch (e: RateLimitedException) {
            println("Rate limit from OSM, retrying in ${delay}s")
            Thread.sleep((delay * 1000).toLong())
            delay *= 2.0
            attempts++
        } catch (e: Exception) {
            throw e
        }
    }

    requireNotNull(osmData) { "Failed to fetch OSM data after $attempts attempts" }

    val nodes = mutableMapOf<Long, RoadElement>()
    val square = RoadOuterClass.RoadSquare.newBuilder()
        .setLatDeg(lat)
        .setLonDeg(lon)

    for (element in osmData.elements) {
        if (element.type == "node") {
            nodes[element.id] = element
            continue
        }

        if (element.type == "way") {
            val name = try {
                elementName(element)
            } catch (e: Exception) {
                println("Warning: invalid way ${element.id}: ${e.message}")
                continue
            }

            val points = mutableListOf<RoadOuterClass.Point>()
            for (nodeId in element.nodes) {
                val node = nodes[nodeId]
                if (node == null) {
                    println("Warning: node $nodeId not found for way ${element.id}")
                    continue
                }

                points.add(
                    RoadOuterClass.Point.newBuilder()
                        .setLat(node.lat)
                        .setLon(node.lon)
                        .build()
                )
            }

            square.addRoads(
                RoadOuterClass.Road.newBuilder()
                    .setType(name)
                    .addAllPoints(points)
                    .build()
            )
        }
    }

    val data = square.build().toByteArray()
    val path = File(outputDir, "${coordToString(lat, lon)}.pb")
    FileOutputStream(path).use { it.write(data) }
}

/**
 * Extracts the road/railway/aeroway type name from an OSM element.
 */
private fun elementName(element: RoadElement): String {
    element.tags["highway"]?.let { return it }
    element.tags["aeroway"]?.let { return it }
    element.tags["railway"]?.let { return it }
    throw Exception("Element ${element.id} has no valid name")
}

