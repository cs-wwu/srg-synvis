package renderer

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"runtime"

	vk "github.com/vulkan-go/vulkan"
)

type Unwind []func()

func (u Unwind) Add(cleanup func()) {
	u = append(u, cleanup)
}

func (u Unwind) Unwind() {
	for i := len(u) - 1; i >= 0; i-- {
		u[i]()
	}
}

func (u Unwind) Discard() {
	if len(u) > 0 {
		u = u[:0]
	}
}

func IsError(ret vk.Result) bool {
	return ret != vk.Success
}

func OrPanic(err error, finalizers ...func()) {
	if err != nil {
		for _, fn := range finalizers {
			fn()
		}
		panic(err)
	}
}

func checkErr(err *error) {
	if v := recover(); v != nil {
		*err = fmt.Errorf("%+v", v)
	}
}

func checkErrStack(err *error) {
	if v := recover(); v != nil {
		stack := make([]byte, 32*1024)
		n := runtime.Stack(stack, false)
		switch event := v.(type) {
		case error:
			*err = fmt.Errorf("%s\n%s", event.Error(), stack[:n])
		default:
			*err = fmt.Errorf("%+v %s", v, stack[:n])
		}
	}
}

type sliceHeader struct {
	Data uintptr
	Len  int
	Cap  int
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Converts the given file to a byte array to be used
func FileToBytes(filename string) []byte {
	file, err := os.Open(filename)
	OrPanic(err)
	defer file.Close()

	// Read the file into data
	fileInfo, err := file.Stat()
	OrPanic(err)
	fileSize := fileInfo.Size()
	data := make([]byte, fileSize)
	_, err = file.Read(data)
	OrPanic(err)

	return data
}

func loadTextureData(name string, rowPitch int) ([]byte, int, int, error) {
	data := FileToBytes(name)
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, 0, 0, err
	}
	newImg := image.NewRGBA(img.Bounds())
	if rowPitch <= 4*img.Bounds().Dy() {
		// apply the proposed row pitch only if supported,
		// as we're using only optimal textures.
		newImg.Stride = rowPitch
	}
	draw.Draw(newImg, newImg.Bounds(), img, image.Point{}, draw.Src)
	size := newImg.Bounds().Size()
	return []byte(newImg.Pix), size.X, size.Y, nil
}
