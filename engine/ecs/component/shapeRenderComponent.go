package component

import (
	"github.com/go-gl/mathgl/mgl32"
	"github.com/yohamta/donburi"
)

// 保存 ECS 渲染接入过渡期所需的最小纯色矩形数据
//
// 当前只用于验证 ECS 到渲染器的完整链路，不属于正式渲染组件
// 后续接入 Sprite 和 Render 组件后应删除，不包含纹理、锚点、图层和深度
type ShapeRenderComponent struct {
	// 矩形在世界空间中的尺寸
	Size mgl32.Vec2
	// 矩形颜色
	Color mgl32.Vec4
}

// 提供给实体创建、查询和组件读写使用的 Donburi 组件类型句柄
//
// 当前句柄只服务于过渡期的最小形状渲染系统，接入正式渲染组件后应一并删除
var ShapeRender = donburi.NewComponentType[ShapeRenderComponent]().SetName("engine.ShapeRender")
