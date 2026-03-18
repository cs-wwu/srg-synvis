package streaming

import (
	"math"
	"sync"

	"github.com/xlab/linmath"
)

// This loads chunks for 1 degree sections in the background.
//
// One of these is created per type of degree chunk. Currently,
// there is one for roads, and there will be one for obstructions.
type DegreeChunkStreamer[T any] struct {
	sync.Mutex

	Chunks map[LatLonInt]T

	request chan LatLonInt

	pending map[LatLonInt]bool
}

// TODO: These really should be consolidated. Also chunks aren't square.
const (
	ARC_DEGREES_TO_MINUTES = 60.0
	ARC_DEGREES_TO_SECONDS = ARC_DEGREES_TO_MINUTES * 60.0
)

type LatLonInt struct {
	Lat int
	Lon int
}
type LatLonFloat struct {
	Lat float64
	Lon float64
}

func NewDegreeStreamer[T any](builder func(LatLonInt) T) *DegreeChunkStreamer[T] {
	streamer := &DegreeChunkStreamer[T]{
		Chunks:  make(map[LatLonInt]T),
		pending: make(map[LatLonInt]bool),
		request: make(chan LatLonInt, 64),
	}

	for range 8 {
		go func() {
			for {
				pos := <-streamer.request
				chunk := builder(pos)

				streamer.Lock()
				streamer.Chunks[pos] = chunk
				delete(streamer.pending, pos)
				streamer.Unlock()
			}
		}()
	}
	return streamer
}

func (d *DegreeChunkStreamer[T]) UpdatePos(pos linmath.Vec3) {
	d.Lock()
	defer d.Unlock()

	lon_req := int(math.Floor(float64(pos[0]) / ARC_DEGREES_TO_SECONDS))
	lat_req := int(math.Floor(float64(-pos[2]) / ARC_DEGREES_TO_SECONDS))

	for lon := lon_req - 1; lon <= lon_req+1; lon++ {
		for lat := lat_req - 1; lat <= lat_req+1; lat++ {
			pos := LatLonInt{Lat: lat, Lon: lon}
			_, exists := d.Chunks[pos]
			_, isPending := d.pending[pos]
			if !exists && !isPending {
				d.pending[pos] = true
				d.request <- pos
			}
		}
	}
}

func (d *DegreeChunkStreamer[T]) GetLoaded() []T {
	d.Lock()
	defer d.Unlock()

	chunks := make([]T, 0, len(d.Chunks))
	for _, chunk := range d.Chunks {
		chunks = append(chunks, chunk)
	}
	return chunks
}
