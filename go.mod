module github.com/cs-wwu/srg-synvis

go 1.24.3

require (
	github.com/elokore/glfw/v3.4/glfw v0.0.0-20251221231958-c1dc85df2170
	github.com/udhos/gwob v1.0.0
	github.com/vulkan-go/vulkan v0.0.0-20221209234627-c0a353ae26c8
	github.com/xlab/closer v1.1.0
	google.golang.org/protobuf v1.33.0
)

require (
	github.com/elokore/cimgui-go-vulkan v0.0.0-20251224102619-f417eab72b92
	github.com/stratoberry/go-gpsd v1.3.0
	github.com/ungerik/go3d v0.0.0-20240502073936-1137f6adf7e9
	github.com/vulkan-go/asche v0.0.0-20190912134304-d4b318b67e07
	github.com/westphae/geomag v1.0.2
	github.com/xlab/linmath v0.0.0-20220922225318-40b6290c3b40
	golang.org/x/net v0.42.0
	golang.org/x/term v0.33.0
)

require golang.org/x/sys v0.36.0 // indirect

replace github.com/vulkan-go/asche => ./pkg/asche
