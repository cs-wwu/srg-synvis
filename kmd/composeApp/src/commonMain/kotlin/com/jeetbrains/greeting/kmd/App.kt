package com.jeetbrains.greeting.kmd

// import androidx.compose.animation.AnimatedVisibility
// import androidx.compose.foundation.Image
// import androidx.compose.foundation.background

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeContentPadding
import androidx.compose.foundation.layout.width
import androidx.compose.material3.Button
import androidx.compose.material3.Checkbox
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.launch
import org.jetbrains.compose.ui.tooling.preview.Preview

data class BoundingBox(val North: Int, val South: Int, val East: Int, val West: Int)

data class Region(val State: String, val box: BoundingBox)

fun USregions() =
    listOf(
        Region("AL", BoundingBox(36, 30, -84, -89)),
        Region("AK", BoundingBox(72, 51, 179, -180)),
        Region("AS", BoundingBox(-12, -14, -168, -172)),
        Region("AZ", BoundingBox(38, 31, -109, -115)),
        Region("AR", BoundingBox(37, 33, -89, 95)),
        Region("CA", BoundingBox(43, 32, -114, -125)),
        Region("CO", BoundingBox(42, 36, -102, -110)),
        Region("MP", BoundingBox(21, 14, 147, 144)),
        Region("CT", BoundingBox(43, 40, -71, -74)),
        Region("DE", BoundingBox(40, 38, -75, -76)),
        Region("DC", BoundingBox(40, 38, -76, -78)),
        Region("FL", BoundingBox(32, 24, -80, -88)),
        Region("GA", BoundingBox(36, 30, -80, -87)),
        Region("GU", BoundingBox(14, 13, 145, 144)),
        Region("HI", BoundingBox(29, 18, -154, -179)),
        Region("ID", BoundingBox(50, 41, -111, -118)),
        Region("IL", BoundingBox(43, 36, -87, -92)),
        Region("IN", BoundingBox(42, 37, -84, -89)),
        Region("IA", BoundingBox(44, 40, -90, -97)),
        Region("KS", BoundingBox(41, 36, -94, -103)),
        Region("KY", BoundingBox(40, 36, -91, -90)),
        Region("LA", BoundingBox(34, 28, -88, -95)),
        Region("ME", BoundingBox(48, 42, -66, -72)),
        Region("MD", BoundingBox(40, 37, -75, -80)),
        Region("MA", BoundingBox(43, 41, -69, -74)),
        Region("MI", BoundingBox(49, 41, -82, -91)),
        Region("MN", BoundingBox(50, 43, -89, -98)),
        Region("MS", BoundingBox(35, 30, -88, -92)),
        Region("MO", BoundingBox(41, 35, -89, -96)),
        Region("MT", BoundingBox(50, 44, -104, -117)),
        Region("NE", BoundingBox(44, 39, -95, -105)),
        Region("NV", BoundingBox(43, 35, -114, -121)),
        Region("NH", BoundingBox(46, 42, -70, -73)),
        Region("NJ", BoundingBox(42, 38, -73, -76)),
        Region("NM", BoundingBox(38, 31, -103, -110)),
        Region("NY", BoundingBox(46, 40, -71, -80)),
        Region("NC", BoundingBox(37, 33, -75, -85)),
        Region("ND", BoundingBox(50, 45, -96, -105)),
        Region("OH", BoundingBox(42, 38, -80, -85)),
        Region("OK", BoundingBox(38, 33, -94, -104)),
        Region("OR", BoundingBox(47, 41, -116, -125)),
        Region("PA", BoundingBox(43, 39, -74, -81)),
        Region("PR", BoundingBox(19, 17, -65, -68)),
        Region("RI", BoundingBox(43, 41, -71, -72)),
        Region("SC", BoundingBox(36, 32, -78, -84)),
        Region("SD", BoundingBox(46, 42, -96, -105)),
        Region("TN", BoundingBox(37, 34, -81, -91)),
        Region("TX", BoundingBox(37, 25, -93, -107)),
        Region("VI", BoundingBox(19, 17, -64, -66)),
        Region("UT", BoundingBox(43, 36, -109, -115)),
        Region("VT", BoundingBox(46, 42, -71, -74)),
        Region("VA", BoundingBox(40, 36, -75, -84)),
        Region("WA", BoundingBox(50, 48, -116, -125)),
        Region("WV", BoundingBox(41, 37, -77, -83)),
        Region("WI", BoundingBox(48, 42, -86, -93)),
        Region("WY", BoundingBox(46, 40, -104, -112)),
        // -124.73364306703067,45.54383071539715,-116.9161607504075,49.00240502974029
    )

@Composable
@Preview
fun App(regions: List<Region> = USregions()) {
    MaterialTheme {
        var showRegion by remember { mutableStateOf(false) }
        var selectedRegions by remember { mutableStateOf(setOf<String>()) }
        var selectedBoxes by remember { mutableStateOf(setOf<BoundingBox>()) }
        val coroutineScope = rememberCoroutineScope()

        // status text
        var isDownloading by remember { mutableStateOf(false) }
        var downloadStatus by remember { mutableStateOf("") }
        var isInstalling by remember { mutableStateOf(false) }
        var installStatus by remember { mutableStateOf("") }

        Column(
            modifier =
                Modifier
                    .padding(20.dp)
                    .safeContentPadding()
                    .fillMaxSize(),
        ) {
            // Text banner at top for regions
            Text(
                if (selectedRegions.isEmpty()) {
                    "No location selected"
                } else {
                    "Selected: ${selectedRegions.joinToString()}"
                },
                style = TextStyle(fontSize = 20.sp),
                textAlign = TextAlign.Center,
                modifier = Modifier.fillMaxWidth().align(Alignment.CenterHorizontally),
            )

            // Drop-down multiple select options for states
            Row(modifier = Modifier.padding(start = 20.dp, top = 10.dp)) {
                DropdownMenu(
                    expanded = showRegion,
                    onDismissRequest = { showRegion = false },
                ) {
                    regions.forEach { (State, box) ->
                        val isSelected = State in selectedRegions
                        DropdownMenuItem(
                            text = {
                                Row(verticalAlignment = Alignment.CenterVertically) {
                                    // null to delegate to onClick
                                    Checkbox(
                                        checked = isSelected,
                                        onCheckedChange = null,
                                    )
                                    Spacer(modifier = Modifier.width(8.dp))
                                    Text(State)
                                }
                            },
                            onClick = {
                                // Toggle selection
                                if (isSelected) {
                                    selectedRegions -= State
                                    selectedBoxes -= box
                                } else {
                                    selectedRegions += State
                                    selectedBoxes += box
                                }
                            },
                        )
                    }
                }
            }

            Button(
                modifier = Modifier.padding(start = 20.dp, top = 10.dp),
                onClick = { showRegion = !showRegion },
            ) {
                Text("Select Location(s)")
            }

            // Button to click download
            Button(
                modifier = Modifier.padding(start = 20.dp, top = 10.dp),
                onClick = {
                    coroutineScope.launch {
                        isDownloading = true
                        downloadStatus = "Downloading..."

                        val result =
                            DownloadManager().downloadData(
                                regions = selectedBoxes,
                                useNASA = true,
                                useOSM = true,
                                useFAA = true,
                                outputDir = "../../assets/maps/",
                            )

                        result.onSuccess { output ->
                            downloadStatus = "Download completed!"
                        }.onFailure { error ->
                            downloadStatus = "Error: ${error.message}"
                        }

                        isDownloading = false
                    }
                },
                enabled = !isDownloading && selectedBoxes.isNotEmpty(),
            ) {
                Text(if (isDownloading) "Downloading..." else "Download data")
            }
            if (downloadStatus.isNotEmpty()) {
                Text(
                    downloadStatus,
                    modifier = Modifier.padding(start = 20.dp, top = 10.dp),
                    style = TextStyle(fontSize = 14.sp),
                )
            }

            // Button to click install
            Button(
                modifier = Modifier.padding(start = 20.dp, top = 10.dp),
                onClick = {
                    coroutineScope.launch {
                        isInstalling = true
                        installStatus = "Installing..."

                        val result = DownloadManager().installScript()

                        result.onSuccess { output ->
                            installStatus = "Installation completed!"
                        }.onFailure { error ->
                            downloadStatus = "Error: ${error.message}"
                        }

                        isInstalling = false
                    }
                },
                enabled = !isInstalling,
            ) {
                Text(if (isInstalling) "Installing..." else "Install synvis app")
            }
            if (installStatus.isNotEmpty()) {
                Text(
                    installStatus,
                    modifier = Modifier.padding(start = 20.dp, top = 10.dp),
                    style = TextStyle(fontSize = 14.sp),
                )
            }
        }
    }
}
