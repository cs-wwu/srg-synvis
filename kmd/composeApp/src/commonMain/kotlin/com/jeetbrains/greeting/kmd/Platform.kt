package com.jeetbrains.greeting.kmd

interface Platform {
    val name: String
}

expect fun getPlatform(): Platform
