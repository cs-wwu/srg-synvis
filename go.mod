module github.com/cs-wwu/srg-synvis

go 1.24.3

require (
	github.com/udhos/gwob v1.0.0
	github.com/vulkan-go/vulkan v0.0.0-20221209234627-c0a353ae26c8
	github.com/xlab/closer v1.1.0
	google.golang.org/protobuf v1.33.0
)

require (
	github.com/elokore/cimgui-go-vulkan v0.0.0-20260415212211-f5a7b491908d
	github.com/stratoberry/go-gpsd v1.3.0
	github.com/ungerik/go3d v0.0.0-20240502073936-1137f6adf7e9
	github.com/vulkan-go/asche v0.0.0-20190912134304-d4b318b67e07
	github.com/westphae/geomag v1.0.2
	github.com/xlab/linmath v0.0.0-20220922225318-40b6290c3b40
	golang.org/x/net v0.42.0
)

require github.com/go-gl/glfw/v3.4/glfw v0.1.0-pre.1

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/stretchr/testify v1.11.1
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/vulkan-go/asche => ./pkg/asche
