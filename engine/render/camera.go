package render

import (
	"math"

	emath "tiny_farm/engine/utils/math"

	"github.com/go-gl/mathgl/mgl32"
)

// 管理世界坐标到逻辑坐标的相机
//
// 当前阶段先支持位置、缩放和可见区域换算
// 位置语义与参考实现一致，表示相机中心点的世界坐标
type Camera struct {
	// 相机中心点在世界坐标系中的位置
	// 该世界点会映射到逻辑画布中心
	position mgl32.Vec2
	// 逻辑分辨率
	logicalSize mgl32.Vec2
	// 缩放倍率
	zoom float32
}

// 创建相机
func NewCamera(logicalSize mgl32.Vec2) *Camera {
	return &Camera{
		logicalSize: emath.Mgl32Vec2Max(mgl32.Vec2{1.0, 1.0}, logicalSize),
		zoom:        1.0,
	}
}

// 设置相机中心点
func (c *Camera) SetPosition(position mgl32.Vec2) {
	if c == nil {
		return
	}
	c.position = position
}

// 平移相机
func (c *Camera) Translate(offset mgl32.Vec2) {
	if c == nil {
		return
	}
	c.position = c.position.Add(offset)
}

// 设置缩放倍率
func (c *Camera) SetZoom(zoom float32) {
	if c == nil {
		return
	}
	if zoom <= 0.0 {
		zoom = 1.0
	}
	c.zoom = zoom
}

// 设置逻辑分辨率
func (c *Camera) SetLogicalSize(logicalSize mgl32.Vec2) {
	if c == nil {
		return
	}
	c.logicalSize = emath.Mgl32Vec2Max(mgl32.Vec2{1.0, 1.0}, logicalSize)
}

// 返回相机中心点
func (c *Camera) Position() mgl32.Vec2 {
	if c == nil {
		return mgl32.Vec2{}
	}
	return c.position
}

// 返回缩放倍率
func (c *Camera) Zoom() float32 {
	if c == nil {
		return 1.0
	}
	return c.zoom
}

// 返回逻辑分辨率
func (c *Camera) LogicalSize() mgl32.Vec2 {
	if c == nil {
		return mgl32.Vec2{}
	}
	return c.logicalSize
}

// 将世界坐标转换成逻辑坐标
// 逻辑坐标 = (世界坐标 - 相机中心) * 缩放 + 逻辑画布中心
func (c *Camera) WorldToLogical(worldPos mgl32.Vec2) mgl32.Vec2 {
	if c == nil || c.zoom <= 0.0 {
		return worldPos
	}
	offset := worldPos.Sub(c.position).Mul(c.zoom)
	return offset.Add(c.logicalSize.Mul(0.5))
}

// 将逻辑坐标转换成世界坐标
// 世界坐标 = (逻辑坐标 - 逻辑画布中心) / 缩放 + 相机中心
func (c *Camera) LogicalToWorld(logicalPos mgl32.Vec2) mgl32.Vec2 {
	if c == nil || c.zoom <= 0.0 {
		return logicalPos
	}
	centered := logicalPos.Sub(c.logicalSize.Mul(0.5))
	return centered.Mul(1.0 / c.zoom).Add(c.position)
}

// 返回当前可见世界区域
func (c *Camera) ViewRect() emath.Rect {
	if c == nil || c.zoom <= 0.0 {
		return emath.Rect{}
	}
	viewSize := c.logicalSize.Mul(1.0 / c.zoom)
	return emath.Rect{
		Position: c.position.Sub(viewSize.Mul(0.5)),
		Size:     viewSize,
	}
}

// 将 world 坐标系下的矩形转换成 logical 坐标系下的矩形
//
// rect 使用 x、y、width、height 表示，x 和 y 是左上角坐标
// 矩形位置通过相机中心点和缩放转换，矩形尺寸只乘以相机缩放
// pixelSnap 开启时会把转换后的坐标和尺寸四舍五入到整数 logical 像素
func (c *Camera) worldRectToLogical(rect mgl32.Vec4, pixelSnap bool) mgl32.Vec4 {
	logicalPos := c.WorldToLogical(mgl32.Vec2{rect.X(), rect.Y()})
	logicalSize := mgl32.Vec2{rect.Z() * c.zoom, rect.W() * c.zoom}
	if pixelSnap {
		// 像素对齐，因为经过 camera 变换后，逻辑坐标和尺寸很可能出现小数
		// 如果直接拿这些小数去画像素风内容，容易抖或者糊。
		// 所以开启 pixelSnap 后，就把位置和尺寸 round 到整数像素。
		logicalPos = mgl32.Vec2{
			// 四舍五入到最接近的整数
			float32(math.Round(float64(logicalPos.X()))),
			float32(math.Round(float64(logicalPos.Y()))),
		}
		logicalSize = mgl32.Vec2{
			float32(math.Round(float64(logicalSize.X()))),
			float32(math.Round(float64(logicalSize.Y()))),
		}
	}
	return mgl32.Vec4{logicalPos.X(), logicalPos.Y(), logicalSize.X(), logicalSize.Y()}
}
