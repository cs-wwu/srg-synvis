package processor

import com.google.protobuf.proto // replace with your actual proto import
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