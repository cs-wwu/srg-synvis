package processor

import com.google.protobuf.Message
import com.google.protobuf.ByteString
import java.io.BufferedReader
import java.io.File
import java.io.FileInputStream
import java.io.FileOutputStream
import java.net.HttpURLConnection
import java.net.URL
import java.nio.file.Paths
import java.time.LocalDate
import java.time.format.DateTimeFormatter
import java.time.temporal.ChronoUnit
import java.util.zip.ZipInputStream
import kotlin.math.floor

// Column offsets in DOF.DAT
private const val HEADER_START = 18
private const val HEADER_END = 26

private const val LAT_DEG_START = 35
private const val LAT_DEG_END = 37
private const val LAT_MIN_START = 38
private const val LAT_MIN_END = 40
private const val LAT_SEC_START = 41
private const val LAT_SEC_END = 46

private const val LON_DEG_START = 48
private const val LON_DEG_END = 51
private const val LON_MIN_START = 52
private const val LON_MIN_END = 54
private const val LON_SEC_START = 55
private const val LON_SEC_END = 60

private const val OBS_START = 62
private const val OBS_END = 81
private const val QTY_START = 81
private const val QTY_END = 82
private const val HGT_START = 84
private const val HGT_END = 89
private const val ELE_START = 90
private const val ELE_END = 95

data class FAAPoint(
    val lat: Double,
    val lon: Double,
    val height: Double,
    val type: String,
    val quantity: Double
)

data class ObsSquare(
    val latDeg: Int,
    val lonDeg: Int,
    val records: MutableList<FAAPoint> = mutableListOf()
)

fun generateSquareKey(lat: Double, lon: Double): String {
    val latDeg = floor(lat).toInt()
    val lonDeg = floor(lon).toInt()
    return "$latDeg,$lonDeg"
}

fun parseDOFFile(filePath: String, squareBuffers: MutableMap<String, ObsSquare>) {
    File(filePath).bufferedReader().useLines { lines ->
        lines.forEachIndexed { index, line ->
            val lineNum = index + 1

            // Skip header rows
            if (lineNum < 5) return@forEachIndexed

            val latDeg = line.substring(LAT_DEG_START, LAT_DEG_END).trim()
            val latMin = line.substring(LAT_MIN_START, LAT_MIN_END).trim()
            val latSec = line.substring(LAT_SEC_START, LAT_SEC_END).trim()
            val latHem = line.substring(LAT_SEC_END, LAT_SEC_END + 1).trim()

            val lonDeg = line.substring(LON_DEG_START, LON_DEG_END).trim()
            val lonMin = line.substring(LON_MIN_START, LON_MIN_END).trim()
            val lonSec = line.substring(LON_SEC_START, LON_SEC_END).trim()
            val lonHem = line.substring(LON_SEC_END, LON_SEC_END + 1).trim()

            val obsType = line.substring(OBS_START, OBS_END).trim()
            val quantity = line.substring(QTY_START, QTY_END).trim()
            val heightStr = line.substring(HGT_START, HGT_END).trim()
            val eleStr = line.substring(ELE_START, ELE_END).trim()

            // Skip line if any field fails to parse
            val latDegVal = latDeg.toIntOrNull() ?: return@forEachIndexed
            val latMinVal = latMin.toIntOrNull() ?: return@forEachIndexed
            val latSecVal = latSec.toDoubleOrNull() ?: return@forEachIndexed
            val lonDegVal = lonDeg.toIntOrNull() ?: return@forEachIndexed
            val lonMinVal = lonMin.toIntOrNull() ?: return@forEachIndexed
            val lonSecVal = lonSec.toDoubleOrNull() ?: return@forEachIndexed
            val quantityVal = quantity.toIntOrNull() ?: return@forEachIndexed
            val elevation = eleStr.toDoubleOrNull() ?: return@forEachIndexed
            val height = heightStr.toDoubleOrNull() ?: return@forEachIndexed

            var lat = latDegVal + latMinVal / 60.0 + latSecVal / 3600.0
            var lon = lonDegVal + lonMinVal / 60.0 + lonSecVal / 3600.0
            if (latHem == "S") lat = -lat
            if (lonHem == "W") lon = -lon
            val ele = elevation + height

            val obs = FAAPoint(
                lat = lat,
                lon = lon,
                height = ele,
                type = obsType,
                quantity = quantityVal.toDouble()
            )

            val key = generateSquareKey(lat, lon)
            squareBuffers.getOrPut(key) {
                ObsSquare(latDeg = floor(lat).toInt(), lonDeg = floor(lon).toInt())
            }.records.add(obs)
        }
    }
}

fun saveToProtobufs(squareBuffers: Map<String, ObsSquare>) {
    val outputDir = File("assets/chunks/obstructions/data")
    outputDir.mkdirs()

    for ((_, square) in squareBuffers) {
        val pbSquare = ObstructionSquare.newBuilder().apply {
            latDeg = square.latDeg
            lonDeg = square.lonDeg
            square.records.forEach { obs ->
                addObstructions(
                    FAAPoint.newBuilder().apply {
                        lat = obs.lat
                        lon = obs.lon
                        height = obs.height
                        type = obs.type
                        quantity = obs.quantity
                    }.build()
                )
            }
        }.build()

        val path = Paths.get(outputDir.path, "${coordToString(square.latDeg, square.lonDeg)}.pb")
        path.toFile().writeBytes(pbSquare.toByteArray())
    }
}

/**
 * Scrapes the FAA DOF webpage and returns the URL of the most recent DOF zip release.
 */
fun getDOFLink(): String {
    val faaUrl = "https://www.faa.gov/air_traffic/flight_info/aeronav/digital_products/dof"
    val connection = URL(faaUrl).openConnection() as HttpURLConnection
    val html = connection.inputStream.bufferedReader().readText()
    connection.disconnect()

    // Find all href values pointing to DOF zip files
    val regex = Regex("""href="([^"]*DOF_[^"]*\.zip)""")
    val matches = regex.findAll(html).map { it.groupValues[1] }.toList()

    // We want the second match (most recent release)
    return matches.getOrNull(1)
        ?: throw IllegalStateException("Could not find DOF zip link on FAA webpage")
}

/**
 * Downloads the latest DOF.DAT file from the FAA and saves it to finalPath.
 */
fun downloadFAADOF(finalPath: String) {
    val dofUrl = getDOFLink()

    val connection = URL(dofUrl).openConnection() as HttpURLConnection
    check(connection.responseCode == HttpURLConnection.HTTP_OK) {
        "Failed to download DOF zip, status: ${connection.responseCode}"
    }

    val initialPath = "./assets/DOF_Files/DOF.zip"
    File(initialPath).outputStream().use { out ->
        connection.inputStream.copyTo(out)
    }
    connection.disconnect()

    // Extract DOF.DAT from the zip
    ZipInputStream(FileInputStream(initialPath)).use { zis ->
        var entry = zis.nextEntry
        while (entry != null) {
            if (entry.name == "DOF.DAT") {
                FileOutputStream(finalPath).use { fos ->
                    zis.copyTo(fos)
                }
                zis.closeEntry()
                File(initialPath).delete()
                return
            }
            zis.closeEntry()
            entry = zis.nextEntry
        }
    }

    throw IllegalStateException("DOF.DAT not found inside downloaded zip")
}

/**
 * Reads the currency date from the header line of the DOF.DAT file.
 */
fun parseCurrencyDate(reader: BufferedReader): LocalDate {
    val header = reader.readLine()
        ?: throw IllegalStateException("DOF.DAT file is empty")
    val currencyString = header.substring(HEADER_START, HEADER_END).trim()
    val formatter = DateTimeFormatter.ofPattern("MM/dd/yy")
    return LocalDate.parse(currencyString, formatter)
}

/**
 * Top-level entry point: downloads (if needed) and processes FAA obstruction data
 * into coordinate-keyed protobuf files.
 */
fun generateObstructionData() {
    File("./assets/DOF_Files/").mkdirs()

    val filePath = "./assets/DOF_Files/DOF.DAT"
    val file = File(filePath)

    var daysSinceUpdate = Double.MAX_VALUE
    if (file.exists()) {
        val currencyDate = file.bufferedReader().use { parseCurrencyDate(it) }
        daysSinceUpdate = ChronoUnit.DAYS.between(currencyDate, LocalDate.now()).toDouble()
    }

    // DOF is updated every 56 days, released 2 days after — refresh threshold is 58 days
    if (!file.exists() || daysSinceUpdate >= 58) {
        downloadFAADOF(filePath)
    }

    val squareBuffers = mutableMapOf<String, ObsSquare>()
    parseDOFFile(filePath, squareBuffers)
    saveToProtobufs(squareBuffers)
}