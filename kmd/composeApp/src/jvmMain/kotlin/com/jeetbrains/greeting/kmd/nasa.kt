package api

import java.io.File
import java.io.FileOutputStream
import java.io.InputStream
import java.net.HttpURLConnection
import java.net.URL
import java.nio.file.Files
import java.nio.file.Paths
import java.util.zip.ZipInputStream

// NASA EarthData API constants
private const val NASA_BASE_URL =
    "https://data.lpdaac.earthdatacloud.nasa.gov/lp-prod-protected/NASADEM_HGT.001/"
private const val FILENAME_BASE = "NASADEM_HGT_"
private val HEIGHTMAP_PATTERN = Regex("[ns]\\d{2}[ew]\\d{3}")

data class Credentials(val token: String)

/**
 * Loads credentials from the EARTHDATA_TOKEN environment variable.
 */
fun loadCredentials(): Credentials {
    val token =
        System.getenv("EARTHDATA_TOKEN")
            ?: throw IllegalStateException("EARTHDATA_TOKEN environment variable not set")
    return Credentials(token)
}

/**
 * Opens an authenticated HTTP connection to the given URL.
 */
fun openConnection(
    url: String,
    creds: Credentials,
): HttpURLConnection {
    val connection = URL(url).openConnection() as HttpURLConnection
    connection.setRequestProperty("Authorization", "Bearer ${creds.token}")
    connection.connect()
    return connection
}

/**
 * Writes an InputStream to the given file path.
 */
fun downloadResponse(
    inputStream: InputStream,
    filePath: String,
) {
    File(filePath).outputStream().use { outputStream ->
        inputStream.copyTo(outputStream)
    }
}

/**
 * Converts a heightmap identifier (e.g. "n48w123") to its NASA download URL.
 * Validates the format before constructing the URL.
 */
fun heightmapToURL(heightmap: String): String {
    require(HEIGHTMAP_PATTERN.matches(heightmap)) {
        "Bad heightmap format: $heightmap"
    }
    // Full URL looks like:
    // https://data.lpdaac.earthdatacloud.nasa.gov/lp-prod-protected/NASADEM_HGT.001/NASADEM_HGT_n48w123/NASADEM_HGT_n48w123.zip
    val filename = FILENAME_BASE + heightmap
    return "$NASA_BASE_URL$filename/$filename.zip"
}

/**
 * Attempts to download the file at the given URL to the specified file path.
 */
fun attemptDownload(
    creds: Credentials,
    filePath: String,
    url: String,
) {
    val connection = openConnection(url, creds)
    try {
        check(connection.responseCode == HttpURLConnection.HTTP_OK) {
            "Failed to download file, status code: ${connection.responseCode}"
        }
        downloadResponse(connection.inputStream, filePath)
    } finally {
        connection.disconnect()
    }
}

/**
 * Downloads a list of heightmap tiles to the specified directory.
 */
fun downloadHeightmaps(
    heightmaps: List<String>,
    directoryPath: String,
) {
    val creds = loadCredentials()

    for (heightmap in heightmaps) {
        println("Downloading: $heightmap")
        val heightmapURL = heightmapToURL(heightmap)
        val filePath = "$directoryPath$heightmap.zip"
        println("at: $filePath")
        attemptDownload(creds, filePath, heightmapURL)
    }
}

/**
 * Unzips a zip archive to the specified target directory.
 */
fun unzip(
    zipFile: String,
    targetDir: String,
) {
    println("Unzipping $zipFile")

    ZipInputStream(File(zipFile).inputStream()).use { zis ->
        var entry = zis.nextEntry
        while (entry != null) {
            val extractPath = Paths.get(targetDir, entry.name)

            if (entry.isDirectory) {
                Files.createDirectories(extractPath)
            } else {
                Files.createDirectories(extractPath.parent)
                FileOutputStream(extractPath.toFile()).use { fos ->
                    zis.copyTo(fos)
                }
            }

            zis.closeEntry()
            entry = zis.nextEntry
        }
    }
}
