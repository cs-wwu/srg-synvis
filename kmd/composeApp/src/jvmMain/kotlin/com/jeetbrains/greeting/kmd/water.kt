package com.jeetbrains.greeting.kmd

import java.awt.image.BufferedImage
import java.io.File
import java.io.FileInputStream
import java.nio.ByteBuffer
import java.nio.ByteOrder
import javax.imageio.ImageIO

private const val LENGTH = 3601

/**
 * Represents a watermap grid of uint8 values.
 */
class Water {
    private val data = Array(LENGTH) { ByteArray(LENGTH) }

    /**
     * Populates the water array from raw byte data in big-endian format.
     */
    fun populate(bytes: ByteArray) {
        val buffer = ByteBuffer.wrap(bytes).order(ByteOrder.BIG_ENDIAN)
        
        for (i in 0 until LENGTH) {
            for (j in 0 until LENGTH) {
                data[i][j] = buffer.get()
            }
        }
    }

    /**
     * Gets the water value at the specified coordinates.
     */
    operator fun get(i: Int, j: Int): Byte = data[i][j]
}

/**
 * Reads a watermap (.swb) file and returns its contents as a byte array.
 */
fun watermapToBytes(waterFilepath: String): ByteArray {
    val file = File(waterFilepath)
    
    // Check file size
    val expectedFileSize = LENGTH * LENGTH
    require(file.length() == expectedFileSize.toLong()) {
        "Wrong file size, expected $expectedFileSize, received ${file.length()}"
    }

    // Read all bytes from the file
    return FileInputStream(file).use { it.readBytes() }
}

/**
 * Converts a watermap (.swb) file to a PNG image.
 * 
 * @param waterFilepath Path to the input .swb file
 * @param pngFilepath Path to the output .png file
 */
fun watermapToPng(waterFilepath: String, pngFilepath: String) {
    println("Creating $pngFilepath")

    // Read watermap data
    val data = watermapToBytes(waterFilepath)

    // Populate water matrix
    val water = Water()
    water.populate(data)

    // Create grayscale image
    val grayImage = BufferedImage(LENGTH, LENGTH, BufferedImage.TYPE_BYTE_GRAY)
    
    for (y in 0 until LENGTH) {
        for (x in 0 until LENGTH) {
            // Convert signed byte to unsigned int (0-255)
            val grayValue = water[y, x].toInt() and 0xFF
            // Pack into RGB (grayscale = same value for R, G, B)
            val rgb = (grayValue shl 16) or (grayValue shl 8) or grayValue
            grayImage.setRGB(x, y, rgb)
        }
    }

    // Write to PNG file
    val pngFile = File(pngFilepath)
    ImageIO.write(grayImage, "PNG", pngFile)
}

