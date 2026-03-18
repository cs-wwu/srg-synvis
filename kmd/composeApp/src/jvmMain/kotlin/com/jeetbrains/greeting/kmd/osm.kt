// NOTE: could not find the Go code version of this file actually being used??

package com.jeetbrains.greeting.kmd

import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import java.net.HttpURLConnection
import java.net.URL
import java.net.URLEncoder

data class OSMPoint(
    val lat: Double,
    val lon: Double,
    val height: Double,
)


@Serializable
data class OverpassResponse(
    val elements: List<Element> = emptyList(),
)

@Serializable
data class Element(
    val type: String = "",
    val id: Long = 0,
    val lat: Double = 0.0,
    val lon: Double = 0.0,
    val nodes: List<Long> = emptyList(),
    val tags: Map<String, String> = emptyMap(),
)

private val json = Json { ignoreUnknownKeys = true }

/**
 * Makes a POST request to the Overpass API for aeroway data within a 1-degree
 * coordinate square defined by the first two values in [map] (lat, lon).
 */
fun makeAirApiRequest(
    apiUrl: String,
    map: List<Int>,
): OverpassResponse {
    val lat = map[0]
    val lon = map[1]
    val bounds = "($lat,$lon,${lat + 1},${lon + 1});"

    val query =
        """
        [out:json];
        (
            way["aeroway"="hangar"]$bounds
            way["aeroway"="terminal"]$bounds
            way["aeroway"="tower"]$bounds
        );
        (._;>;);
        out body;
        """.trimIndent()

    val encodedData = "data=" + URLEncoder.encode(query, "UTF-8")

    val connection = URL(apiUrl).openConnection() as HttpURLConnection
    connection.requestMethod = "POST"
    connection.doOutput = true
    connection.setRequestProperty("Content-Type", "application/x-www-form-urlencoded")

    connection.outputStream.use { it.write(encodedData.toByteArray()) }

    check(connection.responseCode == HttpURLConnection.HTTP_OK) {
        "POST to $apiUrl failed with status: ${connection.responseCode}"
    }

    val body = connection.inputStream.bufferedReader().use { it.readText() }
    connection.disconnect()

    return json.decodeFromString<OverpassResponse>(body)
}

/**
 * Converts a list of way [Element]s into lists of [OSMPoint]s by resolving
 * each way's node IDs against the provided [nodes] map.
 */
fun storeAirData(
    ways: List<Element>,
    nodes: Map<Long, Element>,
    airType: String,
): List<List<OSMPoint>> {
    val airBuf = mutableListOf<List<OSMPoint>>()

    for (way in ways) {
        val wayPoints = mutableListOf<OSMPoint>()

        for (nodeId in way.nodes) {
            val node = nodes[nodeId] ?: continue

            var height = 0.0
            val eleTag = node.tags["ele"]
            if (eleTag != null) {
                height = eleTag.toDoubleOrNull() ?: 0.0
                val point = OSMPoint(lat = node.lat, lon = node.lon, height = height)
                wayPoints.add(point)
            }
        }

        if (wayPoints.isNotEmpty()) {
            airBuf.add(wayPoints)
        }
    }

    return airBuf
}

/**
 * Fetches and processes aeroway OSM data for each coordinate square in [maps].
 */
fun generateAirData(
    maps: List<List<Int>>,
    apiUrl: String,
) {
    require(maps.isNotEmpty()) { "No maps specified" }

    for (map in maps) {
        val osmData = makeAirApiRequest(apiUrl, map)

        val hangars = mutableListOf<Element>()
        val terminals = mutableListOf<Element>()
        val towers = mutableListOf<Element>()
        val nodes = mutableMapOf<Long, Element>()

        for (element in osmData.elements) {
            when (element.type) {
                "node" -> nodes[element.id] = element
                "way" ->
                    when (element.tags["aeroway"]) {
                        "hangar" -> hangars.add(element)
                        "terminal" -> terminals.add(element)
                        "tower" -> towers.add(element)
                    }
            }
        }

        val hangarBuf = storeAirData(hangars, nodes, "hangar")
        val termBuf = storeAirData(terminals, nodes, "terminal")
        val towerBuf = storeAirData(towers, nodes, "tower")

        println("$hangarBuf $termBuf $towerBuf")
    }
}

// JSON deserialization — Go uses encoding/json with struct tags. The Kotlin version uses kotlinx.serialization with @Serializable data classes, which is the idiomatic choice for Kotlin (especially if you're on a multiplatform or Android project). Need the org.jetbrains.kotlinx:kotlinx-serialization-json dependency and the serialization Gradle plugin.
// OverpassResponse and Element — The Go code references these types but defines them elsewhere (your processor package). There are placeholder definitions here based on usage — the fields type, id, lat, lon, nodes, and tags are all implied by the Go code.
