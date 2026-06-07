package component

import (
	"github.com/go-gl/mathgl/mgl32"
	"github.com/yohamta/donburi"
)

const (
	// 默认世界实体渲染层
	MainRenderLayer = 10
)

// 渲染组件，保存精灵绘制顺序和颜色调制参数
type RenderComponent struct {
	// 跨类别绘制层，数值越小越先绘制
	Layer int
	// 同层绘制深度，数值越小越先绘制
	Depth float32
	// 与纹理颜色相乘的颜色调制值
	Color mgl32.Vec4
}

// 提供给实体创建、查询和组件读写使用的 Donburi 组件类型句柄
var Render = donburi.NewComponentType[RenderComponent]().SetName("engine.Render")
