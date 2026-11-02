package com.jeetbrains.greeting.kmd

actual class DownloadManager {
    actual suspend fun downloadData(
        regions: Set<BoundingBox>,
        useNASA: Boolean,
        useOSM: Boolean,
        useFAA: Boolean,
        outputDir: String,
    ): Result<String> {
        return Result.failure(Exception("Download not supported on JS platform"))
    }

    actual suspend fun installScript(): Result<String> {
        return Result.failure(Exception("Install not supported on JS platform"))
    }
}
