package com.jeetbrains.greeting.kmd

expect class DownloadManager() {
    suspend fun downloadData(
        regions: Set<BoundingBox>,
        useNASA: Boolean = true,
        useOSM: Boolean = true,
        useFAA: Boolean = false,
        outputDir: String = "../../assets/maps/",
        username: String,
        password: String,
    ): Result<String>

    suspend fun installScript(): Result<String>
}