package ui

/*
	UDim stands for Universal Dimension. It provides an easy way for us to describe the position or size of some 2D element on the screen
	in terms of pixels, % of screen size or both.

	When applying a UDim object to an imgui element. Call ToPosition() or ToSize() and it will return an imgui.Vec2 in raw pixels
	that can be used for positioning or sizing the element.

	For positioning, UDim also supports the use of anchor points. Normally when positioning a UI object, you are setting the position of the top left corner of the object.
	With an anchor point you can change it to wherever you want. For example, if the anchor point is set to (0.5, 0.5) then the object will be positioned relative to
	its center. If set to (1, 1) then it will be positioned relative to its bottom right corner. The anchor point could also lie beyond the bounds of the object

	Example Usage:
		objectSize := NewUDim(0.5, -20, 0, 50) // half of the screen minus 20 pixels wide on X axis, 50 pixels tall on Y axis
		anchorPoint := imgui.NewVec2(1, 0) // Top right corner
		objectPos := UDimFromScale(0.5, 0) // Position at middle of screen on X axis

		imgui.SetNextWindowSize(objectSize.ToSize())
		imgui.SetNextWindowPos(objectPos.ToPosWithAnchor(anchorPoint, objectSize))

	Easy to center an object on screen:
		objectSize := UDimFromPixels(243, 179)
		anchorPoint := imgui.NewVec2(0.5, 0.5) // Center of object
		objectPos := UDimFromScale(0.5, 0.5) // Position at middle of screen

		imgui.SetNextWindowSize(objectSize.ToSize())
		imgui.SetNextWindowPos(objectPos.ToPosWithAnchor(anchorPoint, objectSize))
*/

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

func (udim *UDim) ToPosition() imgui.Vec2 {
	viewportSize := imgui.MainViewport().Size()
	viewportPos := imgui.MainViewport().Pos()
	absPos := viewportPos.Add(imgui.NewVec2(viewportSize.X*udim.XScale+udim.XPixels, viewportSize.Y*udim.YScale+udim.YPixels))
	return absPos
}

func (udim *UDim) ToPosWithAnchor(anchorPoint imgui.Vec2, size *UDim) imgui.Vec2 {
	pos := udim.ToPosition()
	abSize := size.ToSize()
	return pos.Sub(imgui.NewVec2(anchorPoint.X*abSize.X, anchorPoint.Y*abSize.Y))
}

func (udim *UDim) ToSize() imgui.Vec2 {
	viewportSize := imgui.MainViewport().Size()
	return imgui.NewVec2(viewportSize.X*udim.XScale+udim.XPixels, viewportSize.Y*udim.YScale+udim.YPixels)
}

func (udim *UDim) Add(other *UDim) *UDim {
	return NewUDim(udim.XScale+other.XScale, udim.XPixels+other.XPixels, udim.YScale+other.YScale, udim.YPixels+other.YPixels)
}

func (udim *UDim) Sub(other *UDim) *UDim {
	return NewUDim(udim.XScale-other.XScale, udim.XPixels-other.XPixels, udim.YScale-other.YScale, udim.YPixels-other.YPixels)
}

func (udim *UDim) Mul(other *UDim) *UDim {
	return NewUDim(udim.XScale*other.XScale, udim.XPixels*other.XPixels, udim.YScale*other.YScale, udim.YPixels*other.YPixels)
}

func (udim *UDim) Div(other *UDim) *UDim {
	return NewUDim(udim.XScale/other.XScale, udim.XPixels/other.XPixels, udim.YScale/other.YScale, udim.YPixels/other.YPixels)
}
