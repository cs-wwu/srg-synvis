package ui

import "github.com/elokore/cimgui-go-vulkan/imgui"

type UDim struct {
	XScale  float32
	XPixels float32
	YScale  float32
	YPixels float32
}

func NewUDim(xScale float32, xPixels float32, yScale float32, yPixels float32) *UDim {
	return &UDim{
		XScale:  xScale,
		XPixels: xPixels,
		YScale:  yScale,
		YPixels: yPixels,
	}
}

func UDimFromScale(xScale float32, yScale float32) *UDim {
	return &UDim{
		XScale:  xScale,
		XPixels: 0,
		YScale:  yScale,
		YPixels: 0,
	}
}

func UDimFromPixels(xPixels float32, yPixels float32) *UDim {
	return &UDim{
		XScale:  0,
		XPixels: xPixels,
		YScale:  0,
		YPixels: yPixels,
	}
}

func (udim *UDim) ToAbsolutePosition() imgui.Vec2 {
	viewportSize := imgui.MainViewport().Size()
	viewportPos := imgui.MainViewport().Pos()
	absPos := viewportPos.Add(imgui.NewVec2(viewportSize.X*udim.XScale+udim.XPixels, viewportSize.Y*udim.YScale+udim.YPixels))
	return absPos
}

func (udim *UDim) ToAbsoluteSize() imgui.Vec2 {
	viewportSize := imgui.MainViewport().Size()
	return imgui.NewVec2(viewportSize.X*udim.XScale+udim.XPixels, viewportSize.Y*udim.YScale+udim.YPixels)
}
