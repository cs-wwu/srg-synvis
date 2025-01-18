package models

import lin "github.com/xlab/linmath"

// Current config settings
type AppConfig struct {
	UseGPS         bool
	UseHat         bool
	MonitorNum     int
	CameraStartPos lin.Vec3
}
