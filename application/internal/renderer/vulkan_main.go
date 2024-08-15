// Package renderer for the internal renderings of the svs application
package renderer

// This file is for initializing and handling the main vulkan necessities

func NewApplication() *Application {
	camera := NewCamera()
	frustum := &Frustum{}

	app := &Application{
		camera:        camera,
		frustum:       frustum,
		visibleChunks: make([]*Chunk, 1000),
		chunks:        make(map[string]*Chunk),
	}

	return app
}
