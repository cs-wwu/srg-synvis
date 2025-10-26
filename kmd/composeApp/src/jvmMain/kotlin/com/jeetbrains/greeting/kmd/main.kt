package com.jeetbrains.greeting.kmd

import androidx.compose.ui.unit.DpSize
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.Window
import androidx.compose.ui.window.WindowPosition
import androidx.compose.ui.window.application
import androidx.compose.ui.window.rememberWindowState

fun main() = application {
    val state = rememberWindowState(
        size = DpSize(800.dp, 600.dp),
        position = WindowPosition(300.dp, 300.dp)
    )
    Window(
        title = "Synvis Installer",
        onCloseRequest = ::exitApplication,
        state = state,
        alwaysOnTop = true
    ) {
        App()
    }
}