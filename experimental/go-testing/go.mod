module go-testing

go 1.19

replace github.com/vulkan-go/asche v0.0.0-20190912134304-d4b318b67e07 => ./asche

require (
	github.com/go-gl/glfw/v3.3/glfw v0.0.0-20240506104042-037f3cc74f2a
	github.com/golang/freetype v0.0.0-20170609003504-e2365dfdc4a0
	github.com/vulkan-go/asche v0.0.0-20190912134304-d4b318b67e07
	github.com/vulkan-go/vulkan v0.0.0-20221209234627-c0a353ae26c8
	github.com/xlab/closer v1.1.0
	github.com/xlab/linmath v0.0.0-20220922225318-40b6290c3b40
	golang.org/x/term v0.22.0
)

require (
	golang.org/x/image v0.18.0 // indirect
	golang.org/x/sys v0.22.0 // indirect
)
