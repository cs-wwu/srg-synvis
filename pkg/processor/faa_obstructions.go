package processor

type FAAPoint struct {
	Lat    float64
	Lon    float64
	Height float64
}

// TODO: store reduced format FAA DOF data
// put this reduced format FAA DOF data into coordinate buffers that will be sent to renderer
