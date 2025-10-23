package com.jeetbrains.greeting.demo1

interface Platform {
    val name: String
}

expect fun getPlatform(): Platform