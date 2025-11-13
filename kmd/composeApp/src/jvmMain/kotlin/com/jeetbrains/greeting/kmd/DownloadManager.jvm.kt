package com.jeetbrains.greeting.kmd

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlin.Exception

import java.io.File
import java.nio.file.Files
import java.nio.file.StandardCopyOption
import java.nio.file.Paths
import kotlin.io.path.absolutePathString

actual class DownloadManager {
    private fun getBundledBinaryPath(): String {
        val os = System.getProperty("os.name").lowercase()
        val arch = System.getProperty("os.arch").lowercase()

        val binaryName = when {
            os.contains("win") -> "download.exe"
            os.contains("linux") && (arch.contains("arm64") || arch.contains("aarch")) -> "download-linux-arm64"
            os.contains("linux") && arch.contains("armv7") -> "download-linux-armv7"
            os.contains("linux") -> "download-linux-x64"
            else -> throw IllegalStateException("Unsupported platform: $os ($arch)")
        }

        // Get resource as stream from bundled resources
        val resourcePath = "/binaries/$binaryName"
        val resourceStream = javaClass.getResourceAsStream(resourcePath)
            ?: throw IllegalStateException("Binary not found in resources: $resourcePath")

        // Extract to temp directory (resources are read-only in JAR)
        val tempDir = File(System.getProperty("java.io.tmpdir"), "synvis-binaries")
        tempDir.mkdirs()

        val tempBinary = File(tempDir, binaryName)

        // Only extract if not already present
        if (!tempBinary.exists()) {
            resourceStream.use { input ->
                Files.copy(input, tempBinary.toPath(), StandardCopyOption.REPLACE_EXISTING)
            }

            // Make executable on Unix systems
            if (!os.contains("win")) {
                tempBinary.setExecutable(true)
            }
        }

        return tempBinary.absolutePath
    }

    actual suspend fun downloadData(
        regions: Set<BoundingBox>,
        useNASA: Boolean,
        useOSM: Boolean,
        useFAA: Boolean,
        outputDir: String
    ): Result<String> = withContext(Dispatchers.IO) {
        try {
            val binaryPath = getBundledBinaryPath()

            // Build command
            val bboxArgs = mutableListOf<String>()
            regions.forEach { box ->
                bboxArgs.add("-bbox")
                bboxArgs.add("${box.North},${box.West},${box.South},${box.East}")
            }

        val projectRoot = File(System.getProperty("user.dir")).parentFile?.parentFile
            ?: throw IllegalStateException("Could not determine project root")

        val outputDirectory = File(projectRoot, "assets/maps")
        outputDirectory.mkdirs()

            val command = buildList {
                add(binaryPath)
                add("-out")
                add(outputDirectory.absolutePath)
                if (useNASA) add("-nasa")
                if (useOSM) add("-osm")
                if (useFAA) add("-faa")
                add("-v")
                addAll(bboxArgs)
            }

            println("Executing: ${command.joinToString(" ")}")

            val process = ProcessBuilder(command)
                .redirectErrorStream(true)
                .start()

            // Read output in real-time
            val output = StringBuilder()
            process.inputStream.bufferedReader().forEachLine { line ->
                println(line)  // Log to console
                output.appendLine(line)
            }

            val exitCode = process.waitFor()

            if (exitCode == 0) {
                Result.success(output.toString())
            } else {
                Result.failure(Exception("Download failed with exit code $exitCode:\n${output}"))
            }
        } catch (e: Exception) {
            Result.failure(e)
        }

    }

    private fun getBundledScript(scriptName: String): String {
        val resourcePath = "/scripts/$scriptName"
        val resourceStream = javaClass.getResourceAsStream(resourcePath)
            ?: throw IllegalStateException("Script not found in resources: $resourcePath")

        // Extract to temp directory
        val tempDir = File(System.getProperty("java.io.tmpdir"), "synvis-scripts")
        tempDir.mkdirs()

        val tempScript = File(tempDir, scriptName)

        // Extract script
        resourceStream.use { input ->
            Files.copy(input, tempScript.toPath(), StandardCopyOption.REPLACE_EXISTING)
        }

        // Make executable on Unix systems
        val os = System.getProperty("os.name").lowercase()
        if (!os.contains("win")) {
            tempScript.setExecutable(true)
        }

        return tempScript.absolutePath
    }

    actual suspend fun installScript(): Result<String> = withContext(Dispatchers.IO) {
        try {
            val os = System.getProperty("os.name").lowercase()

            val (scriptName, command) = when {
                os.contains("win") -> {
                    val scriptPath = getBundledScript("install.ps1")
                    Pair("install.ps1", listOf(
                        "powershell",
                        "-ExecutionPolicy", "Bypass",
                        "-File", scriptPath
                    ))
                }
                else -> {
                    val scriptPath = getBundledScript("install.sh")
                    Pair("install.sh", when {
                        commandExists("pkexec") -> listOf("pkexec", "bash", scriptPath)
                        commandExists("gksudo") -> listOf("gksudo", "bash", scriptPath)
                        else -> listOf("sudo", "bash", scriptPath)
                    })
                }
            }

            println("Executing: ${command.joinToString(" ")}")

            val process = ProcessBuilder(command)
                .redirectErrorStream(true)
                .start()

            val output = StringBuilder()
            process.inputStream.bufferedReader().forEachLine { line ->
                println(line)
                output.appendLine(line)
            }

            val exitCode = process.waitFor()

            if (exitCode == 0) {
                Result.success(output.toString())
            } else {
                Result.failure(Exception("Install failed with exit code $exitCode:\n${output}"))
            }
        } catch (e: Exception) {
            Result.failure(e)
        }
    }

    private fun commandExists(command: String): Boolean {
        return try {
            val process = ProcessBuilder("which", command).start()
            process.waitFor() == 0
        } catch (e: Exception) {
            false
        }
    }
}