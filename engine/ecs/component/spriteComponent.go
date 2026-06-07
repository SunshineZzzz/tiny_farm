package component

import (
	"tiny_farm/engine/resource"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/yohamta/donburi"
)

// 精灵组件，保存精灵资源引用和绘制区域
//
// 组件保存纹理 key 和加载路径，不持有 Renderer 或 OpenGL 纹理对象
type SpriteComponent struct {
	// 纹理缓存使用的稳定资源 key
	TextureKey resource.ResourceKey
	// 缓存未命中时使用的纹理文件路径
	TexturePath string
	// 纹理像素空间中的源矩形
	SourceRect mgl32.Vec4
	// 精灵在世界空间中的基础尺寸
	Size mgl32.Vec2
	// 锚点比例，范围通常为 0.0 到 1.0
	Pivot mgl32.Vec2
}

// 提供给实体创建、查询和组件读写使用的 Donburi 组件类型句柄
var Sprite = donburi.NewComponentType[SpriteComponent]().SetName("engine.Sprite")
