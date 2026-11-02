package com.jeetbrains.greeting.kmd

import java.awt.image.BufferedImage
import java.awt.image.DataBufferUShort
import java.io.File
import java.io.FileInputStream
import java.nio.ByteBuffer
import java.nio.ByteOrder
import javax.imageio.IIOImage
import javax.imageio.ImageIO
import javax.imageio.ImageWriteParam

private const val HEIGHTMAP_LENGTH = 3601

/**
 * Represents elevation data as a grid of int16 values.
 */
class Elevations {
    private val data = Array(HEIGHTMAP_LENGTH) { ShortArray(HEIGHTMAP_LENGTH) }

    /**
     * Populates the elevations matrix from raw byte data in big-endian format.
     */
    fun populate(bytes: ByteArray) {
        val buffer = ByteBuffer.wrap(bytes).order(ByteOrder.BIG_ENDIAN)
        
        for (i in 0 until HEIGHTMAP_LENGTH) {
            for (j in 0 until HEIGHTMAP_LENGTH) {
                data[i][j] = buffer.short
            }
        }
    }

    /**
     * Gets the elevation value at the specified coordinates.
     */
    operator fun get(i: Int, j: Int): Short = data[i][j]
}

/**
 * Represents normalized elevation pixels as uint16 values (0-65535).
 */
class ElevationPixels {
    private val data = Array(HEIGHTMAP_LENGTH) { IntArray(HEIGHTMAP_LENGTH) }

    /**
     * Populates the pixel matrix by normalizing elevation values.
     * Converts from int16 range (-32768 to 32767) to uint16 range (0 to 65535).
     * 
     * This matches Go's conversion: uint16(int16_value) + 32768
     * The conversion to unsigned happens via bit reinterpretation.
     */
    fun populate(elevations: Elevations) {
        val minInt16 = 32768 // abs(Short.MIN_VALUE)
        
        for (i in 0 until HEIGHTMAP_LENGTH) {
            for (j in 0 until HEIGHTMAP_LENGTH) {
                // Convert signed short to unsigned int (0-65535) by masking
                val asUnsigned = elevations[i, j].toInt() and 0xFFFF
                // Add offset (this is what Go does: uint16(int16) + minInt16)
                val normalized = (asUnsigned + minInt16) and 0xFFFF
                data[i][j] = normalized
            }
        }
    }

    /**
     * Gets the pixel value at the specified coordinates.
     */
    operator fun get(i: Int, j: Int): Int = data[i][j]
}

/**
 * Reads a heightmap (.hgt) file and returns its contents as a byte array.
 * Expected format: NASADEM_HGT_[NS]DD[EW]DDD.hgt
 */
fun heightmapToBytes(hgtFilepath: String): ByteArray {
    val file = File(hgtFilepath)
    
    // Check file size: 3601 * 3601 * 2 bytes (int16)
    val expectedFileSize = HEIGHTMAP_LENGTH * HEIGHTMAP_LENGTH * 2L
    require(file.length() == expectedFileSize) {
        "Wrong file size, expected $expectedFileSize, received ${file.length()}"
    }

    // Read all bytes from the file
    return FileInputStream(file).use { it.readBytes() }
}

/**
 * Converts a heightmap (.hgt) file to a 16-bit grayscale PNG image.
 * 
 * @param heightmapFilepath Path to the input .hgt file
 * @param pngFilepath Path to the output .png file
 */
fun heightmapToPng(heightmapFilepath: String, pngFilepath: String) {
    println("Creating $pngFilepath")

    // Read heightmap data
    val data = heightmapToBytes(heightmapFilepath)

    // Populate elevations matrix
    val elevations = Elevations()
    elevations.populate(data)

    // Normalize elevations to uint16 pixels (no negatives in PNG)
    val pixels = ElevationPixels()
    pixels.populate(elevations)

    // Create 16-bit grayscale image
    val grayImage = BufferedImage(
        HEIGHTMAP_LENGTH,
        HEIGHTMAP_LENGTH,
        BufferedImage.TYPE_USHORT_GRAY
    )
    
    // Direct access to the underlying short array for efficiency
    val raster = grayImage.raster
    val dataBuffer = raster.dataBuffer as DataBufferUShort
    val pixelData = dataBuffer.data
    
    // Copy pixels directly to the image buffer
    for (y in 0 until HEIGHTMAP_LENGTH) {
        for (x in 0 until HEIGHTMAP_LENGTH) {
            val index = y * HEIGHTMAP_LENGTH + x
            pixelData[index] = pixels[y, x].toShort()
        }
    }

    // Write to PNG file with compression
    val pngFile = File(pngFilepath)
    val writer = ImageIO.getImageWritersByFormatName("png").next()
    
    ImageIO.createImageOutputStream(pngFile).use { output ->
        writer.output = output
        
        val writeParam = writer.defaultWriteParam
        if (writeParam.canWriteCompressed()) {
            writeParam.compressionMode = ImageWriteParam.MODE_EXPLICIT
            writeParam.compressionQuality = 0.9f // Default compression
        }
        
        writer.write(null, IIOImage(grayImage, null, null), writeParam)
        writer.dispose()
    }
}


